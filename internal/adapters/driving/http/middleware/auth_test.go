package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tng-coop/auth-service/internal/core/ports"
)

const (
	testJWTSecret     = "test-jwt-secret"
	testRefreshSecret = "test-refresh-secret"
)

func createTestToken(secret string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))
	return tokenStr
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"id":     "U1",
		"name":   "Test User",
		"role":   "ADMIN",
		"hosId":  "H1",
		"wardId": "W1",
		"exp":    float64(time.Now().Add(1 * time.Hour).Unix()),
		"iat":    float64(time.Now().Unix()),
	}
}

// --- JWTAuth ---

func TestJWTAuth_ValidToken(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuth(testJWTSecret))
	app.Get("/test", func(c fiber.Ctx) error {
		payload := c.Locals("user").(*ports.JwtPayload)
		return c.JSON(fiber.Map{
			"id":   payload.ID,
			"role": payload.Role,
		})
	})

	token := createTestToken(testJWTSecret, validClaims())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	json.Unmarshal(body, &result)
	assert.Equal(t, "U1", result["id"])
	assert.Equal(t, "ADMIN", result["role"])
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuth(testJWTSecret))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuth(testJWTSecret))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuth(testJWTSecret))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	claims := validClaims()
	claims["exp"] = float64(time.Now().Add(-1 * time.Hour).Unix()) // expired
	token := createTestToken(testJWTSecret, claims)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWTAuth_WrongSecret(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuth(testJWTSecret))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	token := createTestToken("wrong-secret", validClaims())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- RefreshJWTAuth ---

func TestRefreshJWTAuth_ValidToken(t *testing.T) {
	app := fiber.New()
	app.Use(RefreshJWTAuth(testRefreshSecret))
	app.Post("/refresh", func(c fiber.Ctx) error {
		payload := c.Locals("user").(*ports.JwtPayload)
		return c.JSON(fiber.Map{"id": payload.ID})
	})

	token := createTestToken(testRefreshSecret, validClaims())
	body := `{"token":"` + token + `"}`

	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRefreshJWTAuth_MissingToken(t *testing.T) {
	app := fiber.New()
	app.Use(RefreshJWTAuth(testRefreshSecret))
	app.Post("/refresh", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRefreshJWTAuth_InvalidToken(t *testing.T) {
	app := fiber.New()
	app.Use(RefreshJWTAuth(testRefreshSecret))
	app.Post("/refresh", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	body := `{"token":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- getString helper ---

func TestGetString(t *testing.T) {
	claims := jwt.MapClaims{
		"id":   "test-id",
		"num":  float64(123),
		"null": nil,
	}
	assert.Equal(t, "test-id", getString(claims, "id"))
	assert.Equal(t, "", getString(claims, "num"))    // not a string
	assert.Equal(t, "", getString(claims, "null"))   // nil
	assert.Equal(t, "", getString(claims, "absent")) // missing key
}
