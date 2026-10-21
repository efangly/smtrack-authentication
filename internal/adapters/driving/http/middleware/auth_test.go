package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/adapters/driven/security"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testJWTSecret     = "test-jwt-secret"
	testRefreshSecret = "test-refresh-secret"
)

// testIssuer is the real JWT adapter, so these tests cover token -> actor
// end to end.
func testIssuer() ports.TokenIssuer {
	return security.NewJWTIssuer(&config.Config{JWTSecret: testJWTSecret, JWTRefreshSecret: testRefreshSecret})
}

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
	app.Use(JWTAuth(testIssuer()))
	app.Get("/test", func(c fiber.Ctx) error {
		payload := c.Locals("user").(*domain.Actor)
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
	app.Use(JWTAuth(testIssuer()))
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
	app.Use(JWTAuth(testIssuer()))
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
	app.Use(JWTAuth(testIssuer()))
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
	app.Use(JWTAuth(testIssuer()))
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
	app.Use(RefreshJWTAuth(testIssuer()))
	app.Post("/refresh", func(c fiber.Ctx) error {
		return c.SendString("ok")
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
	app.Use(RefreshJWTAuth(testIssuer()))
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
	app.Use(RefreshJWTAuth(testIssuer()))
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

func TestRefreshJWTAuth_RejectsAccessToken(t *testing.T) {
	app := fiber.New()
	app.Use(RefreshJWTAuth(testIssuer()))
	app.Post("/refresh", func(c fiber.Ctx) error { return c.SendString("ok") })

	pair, err := testIssuer().IssuePair(domain.Actor{ID: "U1", Role: domain.RoleAdmin})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{"token":"`+pair.Access+`"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWTAuth_MessagesUnchanged(t *testing.T) {
	app := fiber.New()
	app.Use(JWTAuth(testIssuer()))
	app.Get("/test", func(c fiber.Ctx) error { return c.SendString("ok") })

	for header, want := range map[string]string{
		"":             `{"message":"Missing authorization header","statusCode":401}`,
		"Token abc":    `{"message":"Invalid authorization header format","statusCode":401}`,
		"Bearer bogus": `{"message":"Invalid or expired token","statusCode":401}`,
	} {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		assert.JSONEq(t, want, string(body), header)
	}
}
