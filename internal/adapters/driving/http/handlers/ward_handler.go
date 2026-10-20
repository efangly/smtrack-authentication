package handlers

import (
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
)

// WardHandler handles ward HTTP requests
type WardHandler struct {
	wardService ports.WardService
}

// NewWardHandler creates a new ward handler
func NewWardHandler(wardService ports.WardService) *WardHandler {
	return &WardHandler{wardService: wardService}
}

// Create handles ward creation
func (h *WardHandler) Create(c fiber.Ctx) error {
	var req dto.CreateWardRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	caller := getCaller(c)

	ward := req.ToWard()
	result, err := h.wardService.Create(c.Context(), ward, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse(result))
}

// FindAll handles getting all wards
func (h *WardHandler) FindAll(c fiber.Ctx) error {
	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	wards, err := h.wardService.FindAll(c.Context(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(wards))
}

// FindOne handles getting a single ward
func (h *WardHandler) FindOne(c fiber.Ctx) error {
	id := c.Params("id")

	ward, err := h.wardService.FindOne(c.Context(), id)
	if err != nil {
		return serviceError(c, fiber.StatusNotFound, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(ward))
}

// Update handles ward update
func (h *WardHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateWardRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	caller := getCaller(c)
	result, err := h.wardService.Update(c.Context(), id, req.ToMap(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}

// Remove handles ward deletion
func (h *WardHandler) Remove(c fiber.Ctx) error {
	id := c.Params("id")
	caller := getCaller(c)

	result, err := h.wardService.Remove(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}
