package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/tng-coop/auth-service/config"
	"github.com/tng-coop/auth-service/internal/core/ports"
	"github.com/tng-coop/auth-service/pkg/logger"
)

type fileUploadAdapter struct {
	uploadPath string
}

func NewFileUploadAdapter(cfg *config.Config) ports.FileUploadPort {
	return &fileUploadAdapter{uploadPath: cfg.UploadPath}
}

func (f *fileUploadAdapter) Upload(fileHeader *multipart.FileHeader, path string) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Read file content to detect actual MIME type
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	mimeType := http.DetectContentType(fileBytes)
	logger.Info("Uploading file", "filename", fileHeader.Filename, "size", len(fileBytes), "mimeType", mimeType)

	// Use CreatePart with correct Content-Type instead of CreateFormFile
	// (CreateFormFile always sets application/octet-stream)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileHeader.Filename))
	h.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(h)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, bytes.NewReader(fileBytes)); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}
	writer.Close()

	url := fmt.Sprintf("%s/api/image/%s", f.uploadPath, path)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
