# Distributed Rate Limiter

A distributed rate-limiting gateway built in Go. The service sits in front of a backend API and enforces global request limits across multiple gateway instances using Redis-backed token buckets.

## Overview

This project implements a production-style rate limiter that can be placed in front of an API to protect backend services from excessive traffic, noisy neighbors, or abuse.

The gateway supports:

- Reverse proxying requests to a backend API
- Per-client token bucket rate limiting
- Shared distributed state through Redis
- Atomic quota updates using Redis Lua scripting
- Horizontal scaling across multiple gateway nodes
- Nginx load balancing
- Docker Compose deployment
- k6 load testing

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
- k6

## Features

- Distributed token bucket rate limiting
- Redis-backed shared state across gateway instances
- Atomic token updates using Lua scripts
- Three horizontally scaled Go gateway nodes
- Nginx round-robin load balancing
- Reverse proxy behavior using Go's `httputil.ReverseProxy`
- Per-client rate-limit tracking by IP address
- Rate-limit response headers
- Dockerized local deployment
- k6 benchmark and quota correctness tests

## How It Works

Each request first reaches the Nginx load balancer, which forwards traffic to one of three Go gateway instances.

Each gateway extracts the client identity, currently based on IP address, and checks whether that client has enough tokens remaining in its token bucket.

The token bucket state is stored in Redis instead of local memory. This allows all gateway instances to enforce the same global rate limit.

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

## Rate Limit Headers

Responses include headers such as:

```http
X-Gateway-Instance: gateway-1
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
```

`X-Gateway-Instance` is included to prove that requests are being distributed across multiple gateway nodes.

## Project Structure

```txt
distributed-rate-limiter/
├── cmd/
│   ├── demo-api/
│   │   └── main.go
│   └── gateway/
│       └── main.go
├── internal/
│   └── limiter/
│       ├── ip_limiter.go
│       ├── redis_limiter.go
│       └── token_bucket.go
├── deploy/
│   └── nginx/
│       └── nginx.conf
├── loadtest/
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
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
```

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

## Inspecting Redis State

Open Redis CLI:

```bash
docker compose exec redis redis-cli
```

List rate-limit keys:

```redis
KEYS rate_limit:*
```

Inspect a bucket:

```redis
HGETALL rate_limit:<client-ip>
```

Example Redis state:

```txt
1) "tokens"
2) "0.16499999999999954"
3) "last_refill"
4) "1777929314667"
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

Run quota test:

```bash
make quota
```

Run benchmark:

```bash
make benchmark
```

## Future Improvements

- Add Prometheus metrics
- Add Grafana dashboard
- Support API-key based quotas
- Add sliding window rate limiting
- Add leaky bucket rate limiting
- Add Redis Cluster support
- Add Kubernetes manifests
- Add request logging and tracing
- Add configurable route-specific limits
- Add integration tests with Docker Compose

## Resume Bullet

```txt
Built a distributed rate-limiting gateway in Go with Redis-backed token buckets, atomic Lua scripting, Nginx load balancing, and 3 containerized gateway nodes; benchmarked locally with k6 at ~4.4k requests/sec and ~2.6ms p95 latency.
```
