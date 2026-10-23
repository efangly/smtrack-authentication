package handlers

import (
	"regexp"

	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
)

// imageName is what the upload adapter generates: a UUID and a known extension.
var imageName = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp)$`)

// ImageHandler serves stored pictures. They are encrypted at rest, so MinIO
// cannot hand them out directly.
type ImageHandler struct {
	service ports.ImageService
}

func NewImageHandler(service ports.ImageService) *ImageHandler {
	return &ImageHandler{service: service}
}

// Get answers without a token, as the old image URLs did; the unguessable
// generated name is what keeps a picture private to whoever was given its URL.
func (h *ImageHandler) Get(c fiber.Ctx) error {
	path, name := c.Params("path"), c.Params("name")
	if (path != "user" && path != "tenant") || !imageName.MatchString(name) {
		return c.Status(fiber.StatusNotFound).JSON(dto.FailResponse("image not found"))
	}
	file, err := h.service.Get(c.Context(), path, name)
	if err != nil {
		return serviceError(c, fiber.StatusNotFound, err)
	}
	c.Set(fiber.HeaderContentType, file.ContentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set(fiber.HeaderCacheControl, "private, max-age=3600")
	return c.Send(file.Data)
}
