package ports

import "context"

// CachePort defines the driven port for caching operations
type CachePort interface {
	Set(ctx context.Context, key string, value string, expireSeconds int) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, pattern string) error
}
