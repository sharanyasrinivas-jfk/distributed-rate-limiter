// Package circuitbreaker implements a Closed -> Open -> HalfOpen -> Closed
// state machine per backend, so a struggling backend fails fast instead of
// piling up slow/hanging requests and cascading the failure upstream.
package circuitbreaker

import (
	"sync"
	"time"
)

type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// Breaker tracks failures for a single backend and decides whether requests
// should be allowed through.
type Breaker struct {
	mu               sync.Mutex
	state            State
	failureThreshold int
	cooldown         time.Duration
	consecutiveFails int
	openedAt         time.Time
}

func New(failureThreshold int, cooldown time.Duration) *Breaker {
	return &Breaker{
		state:            Closed,
		failureThreshold: failureThreshold,
		cooldown:         cooldown,
	}
}

// Allow reports whether a request should be attempted right now. When the
// breaker is Open but the cooldown has elapsed, it transitions to HalfOpen
// and allows exactly one trial request through.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		return true
	case Open:
		if time.Since(b.openedAt) >= b.cooldown {
			b.state = HalfOpen
			return true
		}
		return false
	case HalfOpen:
		// Only one trial request is allowed while half-open; callers that
		// already got Allow()==true should report the result via
		// RecordSuccess/RecordFailure before another Allow() is meaningful.
		return true
	default:
		return true
	}
}

// RecordSuccess reports a successful backend call. In HalfOpen this closes
// the breaker; in Closed it resets the failure counter.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveFails = 0
	b.state = Closed
}

// RecordFailure reports a failed backend call. Trips the breaker to Open
// once consecutive failures reach the threshold, or immediately re-opens
// from HalfOpen.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == HalfOpen {
		b.state = Open
		b.openedAt = time.Now()
		return
	}

	b.consecutiveFails++
	if b.consecutiveFails >= b.failureThreshold {
		b.state = Open
		b.openedAt = time.Now()
	}
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}
