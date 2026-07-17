package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFixedWindow_AllowsExactlyLimit(t *testing.T) {
	store := setupMiniredis(t)
	limiter := NewFixedWindowLimiter(store, 5, time.Minute, FailOpen)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		d, err := limiter.Allow(ctx, "client1", "/api/orders")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !d.Allowed {
			t.Fatalf("request %d should have been allowed", i+1)
		}
	}

	d, err := limiter.Allow(ctx, "client1", "/api/orders")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Allowed {
		t.Fatal("6th request should have been rejected")
	}
}

func TestFixedWindow_DifferentClientsIndependent(t *testing.T) {
	store := setupMiniredis(t)
	limiter := NewFixedWindowLimiter(store, 1, time.Minute, FailOpen)
	ctx := context.Background()

	d1, _ := limiter.Allow(ctx, "client1", "/api/orders")
	d2, _ := limiter.Allow(ctx, "client2", "/api/orders")
	if !d1.Allowed || !d2.Allowed {
		t.Fatal("each client should get its own independent limit")
	}
}

// TestFixedWindow_ConcurrentRequests_NoOvercounting is the most important
// test in this package: it proves the atomic INCR-based approach holds up
// under real concurrent load against real Redis, with no race allowing
// extra requests through.
func TestFixedWindow_ConcurrentRequests_NoOvercounting(t *testing.T) {
	store := setupRealRedis(t)
	limiter := NewFixedWindowLimiter(store, 100, time.Minute, FailOpen)
	ctx := context.Background()

	var wg sync.WaitGroup
	var allowedCount int64
	for i := 0; i < 1000; i++ {
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
		t.Fatalf("expected exactly 100 allowed requests, got %d (overcounting = race condition)", allowedCount)
	}
}
