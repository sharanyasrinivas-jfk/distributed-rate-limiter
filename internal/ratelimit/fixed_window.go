package ratelimit

import (
	"context"
	"fmt"
	"time"
)

// FixedWindowLimiter counts requests in fixed, non-overlapping time buckets
// (e.g. "the minute starting at :00"). Simple and O(1) memory, but allows up
// to 2x the limit in a short burst that straddles a window boundary.
type FixedWindowLimiter struct {
	store    *Store
	limit    int64
	window   time.Duration
	failMode FailMode
}

func NewFixedWindowLimiter(store *Store, limit int64, window time.Duration, failMode FailMode) *FixedWindowLimiter {
	return &FixedWindowLimiter{store: store, limit: limit, window: window, failMode: failMode}
}

func (l *FixedWindowLimiter) Allow(ctx context.Context, clientID, route string) (Decision, error) {
	windowStart := time.Now().Unix() / int64(l.window.Seconds())
	key := fmt.Sprintf("ratelimit:fixed:%s:%s:%d", clientID, route, windowStart)

	count, err := l.store.IncrWithExpire(ctx, key, l.window)
	if err != nil {
		// Redis is unreachable — apply the configured fail mode rather than
		// erroring out to the caller.
		return l.failureDecision(), nil
	}

	resetAt := time.Unix((windowStart+1)*int64(l.window.Seconds()), 0)
	remaining := l.limit - count
	if remaining < 0 {
		remaining = 0
	}
	return Decision{
		Allowed:    count <= l.limit,
		Limit:      l.limit,
		Remaining:  remaining,
		RetryAfter: time.Until(resetAt),
		ResetAt:    resetAt,
	}, nil
}

func (l *FixedWindowLimiter) failureDecision() Decision {
	if l.failMode == FailClosed {
		return closedDecision(l.limit)
	}
	return openDecision(l.limit)
}
