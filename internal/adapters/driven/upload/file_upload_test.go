package upload

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStore stands in for MinIO.
type memStore struct {
	objects map[string][]byte
	failPut bool
}

func (m *memStore) Put(_ context.Context, key string, data []byte) error {
	if m.failPut {
		return errors.New("minio down")
	}
	m.objects[key] = data
	return nil
}
func (m *memStore) Get(_ context.Context, key string) ([]byte, error) {
	d, ok := m.objects[key]
	if !ok {
		return nil, errors.New("image not found")
	}
	return d, nil
}
func (m *memStore) Remove(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

func newTestAdapter(t *testing.T) (*fileUploadAdapter, *memStore) {
	t.Helper()
	c, err := newImageCipher(make([]byte, KeySize))
	require.NoError(t, err)
	store := &memStore{objects: map[string][]byte{}}
	return &fileUploadAdapter{store: store, cipher: c, baseURL: "https://host/auth/image"}, store
}

var png = &domain.File{Name: "../evil.png", ContentType: "image/png", Data: []byte("\x89PNG-data")}

func TestUpload_StoresEncryptedUnderGeneratedName(t *testing.T) {
	a, store := newTestAdapter(t)

	url, err := a.Upload(context.Background(), png, "user")
	require.NoError(t, err)

	assert.Regexp(t, regexp.MustCompile(`^https://host/auth/image/user/[0-9a-f-]{36}\.png$`), url)
	require.Len(t, store.objects, 1)
	for key, data := range store.objects {
		assert.Contains(t, url, key)
		assert.NotContains(t, string(data), "PNG-data", "stored bytes must be ciphertext")
	}
}

func TestUpload_RoundTrip(t *testing.T) {
	a, _ := newTestAdapter(t)
	url, err := a.Upload(context.Background(), png, "tenant")
	require.NoError(t, err)

	got, err := a.Download(context.Background(), "tenant", url[len("https://host/auth/image/tenant/"):])
	require.NoError(t, err)
	assert.Equal(t, png.Data, got.Data)
	assert.Equal(t, "image/png", got.ContentType)
}

func TestUpload_Rejections(t *testing.T) {
	a, store := newTestAdapter(t)
	_, err := a.Upload(context.Background(), &domain.File{ContentType: "image/gif", Data: []byte("x")}, "user")
	assert.EqualError(t, err, "unsupported image type")
	_, err = a.Upload(context.Background(), png, "../etc")
	assert.Error(t, err)
	assert.Empty(t, store.objects)

	store.failPut = true
	_, err = a.Upload(context.Background(), png, "user")
	assert.Error(t, err)
}

func TestDelete_UsesLastSegmentAndIgnoresLegacyURLs(t *testing.T) {
	a, store := newTestAdapter(t)
	url, err := a.Upload(context.Background(), png, "user")
	require.NoError(t, err)

	require.NoError(t, a.Delete("user", "https://old-image-service/img/user/other.png"))
	assert.Len(t, store.objects, 1, "a legacy URL removes nothing here")

	require.NoError(t, a.Delete("user", url))
	assert.Empty(t, store.objects)
	assert.NoError(t, a.Delete("user", ""))
	assert.Error(t, a.Delete("nope", "x.png"))
}

func TestDownload_RejectsBadNamesAndMissing(t *testing.T) {
	a, _ := newTestAdapter(t)
	for _, tc := range [][2]string{{"user", "../x.png"}, {"other", "x.png"}, {"user", ""}, {"user", "missing.png"}} {
		_, err := a.Download(context.Background(), tc[0], tc[1])
		assert.EqualError(t, err, "image not found", tc)
	}
}

func TestUnconfigured(t *testing.T) {
	a, err := New(&configEmpty)
	require.NoError(t, err)
	_, err = a.Upload(context.Background(), png, "user")
	assert.ErrorIs(t, err, errNotConfigured)
}
