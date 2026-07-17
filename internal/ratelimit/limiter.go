// Package ratelimit implements the four distributed rate-limiting algorithms
// (fixed window, sliding window log, sliding window counter, token bucket),
// all backed by Redis so the limit is enforced correctly across any number
// of stateless gateway replicas.
package ratelimit

import (
	"context"
	"time"
)

// Decision is the result of a single Allow() call.
type Decision struct {
	Allowed    bool
	Limit      int64
	Remaining  int64
	RetryAfter time.Duration
	ResetAt    time.Time
}

// Limiter is implemented by every rate-limiting algorithm. clientID and route
// together form the counting key's identity; limit/window (or capacity/refill
// for token bucket, via WindowOpts) define the policy being enforced.
type Limiter interface {
	Allow(ctx context.Context, clientID, route string) (Decision, error)
}

// FailMode controls gateway behavior when Redis is unreachable.
type FailMode string

const (
	FailOpen   FailMode = "open"
	FailClosed FailMode = "closed"
)

// openDecision is what every algorithm returns when Redis is down and the
// configured fail mode is "open" (let traffic through rather than block it).
func openDecision(limit int64) Decision {
	return Decision{Allowed: true, Limit: limit, Remaining: limit}
}

// closedDecision is what every algorithm returns when Redis is down and the
// configured fail mode is "closed" (reject rather than risk unlimited traffic).
func closedDecision(limit int64) Decision {
	return Decision{Allowed: false, Limit: limit, Remaining: 0, RetryAfter: time.Second}
}
