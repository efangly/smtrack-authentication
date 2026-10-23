package upload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// keyVersion is the first byte of every stored object. It names the key that
// sealed the rest, so a rotated key can be added later without re-encrypting
// what is already stored.
const keyVersion byte = 1

// KeySize is the length of the AES-256 key IMAGE_ENCRYPTION_KEY must decode to.
const KeySize = 32

// imageCipher seals and opens pictures with AES-256-GCM.
// Stored layout: [1 byte key version][12 byte nonce][ciphertext + tag].
type imageCipher struct {
	aead cipher.AEAD
}

// ParseKey decodes the base64 IMAGE_ENCRYPTION_KEY and checks its length.
func ParseKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("IMAGE_ENCRYPTION_KEY must be base64")
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("IMAGE_ENCRYPTION_KEY must decode to %d bytes", KeySize)
	}
	return key, nil
}

func newImageCipher(key []byte) (*imageCipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &imageCipher{aead: aead}, nil
}

func (c *imageCipher) seal(plain []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	out := make([]byte, 0, 1+len(nonce)+len(plain)+c.aead.Overhead())
	out = append(out, keyVersion)
	out = append(out, nonce...)
	// The version byte is authenticated too, so it cannot be swapped.
	return c.aead.Seal(out, nonce, plain, []byte{keyVersion}), nil
}

// open errors are plain (unwrapped) on purpose: callers log them, and a
// corrupted or foreign object must not read as a driver failure.
func (c *imageCipher) open(sealed []byte) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(sealed) < 1+n+c.aead.Overhead() {
		return nil, errors.New("encrypted image is too short")
	}
	if sealed[0] != keyVersion {
		return nil, fmt.Errorf("unknown image key version %d", sealed[0])
	}
	plain, err := c.aead.Open(nil, sealed[1:1+n], sealed[1+n:], sealed[:1])
	if err != nil {
		return nil, errors.New("image could not be decrypted")
	}
	return plain, nil
}
