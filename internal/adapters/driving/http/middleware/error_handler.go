package middleware

import (
	"errors"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/dto"
	"github.com/tng-coop/auth-service/pkg/logger"
)

// ErrorHandler is a custom error handler for Fiber
func ErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal server error"

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		message = fiberErr.Message
	}

	if code >= 500 {
		logger.Error("Server error",
			"method", c.Method(),
			"path", c.Path(),
			"status", code,
			"error", err.Error(),
		)
	}

	return c.Status(code).JSON(dto.ErrorResponse{
		Message: message,
		Success: false,
		Data:    nil,
	})
}

// Recover creates a panic recovery middleware
func Recover() fiber.Handler {
	return func(c fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic recovered",
					"method", c.Method(),
					"path", c.Path(),
					"panic", r,
					"stack", string(debug.Stack()),
				)
				_ = c.Status(fiber.StatusInternalServerError).JSON(dto.ErrorResponse{
					Message: "Internal server error",
					Success: false,
					Data:    nil,
				})
			}
		}()
		return c.Next()
	}
}
