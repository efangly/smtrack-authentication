package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"

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

// maxImageBytes caps an uploaded image. It equals Fiber's default request body
// limit, so today the body limit rejects anything larger first; this keeps
// the cap if that limit is ever raised.
const maxImageBytes = fiber.DefaultBodyLimit

// allowedImageTypes are the content types an uploaded image may have, as
// detected from its bytes.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// errInvalidImage marks a rejected upload; its message is safe to return.
type errInvalidImage struct{ msg string }

func (e errInvalidImage) Error() string { return e.msg }

// formFile reads the optional image in field and checks it. It returns nil,
// nil when the request carries no such file (including non-multipart
// requests), and an errInvalidImage when the file is too large or not an
// accepted image type. The content type is sniffed from the bytes, never taken
// from the client.
func formFile(c fiber.Ctx, field string) (*domain.File, error) {
	header, err := c.FormFile(field)
	if err != nil {
		return nil, nil
	}
	if header.Size > maxImageBytes {
		return nil, errInvalidImage{fmt.Sprintf("image must be %dMB or smaller", maxImageBytes>>20)}
	}

	f, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open upload: %w", err)
	}
	defer f.Close()
	// Read one byte past the cap so a lying Size cannot sneak past it.
	data, err := io.ReadAll(io.LimitReader(f, maxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read upload: %w", err)
	}
	if len(data) > maxImageBytes {
		return nil, errInvalidImage{fmt.Sprintf("image must be %dMB or smaller", maxImageBytes>>20)}
	}

	contentType := http.DetectContentType(data)
	if !allowedImageTypes[contentType] {
		return nil, errInvalidImage{"image must be a JPEG, PNG or WebP file"}
	}
	return &domain.File{Name: header.Filename, ContentType: contentType, Data: data}, nil
}

// uploadError writes the response for a formFile failure.
func uploadError(c fiber.Ctx, err error) error {
	var invalid errInvalidImage
	if errors.As(err, &invalid) {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(invalid.msg))
	}
	return serviceError(c, fiber.StatusBadRequest, err)
}
