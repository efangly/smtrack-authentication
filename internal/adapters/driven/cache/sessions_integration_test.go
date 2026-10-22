//go:build integration

package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Run with REDIS_TEST_ADDR=localhost:6379 go test -tags integration ./internal/adapters/driven/cache/
func TestSessionStore(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR not set")
	}
	ctx := context.Background()
	c, store := NewRedisAdapter(&config.Config{RedisHost: addr})

	// The failure window starts at the first failure and is not extended by later ones.
	n, err := store.RecordLoginFailure(ctx, "alice", 300*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	time.Sleep(200 * time.Millisecond)
	n, _ = store.RecordLoginFailure(ctx, "alice", 300*time.Millisecond)
	assert.Equal(t, 2, n)
	time.Sleep(150 * time.Millisecond)
	n, err = store.LoginFailures(ctx, "alice")
	require.NoError(t, err)
	assert.Zero(t, n, "window must expire 300ms after the first failure")

	_, _ = store.RecordLoginFailure(ctx, "bob", time.Minute)
	require.NoError(t, store.ClearLoginFailures(ctx, "bob"))
	n, _ = store.LoginFailures(ctx, "bob")
	assert.Zero(t, n)

	revoked, err := store.IsTokenRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.False(t, revoked)
	require.NoError(t, store.RevokeToken(ctx, "jti-1", time.Minute))
	revoked, _ = store.IsTokenRevoked(ctx, "jti-1")
	assert.True(t, revoked)

	zero, err := store.TokensValidAfter(ctx, "U1")
	require.NoError(t, err)
	assert.True(t, zero.IsZero())
	at := time.Now().Truncate(time.Second)
	require.NoError(t, store.SetTokensValidAfter(ctx, "U1", at, time.Minute))
	got, _ := store.TokensValidAfter(ctx, "U1")
	assert.True(t, got.Equal(at))

	// Cache invalidation by prefix must never touch security state.
	require.NoError(t, c.Del(ctx, "user"))
	require.NoError(t, c.Del(ctx, "tenant"))
	revoked, _ = store.IsTokenRevoked(ctx, "jti-1")
	assert.True(t, revoked)
}
