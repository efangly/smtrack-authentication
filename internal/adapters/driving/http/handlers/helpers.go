package handlers

import (
	"errors"

	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/pkg/logger"
	"github.com/efangly/smtrack-authentication/pkg/validator"
	"github.com/gofiber/fiber/v3"
)

// badRequest writes a 400 for a failed body bind. Validation failures carry a
// client-safe message; anything else (malformed JSON) stays generic.
func badRequest(c fiber.Ctx, err error) error {
	message := validator.Message(err)
	if message == "" {
		message = "Invalid request body"
	}
	return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(message))
}

// serviceError writes a failure from the core services. Domain errors are
// written by us and safe to return; anything wrapping an infrastructure error
// (GORM, Redis, the upload service) is logged and replaced with a generic
// message so SQL and driver internals do not reach the client.
func serviceError(c fiber.Ctx, status int, err error) error {
	if isInternal(err) {
		logger.Error("Request failed",
			"method", c.Method(),
			"path", c.Path(),
			"error", err.Error(),
		)
		return c.Status(fiber.StatusInternalServerError).
			JSON(dto.FailResponse("Internal server error"))
	}
	return c.Status(status).JSON(dto.FailResponse(err.Error()))
}

// isInternal reports whether err came from infrastructure rather than from a
// deliberate domain rule. Domain errors are built with errors.New/fmt.Errorf
// and wrap nothing; every adapter wraps the driver error it received, so a
// non-nil cause means the detail is not ours to expose.
func isInternal(err error) bool {
	return errors.Unwrap(err) != nil
}

// getCaller extracts the authenticated user's JWT payload from the Fiber context.
func getCaller(c fiber.Ctx) *domain.Actor {
	user := c.Locals("user")
	if user == nil {
		return nil
	}
	payload, ok := user.(*domain.Actor)
	if !ok {
		return nil
	}
	return payload
}
