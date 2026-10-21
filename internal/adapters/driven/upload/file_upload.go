package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
)

// uploadTimeout bounds every call to the image service. Without it a hung
// upstream would pin the request goroutine forever.
const uploadTimeout = 30 * time.Second

type fileUploadAdapter struct {
	uploadPath string
	client     *http.Client
}

func NewFileUploadAdapter(cfg *config.Config) ports.FileUploadPort {
	return &fileUploadAdapter{
		uploadPath: cfg.UploadPath,
		client:     &http.Client{Timeout: uploadTimeout},
	}
}

func (f *fileUploadAdapter) Upload(ctx context.Context, file *domain.File, path string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	logger.Info("Uploading file", "filename", file.Name, "size", len(file.Data), "mimeType", file.ContentType)

	// FormatMediaType quotes and escapes the client-supplied name, so a name
	// containing quotes or CR/LF cannot inject headers into the part sent
	// upstream. CreatePart (not CreateFormFile, which always says
	// application/octet-stream) carries the detected content type.
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": safeFilename(file.Name),
	}))
	h.Set("Content-Type", file.ContentType)
	part, err := writer.CreatePart(h)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(file.Data); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}
	writer.Close()

	url := fmt.Sprintf("%s/api/image/%s", f.uploadPath, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read upload response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Error("Upload server returned error", "status", resp.StatusCode, "body", string(respBody))
		return "", fmt.Errorf("upload server error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to decode upload response: %w", err)
	}

	if result.Path == "" {
		return "", fmt.Errorf("upload succeeded but no path returned, response: %s", string(respBody))
	}

	return fmt.Sprintf("%s%s", f.uploadPath, result.Path), nil
}

// safeFilename keeps only the base name of what the client sent, with control
// characters removed; an empty result becomes "upload".
func safeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." || name == "/" {
		return "upload"
	}
	return name
}

func (f *fileUploadAdapter) Delete(path string, filename string) error {
	if filename == "" {
		return nil
	}

	parts := strings.Split(filename, "/")
	fname := parts[len(parts)-1]

	url := fmt.Sprintf("%s/api/image/%s/%s", f.uploadPath, path, fname)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete server error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
