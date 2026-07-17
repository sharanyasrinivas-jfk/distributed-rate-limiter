package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/yourname/distributed-rate-limiter/internal/circuitbreaker"
	"github.com/yourname/distributed-rate-limiter/internal/config"
)

// CircuitBreaker keeps one Breaker per backend (matched via the route
// config) and short-circuits with 503 when a backend's breaker is open,
// so a struggling backend fails fast instead of piling up hanging requests.
func CircuitBreaker(cfg *config.Config, failureThreshold int) func(http.Handler) http.Handler {
	var mu sync.Mutex
	breakers := make(map[string]*circuitbreaker.Breaker)

	breakerFor := func(backend string) *circuitbreaker.Breaker {
		mu.Lock()
		defer mu.Unlock()
		if b, ok := breakers[backend]; ok {
			return b
		}
		b := circuitbreaker.New(failureThreshold, 5*time.Second)
		breakers[backend] = b
		return b
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := cfg.MatchRoute(r.URL.Path)
			if route == nil {
				next.ServeHTTP(w, r)
				return
			}
			b := breakerFor(route.Backend)
			if !b.Allow() {
				http.Error(w, "backend circuit open, failing fast", http.StatusServiceUnavailable)
				return
			}

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			if rec.status >= 500 {
				b.RecordFailure()
			} else {
				b.RecordSuccess()
			}
		})
	}
}
