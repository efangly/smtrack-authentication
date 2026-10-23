package upload

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var configEmpty = config.Config{}

func testCipher(t *testing.T, fill byte) *imageCipher {
	t.Helper()
	c, err := newImageCipher(bytes.Repeat([]byte{fill}, KeySize))
	require.NoError(t, err)
	return c
}

func TestCipher_RoundTrip(t *testing.T) {
	c := testCipher(t, 1)
	sealed, err := c.seal([]byte("picture"))
	require.NoError(t, err)
	assert.Equal(t, keyVersion, sealed[0])
	got, err := c.open(sealed)
	require.NoError(t, err)
	assert.Equal(t, []byte("picture"), got)

	other, _ := c.seal([]byte("picture"))
	assert.NotEqual(t, sealed, other, "nonce must be random")
}

func TestCipher_RejectsTamperingWrongKeyAndVersion(t *testing.T) {
	c := testCipher(t, 1)
	sealed, _ := c.seal([]byte("picture"))

	flipped := append([]byte(nil), sealed...)
	flipped[len(flipped)-1] ^= 0xff
	_, err := c.open(flipped)
	assert.Error(t, err)

	_, err = testCipher(t, 2).open(sealed)
	assert.Error(t, err)

	wrongVersion := append([]byte(nil), sealed...)
	wrongVersion[0] = 9
	_, err = c.open(wrongVersion)
	assert.ErrorContains(t, err, "key version")

	_, err = c.open([]byte{1, 2})
	assert.Error(t, err)
}

func TestParseKey(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(make([]byte, KeySize))
	_, err := ParseKey(good)
	assert.NoError(t, err)
	_, err = ParseKey("not base64!")
	assert.Error(t, err)
	_, err = ParseKey(base64.StdEncoding.EncodeToString(make([]byte, 16)))
	assert.Error(t, err)
}
