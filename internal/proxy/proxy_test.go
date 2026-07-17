package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourname/distributed-rate-limiter/internal/config"
)

func TestProxy_ForwardsToMatchedBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello from backend"))
	}))
	defer backend.Close()

	cfg := &config.Config{Routes: []config.Route{
		{PathPrefix: "/api/orders", Backend: backend.URL},
	}}
	p := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/orders/123", nil)
	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "hello from backend" {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestProxy_NoRouteMatch_Returns404(t *testing.T) {
	cfg := &config.Config{Routes: []config.Route{}}
	p := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
