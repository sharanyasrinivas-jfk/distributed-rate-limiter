package ratelimit

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupMiniredis gives each test its own fast in-memory fake Redis —
// no network calls, runs in milliseconds, perfect for algorithm unit tests.
func setupMiniredis(t *testing.T) *Store {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return NewStore(client)
}

// setupRealRedis points at a real Redis instance for the concurrency tests,
// which need genuine atomicity guarantees (miniredis is single-threaded and
// would mask races that only show up against real Redis under load).
// Set RATELIMIT_TEST_REDIS_ADDR to override (defaults to localhost:6379).
func setupRealRedis(t *testing.T) *Store {
	t.Helper()
	addr := os.Getenv("RATELIMIT_TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("real redis not available at %s, skipping concurrency test: %v", addr, err)
	}
	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
	})
	return NewStore(client)
}
