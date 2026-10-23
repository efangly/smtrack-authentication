package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// storeTimeout bounds every call to MinIO. Without it a hung server would pin
// the request goroutine forever.
const storeTimeout = 30 * time.Second

// extensions maps the sniffed content type to the object name suffix. Only
// these three types ever pass handlers.formFile, and the suffix is what the
// image endpoint uses to answer with the right Content-Type.
var extensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

// folders are the only prefixes the services use.
var folders = map[string]bool{"user": true, "tenant": true}

// objectStore is the slice of MinIO this adapter needs; tests replace it.
type objectStore interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	Remove(ctx context.Context, key string) error
}

type fileUploadAdapter struct {
	store   objectStore
	cipher  *imageCipher
	baseURL string
}

// New builds the adapter. With S3_ENDPOINT unset it returns one whose uploads
// fail, so the rest of the service still starts, as it did without the old image service URL.
func New(cfg *config.Config) (ports.FileUploadPort, error) {
	if cfg.S3Endpoint == "" {
		return unconfigured{}, nil
	}
	key, err := ParseKey(cfg.ImageEncryptionKey)
	if err != nil {
		return nil, err
	}
	c, err := newImageCipher(key)
	if err != nil {
		return nil, err
	}
	store, err := newMinioStore(cfg)
	if err != nil {
		return nil, err
	}
	return &fileUploadAdapter{store: store, cipher: c, baseURL: strings.TrimRight(cfg.ImageBaseURL, "/")}, nil
}

// NewFileUploadAdapter is New for main.go, where a bad configuration is fatal.
func NewFileUploadAdapter(cfg *config.Config) ports.FileUploadPort {
	adapter, err := New(cfg)
	if err != nil {
		logger.Fatal("Image storage is misconfigured", "error", err)
	}
	if _, off := adapter.(unconfigured); off {
		logger.Warn("S3_ENDPOINT is not set - file upload will be unavailable")
	}
	return adapter
}

func (f *fileUploadAdapter) Upload(ctx context.Context, file *domain.File, path string) (string, error) {
	ext, ok := extensions[file.ContentType]
	if !ok {
		return "", errors.New("unsupported image type")
	}
	if !folders[path] {
		return "", fmt.Errorf("unknown image folder %q", path)
	}
	logger.Info("Uploading file", "size", len(file.Data), "mimeType", file.ContentType)

	sealed, err := f.cipher.seal(file.Data)
	if err != nil {
		return "", err
	}
	// The client's filename is never used: a generated name cannot collide or
	// traverse, and cannot be guessed by someone who was not given the URL.
	name := uuid.NewString() + "." + ext
	if err := f.store.Put(ctx, path+"/"+name, sealed); err != nil {
		return "", err
	}
	return f.baseURL + "/" + path + "/" + name, nil
}

// Delete takes the last segment of filename (a stored URL or a bare name), as
// the image service adapter did. A value that was never stored here, such as a
// URL carried over from the old image service, removes nothing and is not an error.
func (f *fileUploadAdapter) Delete(path string, filename string) error {
	if filename == "" {
		return nil
	}
	if !folders[path] {
		return fmt.Errorf("unknown image folder %q", path)
	}
	parts := strings.Split(filename, "/")
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	return f.store.Remove(ctx, path+"/"+parts[len(parts)-1])
}

func (f *fileUploadAdapter) Download(ctx context.Context, path string, filename string) (*domain.File, error) {
	if !folders[path] || filename == "" || strings.Contains(filename, "/") {
		return nil, errors.New("image not found")
	}
	sealed, err := f.store.Get(ctx, path+"/"+filename)
	if err != nil {
		return nil, err
	}
	data, err := f.cipher.open(sealed)
	if err != nil {
		return nil, fmt.Errorf("open %s/%s: %w", path, filename, err)
	}
	ct := "application/octet-stream"
	for mimeType, ext := range extensions {
		if strings.HasSuffix(filename, "."+ext) {
			ct = mimeType
		}
	}
	return &domain.File{Name: filename, ContentType: ct, Data: data}, nil
}

// unconfigured is the adapter when S3_ENDPOINT is not set.
type unconfigured struct{}

var errNotConfigured = errors.New("image storage is not configured")

func (unconfigured) Upload(context.Context, *domain.File, string) (string, error) {
	return "", fmt.Errorf("upload: %w", errNotConfigured)
}
func (unconfigured) Delete(string, string) error { return fmt.Errorf("delete: %w", errNotConfigured) }
func (unconfigured) Download(context.Context, string, string) (*domain.File, error) {
	return nil, fmt.Errorf("download: %w", errNotConfigured)
}

// minioStore is the objectStore backed by a real MinIO server.
type minioStore struct {
	client *minio.Client
	bucket string
}

// newMinioStore reads the scheme of S3_ENDPOINT to decide on TLS, because the
// client wants a bare host:port, and checks that the bucket exists.
func newMinioStore(cfg *config.Config) (*minioStore, error) {
	endpoint := cfg.S3Endpoint
	secure := false
	if strings.Contains(endpoint, "://") {
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" {
			return nil, errors.New("S3_ENDPOINT is not a valid URL")
		}
		endpoint, secure = u.Host, u.Scheme == "https"
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	exists, err := client.BucketExists(ctx, cfg.S3Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q: %w", cfg.S3Bucket, err)
	}
	if !exists {
		// Private by default: MinIO buckets have no public policy unless one is set.
		if err := client.MakeBucket(ctx, cfg.S3Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket %q: %w", cfg.S3Bucket, err)
		}
		logger.Info("Created image bucket", "bucket", cfg.S3Bucket)
	}
	return &minioStore{client: client, bucket: cfg.S3Bucket}, nil
}

func (s *minioStore) Put(ctx context.Context, key string, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func (s *minioStore) Get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
			return nil, errors.New("image not found")
		}
		return nil, fmt.Errorf("read object: %w", err)
	}
	return data, nil
}

func (s *minioStore) Remove(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, storeTimeout)
	defer cancel()
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove object: %w", err)
	}
	return nil
}
