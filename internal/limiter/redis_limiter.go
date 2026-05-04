package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const tokenBucketLua = `
local key = KEYS[1]

local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local ttl_ms = tonumber(ARGV[4])

local bucket = redis.call("HMGET", key, "tokens", "last_refill")

local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
	tokens = capacity
	last_refill = now_ms
end

local elapsed_seconds = math.max(0, now_ms - last_refill) / 1000
local refill_amount = elapsed_seconds * refill_rate

tokens = math.min(capacity, tokens + refill_amount)
last_refill = now_ms

local allowed = 0

if tokens >= 1 then
	allowed = 1
	tokens = tokens - 1
end

redis.call("HSET", key, "tokens", tokens, "last_refill", last_refill)
redis.call("PEXPIRE", key, ttl_ms)

return {allowed, math.floor(tokens)}
`

type RedisRateLimiter struct {
	client     *redis.Client
	capacity   int
	refillRate float64
	ttl        time.Duration
	script     *redis.Script
}

func NewRedisRateLimiter(redisAddr string, capacity int, refillRate float64) *RedisRateLimiter {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &RedisRateLimiter{
		client:     client,
		capacity:   capacity,
		refillRate: refillRate,
		ttl:        time.Hour,
		script:     redis.NewScript(tokenBucketLua),
	}
}

func (r *RedisRateLimiter) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisRateLimiter) Allow(ctx context.Context, bucketID string) (bool, int, error) {
	return r.AllowWithLimit(ctx, bucketID, r.capacity, r.refillRate)
}

func (r *RedisRateLimiter) AllowWithLimit(ctx context.Context, bucketID string, capacity int, refillRate float64) (bool, int, error) {
	key := "rate_limit:" + bucketID

	nowMs := time.Now().UnixMilli()
	ttlMs := r.ttl.Milliseconds()

	result, err := r.script.Run(
		ctx,
		r.client,
		[]string{key},
		capacity,
		refillRate,
		nowMs,
		ttlMs,
	).Result()

	if err != nil {
		return false, 0, err
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return false, 0, fmt.Errorf("unexpected redis script result: %v", result)
	}

	allowedInt, err := toInt64(values[0])
	if err != nil {
		return false, 0, err
	}

	remainingInt, err := toInt64(values[1])
	if err != nil {
		return false, 0, err
	}

	return allowedInt == 1, int(remainingInt), nil
}

func (r *RedisRateLimiter) Close() error {
	return r.client.Close()
}

func toInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unexpected value type: %T", value)
	}
}
