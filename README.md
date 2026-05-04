# Distributed Rate Limiter

![CI](https://github.com/andygonzalezhp/distributed-rate-limiter/actions/workflows/ci.yml/badge.svg)

A distributed rate-limiting gateway built in Go. The service sits in front of a backend API and enforces global request limits across multiple gateway instances using Redis-backed token buckets.

Built as a backend/systems engineering project focused on distributed state, atomic updates, reverse proxying, load balancing, observability, and performance benchmarking.

## Overview

This project implements a production-style rate limiter that can be placed in front of an API to protect backend services from excessive traffic, noisy neighbors, or abusive clients.

The gateway supports:

- Reverse proxying requests to a backend API
- Per-client token bucket rate limiting
- API-key based quota enforcement
- IP-based fallback quota enforcement
- Shared distributed state through Redis
- Atomic quota updates using Redis Lua scripting
- Horizontal scaling across multiple gateway nodes
- Nginx load balancing
- Prometheus metrics
- Docker Compose deployment
- k6 load testing
- Unit tests for the core token bucket algorithm
- GitHub Actions CI

## Architecture

```txt
Client
  ↓
Nginx Load Balancer
  ↓
gateway-1  gateway-2  gateway-3
  ↓
Redis
  ↓
Demo Backend API
```

## Tech Stack

- Go
- Redis
- Redis Lua scripting
- Docker
- Docker Compose
- Nginx
- Prometheus
- k6
- GitHub Actions

## Features

- Distributed token bucket rate limiting
- Redis-backed shared state across gateway instances
- Atomic token updates using Lua scripts
- Three horizontally scaled Go gateway nodes
- Nginx round-robin load balancing
- Reverse proxy behavior using Go's `httputil.ReverseProxy`
- API-key based client identification through `X-API-Key`
- IP-based client identification fallback
- Per-client rate-limit response headers
- Prometheus `/metrics` endpoint
- Request count metrics
- Rejection count metrics
- Redis error metrics
- Request latency histograms
- Dockerized local deployment
- k6 benchmark tests
- k6 quota correctness tests
- Unit tests for token bucket behavior
- GitHub Actions CI for formatting, tests, builds, and Docker smoke checks

## How It Works

Each request first reaches the Nginx load balancer, which forwards traffic to one of three Go gateway instances.

Each gateway extracts the client identity from the request:

```txt
If X-API-Key exists:
    client_id = api_key:<key>

Otherwise:
    client_id = ip:<client-ip>
```

The gateway then checks whether that client has enough tokens remaining in its token bucket.

The token bucket state is stored in Redis instead of local process memory. This allows all gateway instances to enforce the same global rate limit, even when requests are distributed across multiple containers.

The Redis update is performed using a Lua script so that the following operations happen atomically:

```txt
1. Read current token count
2. Calculate refill amount
3. Update token count
4. Decide whether the request is allowed
5. Store the updated bucket state
```

This prevents race conditions when multiple gateway nodes receive requests at the same time.

## Token Bucket Behavior

Each client gets a bucket with:

```txt
capacity = maximum burst size
refill_rate = number of tokens added per second
```

For example:

```txt
capacity = 10
refill_rate = 1 token/second
```

This allows a client to make a burst of 10 requests, then regain 1 allowed request per second.

When the client has tokens available, the gateway forwards the request to the backend API.

When the client has no tokens left, the gateway returns:

```http
HTTP/1.1 429 Too Many Requests
```

## Client Identification

The gateway supports two forms of client identification.

### API-Key Based Limiting

Requests can include an API key:

```bash
curl -i -H "X-API-Key: user-123" http://localhost:8080/hello
```

This creates a Redis bucket such as:

```txt
rate_limit:api_key:user-123
```

Different API keys receive separate buckets.

### IP-Based Fallback

If no API key is provided, the gateway falls back to IP-based limiting:

```txt
rate_limit:ip:<client-ip>
```

This allows the gateway to work with both authenticated and unauthenticated clients.

## Rate Limit Headers

Responses include headers such as:

```http
X-Gateway-Instance: gateway-1
X-RateLimit-Client: api_key:user-123
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
```

`X-Gateway-Instance` is included to prove that requests are being distributed across multiple gateway nodes.

`X-RateLimit-Client` shows which bucket was used for the request.

## Project Structure

```txt
distributed-rate-limiter/
├── .github/
│   └── workflows/
│       └── ci.yml
├── cmd/
│   ├── demo-api/
│   │   └── main.go
│   └── gateway/
│       └── main.go
├── internal/
│   ├── limiter/
│   │   ├── ip_limiter.go
│   │   ├── redis_limiter.go
│   │   ├── token_bucket.go
│   │   └── token_bucket_test.go
│   └── metrics/
│       └── metrics.go
├── deploy/
│   └── nginx/
│       └── nginx.conf
├── loadtest/
│   ├── api-key-test.js
│   ├── basic.js
│   ├── benchmark.js
│   └── quota-test.js
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

## Running Locally

Start the full distributed system:

```bash
docker compose up --build
```

Or run it in detached mode:

```bash
docker compose up --build -d
```

The system starts:

- 1 Redis container
- 1 demo backend API
- 3 Go gateway containers
- 1 Nginx load balancer

## Test the Gateway

Send a request through Nginx:

```bash
curl -i http://localhost:8080/hello
```

Example response:

```http
HTTP/1.1 200 OK
X-Gateway-Instance: gateway-1
X-RateLimit-Client: ip:142.251.35.113
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
```

Example body:

```json
{
  "message": "hello from backend API",
  "timestamp": "2026-05-04T21:14:33Z"
}
```

## Proving Load Balancing

Run:

```bash
for i in {1..9}; do curl -i -s http://localhost:8080/hello | grep X-Gateway-Instance; done
```

Expected output:

```txt
X-Gateway-Instance: gateway-1
X-Gateway-Instance: gateway-2
X-Gateway-Instance: gateway-3
X-Gateway-Instance: gateway-1
X-Gateway-Instance: gateway-2
X-Gateway-Instance: gateway-3
```

This proves Nginx is rotating traffic across the three gateway nodes.

## Proving Distributed Rate Limiting

First clear Redis:

```bash
docker compose exec redis redis-cli FLUSHALL
```

Then run:

```bash
for i in {1..20}; do curl -i -s http://localhost:8080/hello | grep -E "HTTP/1.1|X-Gateway-Instance|X-Ratelimit-Remaining"; echo "---"; done
```

Expected behavior:

```txt
HTTP/1.1 200 OK
X-Gateway-Instance: gateway-1
X-Ratelimit-Remaining: 9
---
HTTP/1.1 200 OK
X-Gateway-Instance: gateway-2
X-Ratelimit-Remaining: 8
---
HTTP/1.1 200 OK
X-Gateway-Instance: gateway-3
X-Ratelimit-Remaining: 7
---
...
HTTP/1.1 429 Too Many Requests
X-Gateway-Instance: gateway-2
X-Ratelimit-Remaining: 0
---
```

This proves that all gateway nodes share the same Redis-backed token bucket.

Without Redis, each gateway would have its own independent bucket. With Redis, the rate limit is enforced globally across all instances.

## Proving API-Key Based Quotas

Clear Redis:

```bash
docker compose exec redis redis-cli FLUSHALL
```

Send 12 requests as `user-123`:

```bash
for i in {1..12}; do curl -i -s -H "X-API-Key: user-123" http://localhost:8080/hello | grep -E "HTTP/1.1|X-Ratelimit-Client|X-Ratelimit-Remaining"; echo "---"; done
```

Expected behavior:

```txt
HTTP/1.1 200 OK
X-Ratelimit-Client: api_key:user-123
X-Ratelimit-Remaining: 9
---
...
HTTP/1.1 200 OK
X-Ratelimit-Remaining: 0
---
HTTP/1.1 429 Too Many Requests
X-Ratelimit-Remaining: 0
---
```

Now send requests as a different API key:

```bash
for i in {1..3}; do curl -i -s -H "X-API-Key: user-456" http://localhost:8080/hello | grep -E "HTTP/1.1|X-Ratelimit-Client|X-Ratelimit-Remaining"; echo "---"; done
```

Expected behavior:

```txt
HTTP/1.1 200 OK
X-Ratelimit-Client: api_key:user-456
X-Ratelimit-Remaining: 9
---
HTTP/1.1 200 OK
X-Ratelimit-Remaining: 8
---
```

This proves different API keys receive separate rate-limit buckets.

## Inspecting Redis State

Open Redis CLI:

```bash
docker compose exec redis redis-cli
```

List rate-limit keys:

```redis
KEYS rate_limit:*
```

Example output:

```txt
1) "rate_limit:api_key:user-123"
2) "rate_limit:api_key:user-456"
```

Inspect a bucket:

```redis
HGETALL rate_limit:api_key:user-123
```

Example Redis state:

```txt
1) "tokens"
2) "0.16499999999999954"
3) "last_refill"
4) "1777929314667"
```

## Metrics

The gateway exposes Prometheus metrics at:

```http
GET /metrics
```

Test the metrics endpoint:

```bash
curl -s http://localhost:8080/metrics | head -n 20
```

You should see default Go runtime metrics such as:

```txt
go_gc_duration_seconds
go_goroutines
go_info
```

Generate some traffic:

```bash
for i in {1..9}; do curl -s http://localhost:8080/hello > /dev/null; done
```

Then check custom rate limiter metrics:

```bash
for i in {1..6}; do curl -s http://localhost:8080/metrics | grep rate_limiter; echo "---"; done
```

Custom metrics include:

```txt
rate_limiter_requests_total
rate_limiter_redis_errors_total
rate_limiter_request_duration_seconds
```

Example metric:

```txt
rate_limiter_requests_total{client_type="ip",decision="allowed",instance="gateway-1",status_code="200"} 3
```

Because Nginx load balances `/metrics` requests too, repeated calls may return metrics from different gateway instances.

In a production deployment, Prometheus would scrape each gateway instance directly.

## Testing

Run Go tests:

```bash
go test ./...
```

Current unit tests cover the in-memory token bucket algorithm, including:

- Allowing requests until capacity is exhausted
- Rejecting requests when the bucket is empty
- Refilling over time
- Preventing refill above capacity
- Rejecting requests when less than one full token has refilled

Run formatting:

```bash
gofmt -w .
```

## Load Testing

This project uses k6 for load testing.

Install k6 on macOS:

```bash
brew install k6
```

Run the basic load test:

```bash
k6 run loadtest/basic.js
```

Run the quota correctness test:

```bash
k6 run loadtest/quota-test.js
```

Run the API-key quota test:

```bash
k6 run loadtest/api-key-test.js
```

Run the benchmark test:

```bash
k6 run loadtest/benchmark.js
```

## Benchmark Result

Local Docker Compose benchmark with:

- 3 Go gateway nodes
- 1 Redis instance
- 1 Nginx load balancer
- 50 virtual users
- 30 second k6 run

| Test | Requests | Throughput | Avg Latency | p95 Latency | Error Rate |
|---|---:|---:|---:|---:|---:|
| k6 basic load test | 132,551 | ~4,416 req/s | ~1.21ms | ~2.63ms | 0% |

## Quota Correctness Result

With a rate limit of 10 requests:

```txt
200 instance=gateway-1 remaining=9
200 instance=gateway-2 remaining=8
200 instance=gateway-3 remaining=7
200 instance=gateway-1 remaining=6
200 instance=gateway-2 remaining=5
200 instance=gateway-3 remaining=4
200 instance=gateway-1 remaining=3
200 instance=gateway-2 remaining=2
200 instance=gateway-3 remaining=1
200 instance=gateway-1 remaining=0
429 instance=gateway-2 remaining=0
429 instance=gateway-3 remaining=0
429 instance=gateway-1 remaining=0
```

This shows that traffic is distributed across multiple gateway instances while the quota is enforced globally.

## API-Key Correctness Result

With a rate limit of 10 requests per API key:

```txt
200 client=api_key:user-123 remaining=9
200 client=api_key:user-123 remaining=8
...
200 client=api_key:user-123 remaining=0
429 client=api_key:user-123 remaining=0
429 client=api_key:user-123 remaining=0

200 client=api_key:user-456 remaining=9
200 client=api_key:user-456 remaining=8
...
200 client=api_key:user-456 remaining=0
429 client=api_key:user-456 remaining=0
429 client=api_key:user-456 remaining=0
```

This shows that each API key receives an independent Redis-backed token bucket.

## Configuration

Gateway configuration is controlled through environment variables:

| Variable | Description | Example |
|---|---|---|
| `PORT` | Gateway server port | `8080` |
| `BACKEND_URL` | Backend API URL | `http://demo-api:8081` |
| `REDIS_ADDR` | Redis address | `redis:6379` |
| `INSTANCE_ID` | Gateway instance name | `gateway-1` |
| `RATE_LIMIT_CAPACITY` | Maximum burst size | `10` |
| `RATE_LIMIT_REFILL_RATE` | Tokens refilled per second | `1` |

Example Docker Compose gateway config:

```yaml
environment:
  - PORT=8080
  - BACKEND_URL=http://demo-api:8081
  - REDIS_ADDR=redis:6379
  - INSTANCE_ID=gateway-1
  - RATE_LIMIT_CAPACITY=10
  - RATE_LIMIT_REFILL_RATE=1
```

## Useful Commands

Start the system:

```bash
make up
```

Start in detached mode:

```bash
make up-detached
```

Stop and remove containers/volumes:

```bash
make down
```

Clear Redis state:

```bash
make flush
```

Run Go tests:

```bash
make test
```

Run quota test:

```bash
make quota
```

Run API-key quota test:

```bash
make api-key-test
```

Run benchmark:

```bash
make benchmark
```

## Continuous Integration

This project uses GitHub Actions CI to verify the project on every push and pull request.

The workflow checks:

- Go dependency download
- Go formatting
- Go unit tests
- Binary builds
- Docker Compose build
- Docker Compose startup
- Gateway `/health` endpoint
- Gateway reverse proxy behavior through `/hello`
- Prometheus `/metrics` endpoint

## Future Improvements

- Add fail-open / fail-closed Redis failure modes
- Add route-specific rate limits
- Add Redis Cluster support
- Add sliding window rate limiting
- Add leaky bucket rate limiting
- Add Kubernetes manifests
- Add Grafana dashboard
- Add request logging and tracing
- Add configurable per-plan quotas
- Add integration tests with Docker Compose
- Add structured JSON logs

## Resume Bullet

```txt
Built a distributed rate-limiting gateway in Go with Redis-backed token buckets, atomic Lua scripting, API-key/IP based quotas, Prometheus metrics, Nginx load balancing, and 3 containerized gateway nodes; benchmarked locally with k6 at ~4.4k requests/sec and ~2.6ms p95 latency.
```
