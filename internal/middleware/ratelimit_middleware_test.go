package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/yourname/distributed-rate-limiter/internal/config"
	"github.com/yourname/distributed-rate-limiter/internal/ratelimit"
)

type staticLookup struct{}

func (staticLookup) Lookup(ctx context.Context, apiKey string) (string, string, bool) {
	if apiKey == "known-key" {
		return "client1", "default", true
	}
	return "", "", false
}

func TestRateLimitMiddleware_EnforcesLimit(t *testing.T) {
	mr, _ := miniredis.Run()
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := ratelimit.NewStore(rdb)

	cfg := &config.Config{
		Routes: []config.Route{
			{
				PathPrefix: "/api/orders",
				Backend:    "http://backend",
				Algorithm:  config.FixedWindow,
				Limits: map[string]config.Limit{
					"default": {Requests: 2, WindowSeconds: 60},
				},
			},
		},
	}

	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := Auth(staticLookup{})(RateLimit(cfg, store)(final))

	codes := []int{}
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
		req.Header.Set("X-API-Key", "known-key")
		rec := httptest.NewRecorder()
		chain.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}

	if codes[0] != 200 || codes[1] != 200 {
		t.Fatalf("expected first 2 requests allowed, got %v", codes)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("expected 3rd request to be 429, got %d", codes[2])
	}
}

func TestAuth_UnknownKeyFallsBackToIP(t *testing.T) {
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := ClientIDFromContext(r.Context())
		tier := TierFromContext(r.Context())
		if tier != "unauthenticated" {
			t.Errorf("expected unauthenticated tier, got %s", tier)
		}
		if clientID == "" {
			t.Error("expected a fallback client id")
		}
	})
	chain := Auth(staticLookup{})(final)

	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.RemoteAddr = "1.2.3.4:5555"
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
}
