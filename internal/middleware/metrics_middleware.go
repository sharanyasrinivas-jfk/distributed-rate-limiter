package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/yourname/distributed-rate-limiter/internal/metrics"
)

// Metrics records request counts (by path+status) and latency into the
// Prometheus registry. Placed outermost in the chain so it captures the
// final status code, including 429s and 5xxs from inner middleware.
func Metrics(reg *metrics.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			labels := map[string]string{
				"path":   r.URL.Path,
				"status": strconv.Itoa(rec.status),
			}
			reg.IncCounter("gateway_requests_total", "Total requests handled by the gateway", labels)
			reg.ObserveHistogramLike("gateway_request_duration_seconds", "Request duration in seconds",
				map[string]string{"path": r.URL.Path}, time.Since(start).Seconds())
		})
	}
}
