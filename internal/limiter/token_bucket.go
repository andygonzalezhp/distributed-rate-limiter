package limiter

import (
	"math"
	"sync"
	"time"
)

type TokenBucket struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	refillRate float64
	lastRefill time.Time
}

func NewTokenBucket(capacity int, refillRate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   float64(capacity),
		tokens:     float64(capacity),
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (b *TokenBucket) Allow() (bool, int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()

	refilledTokens := elapsed * b.refillRate
	b.tokens = math.Min(b.capacity, b.tokens+refilledTokens)
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens -= 1
		return true, int(b.tokens)
	}

	return false, int(b.tokens)
}
