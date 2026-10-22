package handlers

import (
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
)

// TenantHandler handles tenant HTTP requests
type TenantHandler struct {
	tenantService ports.TenantService
}

// NewTenantHandler creates a new tenant handler
func NewTenantHandler(tenantService ports.TenantService) *TenantHandler {
	return &TenantHandler{tenantService: tenantService}
}

// Create handles tenant creation
func (h *TenantHandler) Create(c fiber.Ctx) error {
	var req dto.CreateTenantRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	file, err := formFile(c, "image")
	if err != nil {
		return uploadError(c, err)
	}
	caller := getCaller(c)

	result, err := h.tenantService.Create(c.Context(), req.ToTenant(), file, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse(dto.FromTenant(result)))
}

// FindAll handles getting all tenants
func (h *TenantHandler) FindAll(c fiber.Ctx) error {
	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	tenants, err := h.tenantService.FindAll(c.Context(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromTenants(tenants)))
}

// FindOne handles getting a single tenant
func (h *TenantHandler) FindOne(c fiber.Ctx) error {
	id := c.Params("id")

	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	tenant, err := h.tenantService.FindOne(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusNotFound, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromTenant(tenant)))
}

// Update handles tenant update
func (h *TenantHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateTenantRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	file, err := formFile(c, "image")
	if err != nil {
		return uploadError(c, err)
	}
	caller := getCaller(c)

	result, err := h.tenantService.Update(c.Context(), id, req.ToPatch(), file, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromTenant(result)))
}

// UpdateLabels handles replacing a tenant's label profile
func (h *TenantHandler) UpdateLabels(c fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateLabelsRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}
	caller := getCaller(c)

	result, err := h.tenantService.UpdateLabels(c.Context(), id, req.Labels, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromTenant(result)))
}

// Remove handles tenant deletion
func (h *TenantHandler) Remove(c fiber.Ctx) error {
	id := c.Params("id")
	caller := getCaller(c)

	result, err := h.tenantService.Remove(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromTenant(result)))
}

// Restore handles bringing a soft-deleted tenant back
func (h *TenantHandler) Restore(c fiber.Ctx) error {
	id := c.Params("id")
	caller := getCaller(c)

	result, err := h.tenantService.Restore(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromTenant(result)))
}
