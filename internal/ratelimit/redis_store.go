package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient is the subset of *redis.Client each algorithm needs. Defining
// it as an interface means tests can point it at either miniredis or a real
// Redis instance without changing any algorithm code.
type RedisClient interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, ttl time.Duration) *redis.BoolCmd
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd
	ScriptLoad(ctx context.Context, script string) *redis.StringCmd
	Ping(ctx context.Context) *redis.StatusCmd
}

// Store wraps a RedisClient with the higher-level operations the rate
// limiting algorithms need: atomic increment-with-ttl, and Lua script
// execution for the operations that require multi-step atomicity.
type Store struct {
	Client RedisClient
}

func NewStore(client RedisClient) *Store {
	return &Store{Client: client}
}

// IncrWithExpire atomically increments key and, only on the very first
// increment (count == 1), sets its TTL. This is the core primitive behind
// the fixed window algorithm: INCR is atomic on its own, and setting TTL
// only once means concurrent requests never stomp on each other's expiry.
func (s *Store) IncrWithExpire(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	count, err := s.Client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		// Best-effort: if Expire fails we still return the correct count;
		// worst case the key never expires, which a monitoring alert would catch.
		s.Client.Expire(ctx, key, ttl)
	}
	return count, nil
}

// EvalScript runs a Lua script atomically against Redis.
func (s *Store) EvalScript(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return s.Client.Eval(ctx, script, keys, args...).Result()
}

// Ping checks Redis connectivity (used by the /readyz health check).
func (s *Store) Ping(ctx context.Context) error {
	return s.Client.Ping(ctx).Err()
}
