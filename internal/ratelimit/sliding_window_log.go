package ratelimit

import (
	"context"
	"fmt"
	"time"
)

// slidingWindowLogScript atomically does all of: trim entries older than the
// window, add the current request's timestamp, count what's left, and set an
// expiry so the key self-cleans. Running this as a single Lua script is what
// makes it race-free across replicas — without it, "trim, add, count" done
// as three separate round trips would let concurrent requests interleave.
const slidingWindowLogScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window_ms)
local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now, member)
    redis.call('PEXPIRE', key, window_ms)
    return {1, count + 1}
else
    redis.call('PEXPIRE', key, window_ms)
    return {0, count}
end
`

// SlidingWindowLogLimiter stores every request timestamp in a Redis sorted
// set and counts how many fall within the trailing window. Perfectly
// accurate (no boundary burst issue) but O(N) memory in requests-per-window.
type SlidingWindowLogLimiter struct {
	store    *Store
	limit    int64
	window   time.Duration
	failMode FailMode
}

func NewSlidingWindowLogLimiter(store *Store, limit int64, window time.Duration, failMode FailMode) *SlidingWindowLogLimiter {
	return &SlidingWindowLogLimiter{store: store, limit: limit, window: window, failMode: failMode}
}

func (l *SlidingWindowLogLimiter) Allow(ctx context.Context, clientID, route string) (Decision, error) {
	key := fmt.Sprintf("ratelimit:sliding_log:%s:%s", clientID, route)
	now := time.Now()
	nowMs := now.UnixMilli()
	windowMs := l.window.Milliseconds()
	// A unique member is required so concurrent requests in the same
	// millisecond don't collide as the same sorted-set entry.
	member := fmt.Sprintf("%d-%s", nowMs, randSuffix())

	res, err := l.store.EvalScript(ctx, slidingWindowLogScript, []string{key}, nowMs, windowMs, l.limit, member)
	if err != nil {
		return l.failureDecision(), nil
	}

	result, ok := res.([]interface{})
	if !ok || len(result) != 2 {
		return l.failureDecision(), nil
	}
	allowed := toInt64(result[0]) == 1
	count := toInt64(result[1])

	remaining := l.limit - count
	if remaining < 0 {
		remaining = 0
	}
	resetAt := now.Add(l.window)
	return Decision{
		Allowed:    allowed,
		Limit:      l.limit,
		Remaining:  remaining,
		RetryAfter: l.window,
		ResetAt:    resetAt,
	}, nil
}

func (l *SlidingWindowLogLimiter) failureDecision() Decision {
	if l.failMode == FailClosed {
		return closedDecision(l.limit)
	}
	return openDecision(l.limit)
}
