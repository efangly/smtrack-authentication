package cache

import (
	"context"
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

func NewRedisAdapter(cfg *config.Config) ports.CachePort {
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

	return &redisAdapter{client: client}
}

func (r *redisAdapter) Set(ctx context.Context, key string, value string, expireSeconds int) error {
	return r.client.Set(ctx, key, value, time.Duration(expireSeconds)*time.Second).Err()
}

func (r *redisAdapter) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Del removes every key starting with pattern. It uses SCAN rather than KEYS
// so a large keyspace does not block the Redis server.
func (r *redisAdapter) Del(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern+"*", scanBatchSize).Result()
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
