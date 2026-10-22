package handlers

import (
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
)

// DepartmentHandler handles department HTTP requests
type DepartmentHandler struct {
	departmentService ports.DepartmentService
}

// NewDepartmentHandler creates a new department handler
func NewDepartmentHandler(departmentService ports.DepartmentService) *DepartmentHandler {
	return &DepartmentHandler{departmentService: departmentService}
}

// Create handles department creation
func (h *DepartmentHandler) Create(c fiber.Ctx) error {
	var req dto.CreateDepartmentRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	caller := getCaller(c)

	result, err := h.departmentService.Create(c.Context(), req.ToDepartment(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse(dto.FromDepartment(result)))
}

// FindAll handles getting all departments
func (h *DepartmentHandler) FindAll(c fiber.Ctx) error {
	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	departments, err := h.departmentService.FindAll(c.Context(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromDepartments(departments)))
}

// FindOne handles getting a single department
func (h *DepartmentHandler) FindOne(c fiber.Ctx) error {
	id := c.Params("id")

	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	department, err := h.departmentService.FindOne(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusNotFound, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromDepartment(department)))
}

// Update handles department update
func (h *DepartmentHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateDepartmentRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	caller := getCaller(c)
	result, err := h.departmentService.Update(c.Context(), id, req.ToPatch(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromDepartment(result)))
}

// Remove handles department deletion
func (h *DepartmentHandler) Remove(c fiber.Ctx) error {
	id := c.Params("id")
	caller := getCaller(c)

	result, err := h.departmentService.Remove(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromDepartment(result)))
}

// Restore handles bringing a soft-deleted department back
func (h *DepartmentHandler) Restore(c fiber.Ctx) error {
	id := c.Params("id")
	caller := getCaller(c)

	result, err := h.departmentService.Restore(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromDepartment(result)))
}
