package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/dto"
	"github.com/tng-coop/auth-service/internal/core/ports"
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
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse("Invalid request body"))
	}

	file, _ := c.FormFile("image")

	hospital := req.ToHospital()
	result, err := h.hospitalService.Create(c.Context(), hospital, file)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
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
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(hospitals))
}

// FindOne handles getting a single hospital
func (h *HospitalHandler) FindOne(c fiber.Ctx) error {
	id := c.Params("id")

	hospital, err := h.hospitalService.FindOne(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(hospital))
}

// Update handles hospital update
func (h *HospitalHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateHospitalRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse("Invalid request body"))
	}

	file, _ := c.FormFile("image")

	result, err := h.hospitalService.Update(c.Context(), id, req.ToMap(), file)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}

// Remove handles hospital deletion
func (h *HospitalHandler) Remove(c fiber.Ctx) error {
	id := c.Params("id")

	result, err := h.hospitalService.Remove(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}
