package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubImages struct {
	file *domain.File
	err  error
}

func (s stubImages) Get(context.Context, string, string) (*domain.File, error) { return s.file, s.err }

const goodName = "0b1c2d3e-4f50-4a6b-8c7d-9e0f1a2b3c4d.png"

func imageApp(s stubImages) *fiber.App {
	app := fiber.New()
	app.Get("/auth/image/:path/:name", NewImageHandler(s).Get)
	return app
}

func getImage(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	require.NoError(t, err)
	return resp
}

func TestImage_ServesDecryptedBytes(t *testing.T) {
	app := imageApp(stubImages{file: &domain.File{ContentType: "image/png", Data: []byte("img")}})
	resp := getImage(t, app, "/auth/image/user/"+goodName)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
}

func TestImage_BadNamesNeverReachTheService(t *testing.T) {
	app := imageApp(stubImages{err: errors.New("should not be called")})
	for _, p := range []string{"/auth/image/user/x.png", "/auth/image/other/" + goodName, "/auth/image/user/" + goodName[:36] + ".gif"} {
		assert.Equal(t, 404, getImage(t, app, p).StatusCode, p)
	}
}

func TestImage_MissingIsNotFoundAndInfraIsGeneric(t *testing.T) {
	assert.Equal(t, 404, getImage(t, imageApp(stubImages{err: errors.New("image not found")}), "/auth/image/user/"+goodName).StatusCode)
	infra := stubImages{err: fmt.Errorf("get object: %w", errors.New("dial tcp"))}
	assert.Equal(t, 500, getImage(t, imageApp(infra), "/auth/image/user/"+goodName).StatusCode)
}
