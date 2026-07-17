package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/yourname/distributed-rate-limiter/internal/config"
	"github.com/yourname/distributed-rate-limiter/internal/ratelimit"
)

// RateLimit builds (and caches) the configured Limiter for each matched
// route+tier, calls it, and either lets the request through with
// X-RateLimit-* headers set, or short-circuits with 429 + Retry-After.
// Unmatched routes pass through untouched — the proxy layer is responsible
// for returning 404 on those.
func RateLimit(cfg *config.Config, store *ratelimit.Store) func(http.Handler) http.Handler {
	failMode := ratelimit.FailOpen
	if cfg.Redis.FailMode == "closed" {
		failMode = ratelimit.FailClosed
	}

	var mu sync.Mutex
	limiters := make(map[string]ratelimit.Limiter)

	limiterFor := func(route *config.Route, tier string) (ratelimit.Limiter, bool) {
		limitCfg, ok := route.LimitFor(tier)
		if !ok {
			return nil, false
		}
		cacheKey := fmt.Sprintf("%s|%s", route.PathPrefix, tier)

		mu.Lock()
		defer mu.Unlock()
		if l, ok := limiters[cacheKey]; ok {
			return l, true
		}

		var l ratelimit.Limiter
		switch route.Algorithm {
		case config.FixedWindow:
			l = ratelimit.NewFixedWindowLimiter(store, limitCfg.Requests, time.Duration(limitCfg.WindowSeconds)*time.Second, failMode)
		case config.SlidingWindowLog:
			l = ratelimit.NewSlidingWindowLogLimiter(store, limitCfg.Requests, time.Duration(limitCfg.WindowSeconds)*time.Second, failMode)
		case config.SlidingWindowCounter:
			l = ratelimit.NewSlidingWindowCounterLimiter(store, limitCfg.Requests, time.Duration(limitCfg.WindowSeconds)*time.Second, failMode)
		case config.TokenBucket:
			l = ratelimit.NewTokenBucketLimiter(store, limitCfg.Capacity, limitCfg.RefillRatePerSec, failMode)
		default:
			return nil, false
		}
		limiters[cacheKey] = l
		return l, true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := cfg.MatchRoute(r.URL.Path)
			if route == nil {
				next.ServeHTTP(w, r)
				return
			}

			clientID := ClientIDFromContext(r.Context())
			tier := TierFromContext(r.Context())

			limiter, ok := limiterFor(route, tier)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			decision, err := limiter.Allow(r.Context(), clientID, r.URL.Path)
			if err != nil {
				http.Error(w, "rate limiter error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(decision.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
			if !decision.ResetAt.IsZero() {
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))
			}

			if !decision.Allowed {
				retryAfterSec := int(decision.RetryAfter.Seconds())
				if retryAfterSec < 1 {
					retryAfterSec = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
