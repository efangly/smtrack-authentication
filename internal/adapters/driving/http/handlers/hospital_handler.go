package handlers

import (
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
)

// HospitalHandler handles hospital HTTP requests
type HospitalHandler struct {
	hospitalService ports.HospitalService
}

// NewHospitalHandler creates a new hospital handler
func NewHospitalHandler(hospitalService ports.HospitalService) *HospitalHandler {
	return &HospitalHandler{hospitalService: hospitalService}
}

// Create handles hospital creation
func (h *HospitalHandler) Create(c fiber.Ctx) error {
	var req dto.CreateHospitalRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	file, err := formFile(c, "image")
	if err != nil {
		return uploadError(c, err)
	}
	caller := getCaller(c)

	hospital := req.ToHospital()
	result, err := h.hospitalService.Create(c.Context(), hospital, file, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse(result))
}

// FindAll handles getting all hospitals
func (h *HospitalHandler) FindAll(c fiber.Ctx) error {
	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	hospitals, err := h.hospitalService.FindAll(c.Context(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(hospitals))
}

// FindOne handles getting a single hospital
func (h *HospitalHandler) FindOne(c fiber.Ctx) error {
	id := c.Params("id")

	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	hospital, err := h.hospitalService.FindOne(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusNotFound, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(hospital))
}

// Update handles hospital update
func (h *HospitalHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateHospitalRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	file, err := formFile(c, "image")
	if err != nil {
		return uploadError(c, err)
	}
	caller := getCaller(c)

	result, err := h.hospitalService.Update(c.Context(), id, req.ToPatch(), file, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}

// Remove handles hospital deletion
func (h *HospitalHandler) Remove(c fiber.Ctx) error {
	id := c.Params("id")
	caller := getCaller(c)

	result, err := h.hospitalService.Remove(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}
