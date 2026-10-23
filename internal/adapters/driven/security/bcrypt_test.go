package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// testCost is the configured minimum, so the test exercises the production work factor.
const testCost = 12

func TestBcryptHasher(t *testing.T) {
	h := NewBcryptHasher(testCost)

	hash, err := h.Hash("pass1234")
	require.NoError(t, err)
	assert.NotEqual(t, "pass1234", hash)

	cost, err := bcrypt.Cost([]byte(hash))
	require.NoError(t, err)
	assert.Equal(t, testCost, cost)

	assert.True(t, h.Compare(hash, "pass1234"))
	assert.False(t, h.Compare(hash, "wrong"))
	assert.False(t, h.Compare("not-a-hash", "pass1234"))

	// Must not panic and must not match anything.
	h.CompareDummy("pass1234")
}

// Hashes written by the previous implementation must keep verifying.
func TestBcryptHasher_AcceptsExistingHashes(t *testing.T) {
	existing, err := bcrypt.GenerateFromPassword([]byte("legacy-pass"), 10)
	require.NoError(t, err)
	assert.True(t, NewBcryptHasher(testCost).Compare(string(existing), "legacy-pass"))
}
