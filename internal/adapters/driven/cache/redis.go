package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tng-coop/auth-service/config"
	"github.com/tng-coop/auth-service/internal/core/ports"
	"github.com/tng-coop/auth-service/pkg/logger"
)

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
		client.FlushAll(ctx)
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

func (r *redisAdapter) Del(ctx context.Context, pattern string) error {
	keys, err := r.client.Keys(ctx, pattern+"*").Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return r.client.Del(ctx, keys...).Err()
	}
	return nil
}
