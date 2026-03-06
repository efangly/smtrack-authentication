package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tng-coop/auth-service/internal/core/ports"
)

// getCaller extracts the authenticated user's JWT payload from the Fiber context.
func getCaller(c fiber.Ctx) *ports.JwtPayload {
	user := c.Locals("user")
	if user == nil {
		return nil
	}
	payload, ok := user.(*ports.JwtPayload)
	if !ok {
		return nil
	}
	return payload
}
