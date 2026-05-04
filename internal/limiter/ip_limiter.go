package limiter

import "sync"

type IPRateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*TokenBucket
	capacity   int
	refillRate float64
}

func NewIPRateLimiter(capacity int, refillRate float64) *IPRateLimiter {
	return &IPRateLimiter{
		buckets:    make(map[string]*TokenBucket),
		capacity:   capacity,
		refillRate: refillRate,
	}
}

func (l *IPRateLimiter) Allow(clientID string) (bool, int) {
	l.mu.Lock()

	bucket, exists := l.buckets[clientID]
	if !exists {
		bucket = NewTokenBucket(l.capacity, l.refillRate)
		l.buckets[clientID] = bucket
	}

	l.mu.Unlock()

	return bucket.Allow()
}
