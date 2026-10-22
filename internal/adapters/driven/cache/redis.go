package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// scanBatchSize is the COUNT hint given to each SCAN round.
const scanBatchSize = 200

type redisAdapter struct {
	client *redis.Client
}

// NewRedisAdapter connects to Redis and returns the one client as both the cache and the
// session store (failed logins, revoked tokens), which share a server but not a keyspace:
// session keys start with "auth:", and cache invalidation is by other prefixes.
func NewRedisAdapter(cfg *config.Config) (ports.CachePort, ports.SessionStore) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost,
		Password: cfg.RedisPassword,
		DB:       0,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Warn("Redis connection error", "error", err)
	} else {
		logger.Info("Redis connected successfully")
	}

	r := &redisAdapter{client: client}
	return r, r
}

// Values are stored as JSON.
func (r *redisAdapter) Set(ctx context.Context, key string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("cache encode %s: %w", key, err)
	}
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache set %s: %w", key, err)
	}
	return nil
}

func (r *redisAdapter) Get(ctx context.Context, key string, dest any) (bool, error) {
	data, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cache get %s: %w", key, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("cache decode %s: %w", key, err)
	}
	return true, nil
}

// Del removes every key starting with prefix. It uses SCAN rather than KEYS
// so a large keyspace does not block the Redis server.
func (r *redisAdapter) Del(ctx context.Context, prefix string) error {
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, prefix+"*", scanBatchSize).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		if next == 0 {
			return nil
		}
		cursor = next
	}
}

// Close releases the connection pool.
func (r *redisAdapter) Close() error {
	return r.client.Close()
}
