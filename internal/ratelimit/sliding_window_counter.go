package ratelimit

import (
	"context"
	"fmt"
	"time"
)

// slidingWindowCounterScript atomically increments the counter for the
// current fixed sub-window and reads the previous sub-window's final count,
// so the caller can compute the weighted estimate without a second round trip.
const slidingWindowCounterScript = `
local curKey = KEYS[1]
local prevKey = KEYS[2]
local window_seconds = tonumber(ARGV[1])

local cur = redis.call('INCR', curKey)
if cur == 1 then
    redis.call('EXPIRE', curKey, window_seconds * 2)
end
local prev = tonumber(redis.call('GET', prevKey) or '0')

return {cur, prev}
`

// SlidingWindowCounterLimiter approximates a true sliding window using two
// adjacent fixed windows and a weighted average based on how far into the
// current window we are. O(1) memory like fixed window, but smooths out the
// boundary-burst problem fixed window has.
type SlidingWindowCounterLimiter struct {
	store    *Store
	limit    int64
	window   time.Duration
	failMode FailMode
}

func NewSlidingWindowCounterLimiter(store *Store, limit int64, window time.Duration, failMode FailMode) *SlidingWindowCounterLimiter {
	return &SlidingWindowCounterLimiter{store: store, limit: limit, window: window, failMode: failMode}
}

func (l *SlidingWindowCounterLimiter) Allow(ctx context.Context, clientID, route string) (Decision, error) {
	windowSeconds := int64(l.window.Seconds())
	now := time.Now()
	curWindow := now.Unix() / windowSeconds
	prevWindow := curWindow - 1

	curKey := fmt.Sprintf("ratelimit:sw_counter:%s:%s:%d", clientID, route, curWindow)
	prevKey := fmt.Sprintf("ratelimit:sw_counter:%s:%s:%d", clientID, route, prevWindow)

	res, err := l.store.EvalScript(ctx, slidingWindowCounterScript, []string{curKey, prevKey}, windowSeconds)
	if err != nil {
		return l.failureDecision(), nil
	}
	result, ok := res.([]interface{})
	if !ok || len(result) != 2 {
		return l.failureDecision(), nil
	}
	curCount := toInt64(result[0])
	prevCount := toInt64(result[1])

	// Fraction of the current window that has elapsed. elapsedFraction=0
	// means we just entered the window (weight prev heavily); ~1 means
	// we're about to leave it (weight prev negligibly).
	windowStartUnix := curWindow * windowSeconds
	elapsedSeconds := float64(now.Unix()-windowStartUnix) + float64(now.Nanosecond())/1e9
	elapsedFraction := elapsedSeconds / float64(windowSeconds)
	if elapsedFraction > 1 {
		elapsedFraction = 1
	}
	if elapsedFraction < 0 {
		elapsedFraction = 0
	}

	estimate := float64(prevCount)*(1-elapsedFraction) + float64(curCount)
	remaining := l.limit - int64(estimate)
	if remaining < 0 {
		remaining = 0
	}
	resetAt := time.Unix((curWindow+1)*windowSeconds, 0)

	return Decision{
		Allowed:    estimate <= float64(l.limit),
		Limit:      l.limit,
		Remaining:  remaining,
		RetryAfter: time.Until(resetAt),
		ResetAt:    resetAt,
	}, nil
}

func (l *SlidingWindowCounterLimiter) failureDecision() Decision {
	if l.failMode == FailClosed {
		return closedDecision(l.limit)
	}
	return openDecision(l.limit)
}
