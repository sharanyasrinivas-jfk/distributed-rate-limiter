package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucket_AllowsBurstUpToCapacity(t *testing.T) {
	store := setupMiniredis(t)
	limiter := NewTokenBucketLimiter(store, 5, 1, FailOpen)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		d, err := limiter.Allow(ctx, "client1", "/api/users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !d.Allowed {
			t.Fatalf("request %d within capacity should be allowed", i+1)
		}
	}
	d, _ := limiter.Allow(ctx, "client1", "/api/users")
	if d.Allowed {
		t.Fatal("6th request should exhaust the bucket")
	}
}

func TestTokenBucket_RefillsOverTime(t *testing.T) {
	store := setupMiniredis(t)
	// capacity 1, refills 10 tokens/sec -> 100ms per token
	limiter := NewTokenBucketLimiter(store, 1, 10, FailOpen)
	ctx := context.Background()

	d1, _ := limiter.Allow(ctx, "client1", "/api/users")
	if !d1.Allowed {
		t.Fatal("first request should consume the initial token")
	}
	d2, _ := limiter.Allow(ctx, "client1", "/api/users")
	if d2.Allowed {
		t.Fatal("immediate second request should be rejected, bucket just emptied")
	}

	time.Sleep(150 * time.Millisecond)
	d3, _ := limiter.Allow(ctx, "client1", "/api/users")
	if !d3.Allowed {
		t.Fatal("after waiting for refill, request should be allowed")
	}
}

func TestTokenBucket_ConcurrentRequests_NoOvercounting(t *testing.T) {
	store := setupRealRedis(t)
	// Large capacity, zero refill during the test window so the total
	// allowed count should be exactly the capacity.
	limiter := NewTokenBucketLimiter(store, 50, 0, FailOpen)
	ctx := context.Background()

	var wg sync.WaitGroup
	var allowedCount int64
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := limiter.Allow(ctx, "concurrent-client", "/api/users")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if d.Allowed {
				atomic.AddInt64(&allowedCount, 1)
			}
		}()
	}
	wg.Wait()

	if allowedCount != 50 {
		t.Fatalf("expected exactly 50 allowed requests (capacity, no refill), got %d", allowedCount)
	}
}
