package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type identityCtxKey string

const (
	clientIDKey identityCtxKey = "client_id"
	tierKey     identityCtxKey = "tier"
)

// ClientLookup resolves an API key to a client identity. In production this
// is backed by Redis (config:client:{key} hash); tests can supply a fake.
type ClientLookup interface {
	// Lookup returns (clientID, tier, found).
	Lookup(ctx context.Context, apiKey string) (clientID string, tier string, found bool)
}

// Auth extracts X-API-Key and resolves it to a client_id + tier for
// downstream rate limiting. Unknown or missing keys fall back to IP-based
// identification at the strictest ("unauthenticated") tier — unauthenticated
// traffic must never bypass limiting entirely.
func Auth(lookup ClientLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")

			var clientID, tier string
			if apiKey != "" {
				if id, t, found := lookup.Lookup(r.Context(), apiKey); found {
					clientID, tier = id, t
				}
			}
			if clientID == "" {
				clientID = "ip:" + clientIP(r)
				tier = "unauthenticated"
			}

			ctx := context.WithValue(r.Context(), clientIDKey, clientID)
			ctx = context.WithValue(ctx, tierKey, tier)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ClientIDFromContext and TierFromContext retrieve the identity Auth resolved.
func ClientIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(clientIDKey).(string); ok {
		return v
	}
	return ""
}

func TierFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(tierKey).(string); ok {
		return v
	}
	return "default"
}
