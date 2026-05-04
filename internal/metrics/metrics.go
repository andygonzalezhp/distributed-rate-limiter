package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var RequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "rate_limiter_requests_total",
		Help: "Total number of requests processed by the rate limiter.",
	},
	[]string{"instance", "client_type", "decision", "status_code"},
)

var RedisErrorsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "rate_limiter_redis_errors_total",
		Help: "Total number of Redis errors encountered by the rate limiter.",
	},
	[]string{"instance"},
)

var RequestDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "rate_limiter_request_duration_seconds",
		Help: "Request duration in seconds.",
		Buckets: []float64{
			0.0005,
			0.001,
			0.0025,
			0.005,
			0.01,
			0.025,
			0.05,
			0.1,
			0.25,
			0.5,
			1,
		},
	},
	[]string{"instance", "decision"},
)

func Handler() http.Handler {
	return promhttp.Handler()
}

func ObserveRequest(instanceID string, clientID string, decision string, statusCode int, duration time.Duration) {
	clientType := getClientType(clientID)

	RequestsTotal.WithLabelValues(
		instanceID,
		clientType,
		decision,
		strconv.Itoa(statusCode),
	).Inc()

	RequestDurationSeconds.WithLabelValues(
		instanceID,
		decision,
	).Observe(duration.Seconds())
}

func ObserveRedisError(instanceID string) {
	RedisErrorsTotal.WithLabelValues(instanceID).Inc()
}

func getClientType(clientID string) string {
	if strings.HasPrefix(clientID, "api_key:") {
		return "api_key"
	}

	if strings.HasPrefix(clientID, "ip:") {
		return "ip"
	}

	return "unknown"
}
