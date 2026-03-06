package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/dto"
)

func TestErrorHandler_FiberError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
	})
	app.Get("/err", func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "resource not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var errResp dto.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "resource not found", errResp.Message)
	assert.False(t, errResp.Success)
}

func TestErrorHandler_GenericError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
	})
	app.Get("/err", func(c fiber.Ctx) error {
		return errors.New("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/err", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var errResp dto.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "Internal server error", errResp.Message)
	assert.False(t, errResp.Success)
}

func TestRecover_Panic(t *testing.T) {
	app := fiber.New()
	app.Use(Recover())
	app.Get("/panic", func(c fiber.Ctx) error {
		panic("unexpected panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var errResp dto.ErrorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "Internal server error", errResp.Message)
	assert.False(t, errResp.Success)
}

func TestRecover_NoPanic(t *testing.T) {
	app := fiber.New()
	app.Use(Recover())
	app.Get("/ok", func(c fiber.Ctx) error {
		return c.SendString("healthy")
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "healthy", string(body))
}
