package ports

import (
	"context"
	"time"
)

// SessionStore keeps the short-lived security state that must outlive a request but not a
// release: failed-login counters, revoked refresh tokens and per-user "no token issued before"
// marks. Unlike CachePort it is never invalidated by prefix.
type SessionStore interface {
	// RecordLoginFailure counts one failure for key and returns the total in the current
	// window. The window starts at the first failure and is not extended by later ones.
	RecordLoginFailure(ctx context.Context, key string, window time.Duration) (int, error)
	LoginFailures(ctx context.Context, key string) (int, error)
	ClearLoginFailures(ctx context.Context, key string) error

	// RevokeToken denies the token id until ttl has passed (its own expiry).
	RevokeToken(ctx context.Context, tokenID string, ttl time.Duration) error
	IsTokenRevoked(ctx context.Context, tokenID string) (bool, error)

	// SetTokensValidAfter makes every token issued before at invalid for ttl. A zero result
	// from TokensValidAfter means no mark is set.
	SetTokensValidAfter(ctx context.Context, userID string, at time.Time, ttl time.Duration) error
	TokensValidAfter(ctx context.Context, userID string) (time.Time, error)
}
