package admin

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// RedisClientStore implements ClientStore against the
// config:client:{client_id} hash described in the architecture doc:
// { tier, custom_limit_override }. It doubles as the auth middleware's
// api-key -> client lookup via config:apikey:{key} -> client_id.
type RedisClientStore struct {
	rdb *redis.Client
}

func NewRedisClientStore(rdb *redis.Client) *RedisClientStore {
	return &RedisClientStore{rdb: rdb}
}

func clientKey(clientID string) string { return "config:client:" + clientID }
func apiKeyKey(apiKey string) string   { return "config:apikey:" + apiKey }

func (s *RedisClientStore) GetClient(ctx context.Context, clientID string) (tier string, usage, limit int64, found bool) {
	vals, err := s.rdb.HGetAll(ctx, clientKey(clientID)).Result()
	if err != nil || len(vals) == 0 {
		return "", 0, 0, false
	}
	tier = vals["tier"]
	limit, _ = strconv.ParseInt(vals["custom_limit_override"], 10, 64)
	usage, _ = strconv.ParseInt(vals["usage"], 10, 64)
	return tier, usage, limit, true
}

func (s *RedisClientStore) SetClientLimit(ctx context.Context, clientID string, limit int64) error {
	return s.rdb.HSet(ctx, clientKey(clientID), "custom_limit_override", limit).Err()
}

// Lookup implements middleware.ClientLookup: resolves an API key to a
// client_id + tier via the config:apikey:{key} -> config:client:{id} chain.
func (s *RedisClientStore) Lookup(ctx context.Context, apiKey string) (clientID, tier string, found bool) {
	id, err := s.rdb.Get(ctx, apiKeyKey(apiKey)).Result()
	if err != nil || id == "" {
		return "", "", false
	}
	vals, err := s.rdb.HGetAll(ctx, clientKey(id)).Result()
	if err != nil || len(vals) == 0 {
		return id, "default", true
	}
	return id, vals["tier"], true
}
