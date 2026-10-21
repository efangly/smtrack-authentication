package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	jpegBytes = append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{0}, 64)...)
	webpBytes = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 64)...)
)

// uploadApp echoes what formFile produced. Its body limit is above
// maxImageBytes so the image cap itself is exercised, not Fiber's.
func uploadApp() *fiber.App {
	app := fiber.New(fiber.Config{BodyLimit: 2 * maxImageBytes})
	app.Post("/up", func(c fiber.Ctx) error {
		file, err := formFile(c, "image")
		if err != nil {
			return uploadError(c, err)
		}
		if file == nil {
			return c.JSON(fiber.Map{"file": nil})
		}
		return c.JSON(fiber.Map{"name": file.Name, "type": file.ContentType, "size": len(file.Data)})
	})
	return app
}

func multipartRequest(t *testing.T, field, filename, claimedType string, data []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	require.NoError(t, w.WriteField("username", "jdoe"))
	if field != "" {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{`form-data; name="` + field + `"; filename="` + filename + `"`}
		h["Content-Type"] = []string{claimedType}
		part, err := w.CreatePart(h)
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	req := httptest.NewRequest(http.MethodPost, "/up", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func decode(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m), string(raw))
	return m
}

func TestFormFile_AcceptsImages(t *testing.T) {
	for name, data := range map[string][]byte{"png": pngBytes, "jpeg": jpegBytes, "webp": webpBytes} {
		t.Run(name, func(t *testing.T) {
			// The claimed type is ignored; the bytes decide.
			resp, err := uploadApp().Test(multipartRequest(t, "image", "pic."+name, "application/octet-stream", data))
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body := decode(t, resp)
			assert.Equal(t, "image/"+name, body["type"])
			assert.Equal(t, "pic."+name, body["name"])
			assert.EqualValues(t, len(data), body["size"])
		})
	}
}

func TestFormFile_RejectsNonImageEvenIfClaimed(t *testing.T) {
	resp, err := uploadApp().Test(multipartRequest(t, "image", "evil.png", "image/png", []byte("<html><script>alert(1)</script>")))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "image must be a JPEG, PNG or WebP file", decode(t, resp)["message"])
}

func TestFormFile_RejectsOversized(t *testing.T) {
	big := append(append([]byte{}, pngBytes...), make([]byte, maxImageBytes)...)
	resp, err := uploadApp().Test(multipartRequest(t, "image", "big.png", "image/png", big))
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "image must be 4MB or smaller", decode(t, resp)["message"])
}

func TestFormFile_NoFileIsNotAnError(t *testing.T) {
	// Multipart without the field, and a plain JSON request.
	resp, err := uploadApp().Test(multipartRequest(t, "", "", "", nil))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Nil(t, decode(t, resp)["file"])

	req := httptest.NewRequest(http.MethodPost, "/up", bytes.NewReader([]byte(`{"username":"jdoe"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err = uploadApp().Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
