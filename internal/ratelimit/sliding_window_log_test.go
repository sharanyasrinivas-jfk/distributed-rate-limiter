package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSlidingWindowLog_AllowsExactlyLimit(t *testing.T) {
	store := setupMiniredis(t)
	limiter := NewSlidingWindowLogLimiter(store, 3, time.Minute, FailOpen)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		d, err := limiter.Allow(ctx, "client1", "/api/orders")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !d.Allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}
	d, _ := limiter.Allow(ctx, "client1", "/api/orders")
	if d.Allowed {
		t.Fatal("4th request should have been rejected")
	}
}

func TestSlidingWindowLog_ConcurrentRequests_NoOvercounting(t *testing.T) {
	store := setupRealRedis(t)
	limiter := NewSlidingWindowLogLimiter(store, 100, time.Minute, FailOpen)
	ctx := context.Background()

	var wg sync.WaitGroup
	var allowedCount int64
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, err := limiter.Allow(ctx, "concurrent-client", "/api/orders")
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

	if allowedCount != 100 {
		t.Fatalf("expected exactly 100 allowed requests, got %d", allowedCount)
	}
}
