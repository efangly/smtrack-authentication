package middleware

import (
	"encoding/json"
	"strings"

	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
)

func unauthorized(c fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"statusCode": 401,
		"message":    message,
	})
}

// JWTAuth verifies the bearer access token and puts the *domain.Actor it
// names in c.Locals("user").
func JWTAuth(tokens ports.TokenIssuer) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return unauthorized(c, "Missing authorization header")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return unauthorized(c, "Invalid authorization header format")
		}

		actor, err := tokens.VerifyAccess(parts[1])
		if err != nil {
			return unauthorized(c, "Invalid or expired token")
		}

		c.Locals("user", actor)
		return c.Next()
	}
}

// RefreshJWTAuth rejects a request whose body does not carry a valid refresh
// token, before it reaches the handler.
func RefreshJWTAuth(tokens ports.TokenIssuer) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(c.Body(), &body); err != nil || body.Token == "" {
			return unauthorized(c, "Missing refresh token")
		}

		if _, err := tokens.VerifyRefresh(body.Token); err != nil {
			return unauthorized(c, "Invalid or expired refresh token")
		}
		return c.Next()
	}
}
