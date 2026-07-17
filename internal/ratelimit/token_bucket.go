package ratelimit

import (
	"context"
	"fmt"
	"time"
)

// tokenBucketScript atomically: computes elapsed time since the last refill,
// adds tokens accordingly (capped at capacity), and consumes one token if
// available. Doing refill-math + consume in one script is what prevents two
// concurrent requests from both reading "3 tokens available" and both
// succeeding when only one token should have been left.
const tokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate_per_sec = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local ttl_seconds = tonumber(ARGV[4])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill_ms')
local tokens = tonumber(bucket[1])
local last_refill_ms = tonumber(bucket[2])

if tokens == nil then
    tokens = capacity
    last_refill_ms = now_ms
end

local elapsed_sec = math.max(0, (now_ms - last_refill_ms) / 1000)
local refill = elapsed_sec * refill_rate_per_sec
tokens = math.min(capacity, tokens + refill)

local allowed = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
end

redis.call('HMSET', key, 'tokens', tokens, 'last_refill_ms', now_ms)
redis.call('EXPIRE', key, ttl_seconds)

return {allowed, tostring(tokens)}
`

// TokenBucketLimiter stores {tokens, last_refill_ts} per client/route and
// refills continuously at a fixed rate, capped at capacity. Smooths bursts
// better than any window-based algorithm because it allows a controlled
// burst up to the bucket's capacity, then throttles to the steady refill rate.
type TokenBucketLimiter struct {
	store            *Store
	capacity         int64
	refillRatePerSec int64
	failMode         FailMode
}

func NewTokenBucketLimiter(store *Store, capacity, refillRatePerSec int64, failMode FailMode) *TokenBucketLimiter {
	return &TokenBucketLimiter{store: store, capacity: capacity, refillRatePerSec: refillRatePerSec, failMode: failMode}
}

func (l *TokenBucketLimiter) Allow(ctx context.Context, clientID, route string) (Decision, error) {
	key := fmt.Sprintf("ratelimit:token_bucket:%s:%s", clientID, route)
	nowMs := time.Now().UnixMilli()
	// Give the bucket a generous TTL so an idle client's state doesn't
	// evaporate mid-burst, but still self-cleans if abandoned entirely.
	ttlSeconds := int64(3600)
	if l.refillRatePerSec > 0 {
		fullRefillSeconds := l.capacity/l.refillRatePerSec + 60
		if fullRefillSeconds > ttlSeconds {
			ttlSeconds = fullRefillSeconds
		}
	}

	res, err := l.store.EvalScript(ctx, tokenBucketScript, []string{key}, l.capacity, l.refillRatePerSec, nowMs, ttlSeconds)
	if err != nil {
		return l.failureDecision(), nil
	}
	result, ok := res.([]interface{})
	if !ok || len(result) != 2 {
		return l.failureDecision(), nil
	}
	allowed := toInt64(result[0]) == 1

	var retryAfter time.Duration
	if !allowed && l.refillRatePerSec > 0 {
		retryAfter = time.Second / time.Duration(l.refillRatePerSec)
	}

	return Decision{
		Allowed:    allowed,
		Limit:      l.capacity,
		RetryAfter: retryAfter,
	}, nil
}

func (l *TokenBucketLimiter) failureDecision() Decision {
	if l.failMode == FailClosed {
		return closedDecision(l.capacity)
	}
	return openDecision(l.capacity)
}
