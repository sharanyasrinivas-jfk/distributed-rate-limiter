package circuitbreaker

import (
	"testing"
	"time"
)

func TestBreaker_TripsAfterThreshold(t *testing.T) {
	b := New(3, 50*time.Millisecond)

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("call %d should be allowed while closed", i+1)
		}
		b.RecordFailure()
	}

	if b.State() != Open {
		t.Fatalf("expected breaker to be Open after %d failures, got %s", 3, b.State())
	}
	if b.Allow() {
		t.Fatal("breaker should reject calls immediately after tripping open")
	}
}

func TestBreaker_HalfOpenAfterCooldown(t *testing.T) {
	b := New(1, 30*time.Millisecond)
	b.Allow()
	b.RecordFailure() // trips open

	if b.Allow() {
		t.Fatal("should still be open before cooldown elapses")
	}

	time.Sleep(40 * time.Millisecond)

	if !b.Allow() {
		t.Fatal("should allow one trial request after cooldown (half-open)")
	}
	if b.State() != HalfOpen {
		t.Fatalf("expected HalfOpen state, got %s", b.State())
	}
}

func TestBreaker_ClosesOnHalfOpenSuccess(t *testing.T) {
	b := New(1, 10*time.Millisecond)
	b.Allow()
	b.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	b.Allow() // transitions to half-open

	b.RecordSuccess()
	if b.State() != Closed {
		t.Fatalf("expected Closed after half-open success, got %s", b.State())
	}
}

func TestBreaker_ReopensOnHalfOpenFailure(t *testing.T) {
	b := New(1, 10*time.Millisecond)
	b.Allow()
	b.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	b.Allow() // half-open

	b.RecordFailure()
	if b.State() != Open {
		t.Fatalf("expected Open after half-open failure, got %s", b.State())
	}
}
