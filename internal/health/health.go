// Package health implements Kubernetes-style liveness and readiness checks.
package health

import (
	"context"
	"net/http"
	"time"
)

// Pinger is satisfied by our redis store wrapper; kept minimal so this
// package doesn't need to import the ratelimit package directly.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Liveness always returns 200 if the process is alive enough to handle
// HTTP at all — Kubernetes restarts the pod if this stops responding.
func Liveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Readiness returns 200 only if Redis is reachable — Kubernetes stops
// routing traffic to this pod (without restarting it) if this fails.
func Readiness(pinger Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pinger.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("redis unreachable: " + err.Error()))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
