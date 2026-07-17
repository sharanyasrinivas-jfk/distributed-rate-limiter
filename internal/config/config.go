// Package config loads the gateway's static configuration from YAML.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Algorithm identifies which rate-limiting strategy a route uses.
type Algorithm string

const (
	FixedWindow           Algorithm = "fixed_window"
	SlidingWindowLog       Algorithm = "sliding_window_log"
	SlidingWindowCounter   Algorithm = "sliding_window_counter"
	TokenBucket            Algorithm = "token_bucket"
)

// Limit describes the numeric parameters for a rate-limiting tier.
// Requests/WindowSeconds are used by window-based algorithms;
// Capacity/RefillRatePerSec are used by the token bucket.
type Limit struct {
	Requests         int64 `yaml:"requests,omitempty"`
	WindowSeconds    int64 `yaml:"window_seconds,omitempty"`
	Capacity         int64 `yaml:"capacity,omitempty"`
	RefillRatePerSec int64 `yaml:"refill_rate_per_sec,omitempty"`
}

// Route maps a path prefix to a backend, an algorithm, and per-tier limits.
type Route struct {
	PathPrefix string           `yaml:"path_prefix"`
	Backend    string           `yaml:"backend"`
	Algorithm  Algorithm        `yaml:"algorithm"`
	Limits     map[string]Limit `yaml:"limits"`
}

// RedisConfig holds connection and failure-mode settings.
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	FailMode string `yaml:"fail_mode"` // "open" or "closed"
}

// Config is the fully parsed configs/config.yaml.
type Config struct {
	Routes []Route     `yaml:"routes"`
	Redis  RedisConfig `yaml:"redis"`
}

// Load reads and parses a YAML config file from disk.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses raw YAML bytes into a Config. Split out from Load so tests
// can exercise parsing without touching the filesystem.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Redis.FailMode == "" {
		cfg.Redis.FailMode = "open"
	}
	return &cfg, nil
}

// MatchRoute finds the first configured route whose PathPrefix matches the
// given request path. Returns nil if no route matches.
func (c *Config) MatchRoute(path string) *Route {
	for i := range c.Routes {
		r := &c.Routes[i]
		if len(path) >= len(r.PathPrefix) && path[:len(r.PathPrefix)] == r.PathPrefix {
			return r
		}
	}
	return nil
}

// LimitFor returns the Limit for a given tier, falling back to "default".
func (r *Route) LimitFor(tier string) (Limit, bool) {
	if l, ok := r.Limits[tier]; ok {
		return l, true
	}
	l, ok := r.Limits["default"]
	return l, ok
}
