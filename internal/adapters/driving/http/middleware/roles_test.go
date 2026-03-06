package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
)

func setupRolesApp(allowedRoles ...domain.Role) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		// Simulate JWT auth middleware having run
		tokenStr := c.Get("X-Test-Token")
		if tokenStr != "" {
			token, _ := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				return []byte(testJWTSecret), nil
			})
			if token != nil && token.Valid {
				claims := token.Claims.(jwt.MapClaims)
				c.Locals("user", &ports.JwtPayload{
					ID:   getString(claims, "id"),
					Role: getString(claims, "role"),
				})
			}
		}
		return c.Next()
	})
	app.Get("/protected", RolesGuard(allowedRoles...), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func TestRolesGuard_AllowedRole(t *testing.T) {
	app := setupRolesApp(domain.RoleAdmin, domain.RoleSuper)

	claims := jwt.MapClaims{
		"id":   "U1",
		"role": "ADMIN",
		"exp":  float64(time.Now().Add(1 * time.Hour).Unix()),
	}
	token := createTestToken(testJWTSecret, claims)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Test-Token", token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRolesGuard_ForbiddenRole(t *testing.T) {
	app := setupRolesApp(domain.RoleSuper) // Only SUPER allowed

	claims := jwt.MapClaims{
		"id":   "U1",
		"role": "USER",
		"exp":  float64(time.Now().Add(1 * time.Hour).Unix()),
	}
	token := createTestToken(testJWTSecret, claims)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Test-Token", token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestRolesGuard_NoUser(t *testing.T) {
	app := setupRolesApp(domain.RoleAdmin)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	// No X-Test-Token header → no user in Locals
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRolesGuard_NoRolesSpecified(t *testing.T) {
	// When no roles are specified, all authenticated users pass
	app := setupRolesApp() // empty roles

	claims := jwt.MapClaims{
		"id":   "U1",
		"role": "GUEST",
		"exp":  float64(time.Now().Add(1 * time.Hour).Unix()),
	}
	token := createTestToken(testJWTSecret, claims)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Test-Token", token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
