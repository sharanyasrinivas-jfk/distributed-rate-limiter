package config

import "testing"

const sampleYAML = `
routes:
  - path_prefix: /api/orders
    backend: http://orders-service:8081
    algorithm: sliding_window_counter
    limits:
      default: { requests: 100, window_seconds: 60 }
      pro:     { requests: 1000, window_seconds: 60 }
  - path_prefix: /api/users
    backend: http://users-service:8082
    algorithm: token_bucket
    limits:
      default: { capacity: 20, refill_rate_per_sec: 1 }
redis:
  addr: "redis:6379"
  fail_mode: "open"
`

func TestParse(t *testing.T) {
	cfg, err := Parse([]byte(sampleYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(cfg.Routes))
	}
	if cfg.Redis.Addr != "redis:6379" {
		t.Errorf("expected redis addr redis:6379, got %s", cfg.Redis.Addr)
	}
	if cfg.Routes[0].Algorithm != SlidingWindowCounter {
		t.Errorf("expected sliding_window_counter, got %s", cfg.Routes[0].Algorithm)
	}
}

func TestMatchRoute(t *testing.T) {
	cfg, _ := Parse([]byte(sampleYAML))
	r := cfg.MatchRoute("/api/orders/123")
	if r == nil {
		t.Fatal("expected a route match")
	}
	if r.Backend != "http://orders-service:8081" {
		t.Errorf("wrong backend matched: %s", r.Backend)
	}

	none := cfg.MatchRoute("/api/unknown")
	if none != nil {
		t.Error("expected no match for unknown path")
	}
}

func TestLimitFor(t *testing.T) {
	cfg, _ := Parse([]byte(sampleYAML))
	r := cfg.MatchRoute("/api/orders")

	proLimit, ok := r.LimitFor("pro")
	if !ok || proLimit.Requests != 1000 {
		t.Errorf("expected pro limit of 1000, got %+v", proLimit)
	}

	defaultLimit, ok := r.LimitFor("unknown-tier")
	if !ok || defaultLimit.Requests != 100 {
		t.Errorf("expected fallback to default limit of 100, got %+v", defaultLimit)
	}
}

func TestDefaultFailMode(t *testing.T) {
	cfg, _ := Parse([]byte(`routes: []`))
	if cfg.Redis.FailMode != "open" {
		t.Errorf("expected default fail_mode 'open', got %s", cfg.Redis.FailMode)
	}
}
