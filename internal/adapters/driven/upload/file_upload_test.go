package upload

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type received struct {
	path        string
	disposition string
	filename    string
	contentType string
	data        []byte
	headers     http.Header
}

// imageServer stands in for the image service and records the part it got.
func imageServer(t *testing.T, got *received) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		reader, err := r.MultipartReader()
		require.NoError(t, err)
		part, err := reader.NextPart()
		require.NoError(t, err)
		got.disposition = part.Header.Get("Content-Disposition")
		got.filename = part.FileName()
		got.contentType = part.Header.Get("Content-Type")
		got.headers = http.Header(part.Header)
		got.data, err = io.ReadAll(part)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"path":"/img/user/stored.png"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestUpload_SendsFileWithDetectedType(t *testing.T) {
	var got received
	srv := imageServer(t, &got)
	adapter := NewFileUploadAdapter(&config.Config{UploadPath: srv.URL})

	url, err := adapter.Upload(context.Background(), &domain.File{
		Name: "pic.png", ContentType: "image/png", Data: []byte("\x89PNG-data"),
	}, "user")
	require.NoError(t, err)

	assert.Equal(t, srv.URL+"/img/user/stored.png", url)
	assert.Equal(t, "/api/image/user", got.path)
	assert.Equal(t, "pic.png", got.filename)
	assert.Equal(t, "image/png", got.contentType)
	assert.Equal(t, []byte("\x89PNG-data"), got.data)
}

func TestUpload_FilenameCannotInjectHeaders(t *testing.T) {
	var got received
	srv := imageServer(t, &got)
	adapter := NewFileUploadAdapter(&config.Config{UploadPath: srv.URL})

	_, err := adapter.Upload(context.Background(), &domain.File{
		Name: "evil\"\r\nX-Injected: yes\r\n.png", ContentType: "image/png", Data: []byte("x"),
	}, "user")
	require.NoError(t, err)

	assert.Empty(t, got.headers.Get("X-Injected"), "client filename injected a header")
	_, params, err := mime.ParseMediaType(got.disposition)
	require.NoError(t, err)
	assert.Equal(t, "file", params["name"])
	assert.NotContains(t, params["filename"], "\r")
	assert.NotContains(t, params["filename"], "\n")
}

func TestUpload_HonoursContext(t *testing.T) {
	var got received
	srv := imageServer(t, &got)
	adapter := NewFileUploadAdapter(&config.Config{UploadPath: srv.URL})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := adapter.Upload(ctx, &domain.File{Name: "a.png", ContentType: "image/png", Data: []byte("x")}, "user")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSafeFilename(t *testing.T) {
	tests := map[string]string{
		"pic.png":                "pic.png",
		"../../etc/passwd":       "passwd",
		`C:\Users\me\pic.png`:    "pic.png",
		"a\r\nb.png":             "ab.png",
		"":                       "upload",
		"/":                      "upload",
		"ภาพถ่าย.jpg":            "ภาพถ่าย.jpg",
		"name with \"quotes\".j": "name with \"quotes\".j",
	}
	for in, want := range tests {
		assert.Equal(t, want, safeFilename(in), in)
	}
}

// Non-ASCII names survive the trip; FormatMediaType encodes them per RFC 2231.
func TestUpload_NonASCIIFilenameRoundTrips(t *testing.T) {
	var got received
	srv := imageServer(t, &got)
	adapter := NewFileUploadAdapter(&config.Config{UploadPath: srv.URL})

	_, err := adapter.Upload(context.Background(), &domain.File{Name: "ภาพ.png", ContentType: "image/png", Data: []byte("x")}, "user")
	require.NoError(t, err)
	assert.Equal(t, "ภาพ.png", got.filename)
}
