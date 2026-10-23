package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	failuresPrefix = "auth:fail:"
	revokedPrefix  = "auth:revoked:"
	validAfterKey  = "auth:valid-after:"
)

// incrWithWindow adds one and starts the expiry only when the key is new, so later failures
// do not extend the window.
var incrWithWindow = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return n`)

func (r *redisAdapter) RecordLoginFailure(ctx context.Context, key string, window time.Duration) (int, error) {
	n, err := incrWithWindow.Run(ctx, r.client, []string{failuresPrefix + key}, window.Milliseconds()).Int()
	if err != nil {
		return 0, fmt.Errorf("record login failure: %w", err)
	}
	return n, nil
}

func (r *redisAdapter) LoginFailures(ctx context.Context, key string) (int, error) {
	n, err := r.client.Get(ctx, failuresPrefix+key).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read login failures: %w", err)
	}
	return n, nil
}

func (r *redisAdapter) ClearLoginFailures(ctx context.Context, key string) error {
	if err := r.client.Del(ctx, failuresPrefix+key).Err(); err != nil {
		return fmt.Errorf("clear login failures: %w", err)
	}
	return nil
}

func (r *redisAdapter) RevokeToken(ctx context.Context, tokenID string, ttl time.Duration) error {
	if err := r.client.Set(ctx, revokedPrefix+tokenID, 1, ttl).Err(); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return nil
}

func (r *redisAdapter) IsTokenRevoked(ctx context.Context, tokenID string) (bool, error) {
	n, err := r.client.Exists(ctx, revokedPrefix+tokenID).Result()
	if err != nil {
		return false, fmt.Errorf("check token revoked: %w", err)
	}
	return n > 0, nil
}

func (r *redisAdapter) SetTokensValidAfter(ctx context.Context, userID string, at time.Time, ttl time.Duration) error {
	if err := r.client.Set(ctx, validAfterKey+userID, at.Unix(), ttl).Err(); err != nil {
		return fmt.Errorf("set tokens valid after: %w", err)
	}
	return nil
}

func (r *redisAdapter) TokensValidAfter(ctx context.Context, userID string) (time.Time, error) {
	v, err := r.client.Get(ctx, validAfterKey+userID).Result()
	if errors.Is(err, redis.Nil) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("read tokens valid after: %w", err)
	}
	unix, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("read tokens valid after: %w", err)
	}
	return time.Unix(unix, 0), nil
}
