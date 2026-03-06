package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/dto"
	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
)

// RolesGuard creates a role-based authorization middleware
func RolesGuard(allowedRoles ...domain.Role) fiber.Handler {
	return func(c fiber.Ctx) error {
		user := c.Locals("user")
		if user == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
		}

		payload, ok := user.(*ports.JwtPayload)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Invalid user context"))
		}

		// If no roles specified, allow all authenticated users
		if len(allowedRoles) == 0 {
			return c.Next()
		}

		for _, role := range allowedRoles {
			if string(role) == payload.Role {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(dto.FailResponse("You do not have the required role"))
	}
}
