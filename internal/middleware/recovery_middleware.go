package middleware

import (
	"log/slog"
	"net/http"
)

// Recovery wraps a handler so a panic anywhere downstream becomes a 500
// response instead of crashing the whole gateway process — one bad request
// should never take down every other in-flight request.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "err", err, "path", r.URL.Path)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
