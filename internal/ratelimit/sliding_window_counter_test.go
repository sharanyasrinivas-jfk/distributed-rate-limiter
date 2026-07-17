package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestSlidingWindowCounter_AllowsWithinLimit(t *testing.T) {
	store := setupMiniredis(t)
	limiter := NewSlidingWindowCounterLimiter(store, 5, time.Minute, FailOpen)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		d, err := limiter.Allow(ctx, "client1", "/api/orders")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !d.Allowed {
			t.Fatalf("request %d should have been allowed, remaining estimate too low", i+1)
		}
	}
}

func TestSlidingWindowCounter_RejectsOverLimit(t *testing.T) {
	store := setupMiniredis(t)
	limiter := NewSlidingWindowCounterLimiter(store, 2, time.Minute, FailOpen)
	ctx := context.Background()

	limiter.Allow(ctx, "client1", "/api/orders")
	limiter.Allow(ctx, "client1", "/api/orders")
	d, _ := limiter.Allow(ctx, "client1", "/api/orders")
	if d.Allowed {
		t.Fatal("3rd request should have been rejected when limit is 2")
	}
}

// TestSlidingWindowCounter_BoundaryEdgeCase verifies the weighted-average
// math correctly throttles a burst that straddles the fixed-window boundary
// — the exact failure mode a plain fixed-window algorithm has.
func TestSlidingWindowCounter_BoundaryEdgeCase(t *testing.T) {
	store := setupMiniredis(t)
	windowSeconds := int64(1) // use a 1s window so the test runs fast
	limiter := NewSlidingWindowCounterLimiter(store, 10, time.Duration(windowSeconds)*time.Second, FailOpen)
	ctx := context.Background()

	// Fill up most of the limit right near the end of window N.
	for i := 0; i < 8; i++ {
		limiter.Allow(ctx, "client1", "/api/orders")
	}

	// Sleep past the window boundary into window N+1.
	time.Sleep(time.Duration(windowSeconds) * time.Second)

	allowedInNewWindow := 0
	for i := 0; i < 8; i++ {
		d, _ := limiter.Allow(ctx, "client1", "/api/orders")
		if d.Allowed {
			allowedInNewWindow++
		}
	}

	// A naive fixed-window limiter would allow all 8 again immediately
	// (2x burst at the boundary). The weighted estimate should throttle
	// well before that, since the previous window's 8 requests still
	// count against the early part of the new window.
	if allowedInNewWindow >= 8 {
		t.Errorf("boundary burst not smoothed: allowed %d/8 immediately after window rollover", allowedInNewWindow)
	}
}
