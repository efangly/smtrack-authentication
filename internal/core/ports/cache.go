package ports

import (
	"context"
	"time"
)

// CachePort defines the driven port for caching operations. The adapter owns
// the encoding: callers hand it values and get values back.
type CachePort interface {
	// Set stores v under key for ttl. Never pass a value that holds a secret
	// (see domain.User.WithoutSecrets).
	Set(ctx context.Context, key string, v any, ttl time.Duration) error
	// Get decodes the value under key into dest. hit is false when the key
	// is missing; a value that cannot be decoded is reported as an error.
	Get(ctx context.Context, key string, dest any) (hit bool, err error)
	// Del removes every key that starts with prefix.
	Del(ctx context.Context, prefix string) error
}
