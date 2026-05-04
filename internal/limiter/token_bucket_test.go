package limiter

import (
	"testing"
	"time"
)

func TestTokenBucketAllowsRequestsUntilCapacity(t *testing.T) {
	bucket := NewTokenBucket(3, 1)

	for i := 0; i < 3; i++ {
		allowed, remaining := bucket.Allow()
		if !allowed {
			t.Fatalf("expected request %d to be allowed", i+1)
		}

		expectedRemaining := 2 - i
		if remaining != expectedRemaining {
			t.Fatalf("expected %d remaining tokens, got %d", expectedRemaining, remaining)
		}
	}
}

func TestTokenBucketRejectsWhenEmpty(t *testing.T) {
	bucket := NewTokenBucket(2, 1)

	allowed, _ := bucket.Allow()
	if !allowed {
		t.Fatal("expected first request to be allowed")
	}

	allowed, _ = bucket.Allow()
	if !allowed {
		t.Fatal("expected second request to be allowed")
	}

	allowed, remaining := bucket.Allow()
	if allowed {
		t.Fatal("expected third request to be rejected")
	}

	if remaining != 0 {
		t.Fatalf("expected 0 remaining tokens, got %d", remaining)
	}
}

func TestTokenBucketRefillsOverTime(t *testing.T) {
	bucket := NewTokenBucket(1, 10)

	allowed, _ := bucket.Allow()
	if !allowed {
		t.Fatal("expected first request to be allowed")
	}

	allowed, _ = bucket.Allow()
	if allowed {
		t.Fatal("expected second immediate request to be rejected")
	}

	time.Sleep(150 * time.Millisecond)

	allowed, remaining := bucket.Allow()
	if !allowed {
		t.Fatal("expected request to be allowed after refill")
	}

	if remaining != 0 {
		t.Fatalf("expected 0 remaining tokens after consuming refilled token, got %d", remaining)
	}
}

func TestTokenBucketDoesNotExceedCapacity(t *testing.T) {
	bucket := NewTokenBucket(3, 100)

	time.Sleep(100 * time.Millisecond)

	bucket.mu.Lock()
	tokens := bucket.tokens
	capacity := bucket.capacity
	bucket.mu.Unlock()

	if tokens > capacity {
		t.Fatalf("expected tokens to not exceed capacity %.2f, got %.2f", capacity, tokens)
	}
}

func TestTokenBucketPartialRefillStillRejectsIfLessThanOneToken(t *testing.T) {
	bucket := NewTokenBucket(1, 1)

	allowed, _ := bucket.Allow()
	if !allowed {
		t.Fatal("expected first request to be allowed")
	}

	time.Sleep(100 * time.Millisecond)

	allowed, remaining := bucket.Allow()
	if allowed {
		t.Fatal("expected request to be rejected because less than 1 token refilled")
	}

	if remaining != 0 {
		t.Fatalf("expected 0 visible remaining tokens, got %d", remaining)
	}
}
