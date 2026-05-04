package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"distributed-rate-limiter/internal/limiter"
	appmetrics "distributed-rate-limiter/internal/metrics"
)

const (
	FailModeOpen   = "open"
	FailModeClosed = "closed"
)

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func normalizeFailMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))

	switch value {
	case FailModeOpen:
		return FailModeOpen
	case FailModeClosed:
		return FailModeClosed
	default:
		return FailModeClosed
	}
}

func getClientIP(r *http.Request) string {
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func getClientID(r *http.Request) string {
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		return "api_key:" + apiKey
	}

	return "ip:" + getClientIP(r)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}

	return r.ResponseWriter.Write(body)
}

func proxyRequest(proxy *httputil.ReverseProxy, w http.ResponseWriter, r *http.Request) int {
	recorder := &statusRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}

	proxy.ServeHTTP(recorder, r)

	return recorder.statusCode
}

func main() {
	ctx := context.Background()

	backendAddr := getEnv("BACKEND_URL", "http://localhost:8081")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	port := getEnv("PORT", "8080")
	instanceID := getEnv("INSTANCE_ID", "gateway-local")
	failMode := normalizeFailMode(getEnv("FAIL_MODE", FailModeClosed))

	backendURL, err := url.Parse(backendAddr)
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	capacity := getEnvInt("RATE_LIMIT_CAPACITY", 10)
	refillRate := getEnvFloat("RATE_LIMIT_REFILL_RATE", 1)

	rateLimiter := limiter.NewRedisRateLimiter(redisAddr, capacity, refillRate)
	defer rateLimiter.Close()

	if err := rateLimiter.Ping(ctx); err != nil {
		log.Fatalf("could not connect to Redis at %s: %v", redisAddr, err)
	}

	log.Printf("Connected to Redis at %s", redisAddr)
	log.Printf("Redis failure mode: %s", failMode)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("gateway healthy"))
	})

	mux.Handle("/metrics", appmetrics.Handler())

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		clientID := getClientID(r)

		w.Header().Set("X-Gateway-Instance", instanceID)
		w.Header().Set("X-RateLimit-Client", clientID)
		w.Header().Set("X-RateLimit-Fail-Mode", failMode)

		allowed, remaining, err := rateLimiter.Allow(r.Context(), clientID)
		if err != nil {
			log.Printf("rate limiter error: %v", err)

			appmetrics.ObserveRedisError(instanceID)

			if failMode == FailModeOpen {
				w.Header().Set("X-RateLimit-Decision", "fail_open")

				statusCode := proxyRequest(proxy, w, r)

				appmetrics.ObserveRequest(
					instanceID,
					clientID,
					"fail_open",
					statusCode,
					time.Since(start),
				)

				return
			}

			statusCode := http.StatusServiceUnavailable

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Decision", "fail_closed")
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"error":"rate limiter unavailable"}`))

			appmetrics.ObserveRequest(
				instanceID,
				clientID,
				"fail_closed",
				statusCode,
				time.Since(start),
			)

			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(capacity))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			statusCode := http.StatusTooManyRequests

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Decision", "rejected")
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"error":"rate limit exceeded"}`))

			appmetrics.ObserveRequest(instanceID, clientID, "rejected", statusCode, time.Since(start))
			return
		}

		w.Header().Set("X-RateLimit-Decision", "allowed")

		statusCode := proxyRequest(proxy, w, r)

		appmetrics.ObserveRequest(instanceID, clientID, "allowed", statusCode, time.Since(start))
	})

	addr := ":" + port

	log.Printf("Rate limiter gateway running on %s", addr)
	log.Printf("Forwarding requests to backend API at %s", backendAddr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
