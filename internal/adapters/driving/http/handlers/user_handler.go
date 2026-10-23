package handlers

import (
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
)

// UserHandler handles user HTTP requests
type UserHandler struct {
	userService ports.UserService
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService ports.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// Create handles user creation
func (h *UserHandler) Create(c fiber.Ctx) error {
	var req dto.CreateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	user := req.ToUser()
	result, err := h.userService.Create(c.Context(), user, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse(dto.FromUser(result)))
}

// FindAll handles getting all users
// AccessReport lists the users in the caller's scope for access review.
func (h *UserHandler) AccessReport(c fiber.Ctx) error {
	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}
	users, err := h.userService.AccessReport(c.Context(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromUsers(users)))
}

func (h *UserHandler) FindAll(c fiber.Ctx) error {
	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	users, err := h.userService.FindAll(c.Context(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromUsers(users)))
}

// FindOne handles getting a single user
func (h *UserHandler) FindOne(c fiber.Ctx) error {
	id := c.Params("id")

	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}
	if !caller.IsAdminTier() && caller.ID != id {
		return c.Status(fiber.StatusForbidden).JSON(dto.FailResponse("You can only access your own account"))
	}

	user, err := h.userService.FindOne(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusNotFound, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromUser(user)))
}

// Update handles user update
func (h *UserHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	patch := req.ToPatch()
	if !caller.IsAdminTier() {
		if caller.ID != id {
			return c.Status(fiber.StatusForbidden).JSON(dto.FailResponse("You can only update your own account"))
		}
		// A non-admin must not be able to grant itself a role, reactivate a
		// disabled account, or move itself to another tenant or department.
		patch.Role, patch.Status, patch.TenantID, patch.DepartmentIDs = nil, nil, nil, nil
	}

	file, err := formFile(c, "image")
	if err != nil {
		return uploadError(c, err)
	}

	result, err := h.userService.Update(c.Context(), id, patch, file, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromUser(result)))
}

// Remove handles user deletion
func (h *UserHandler) Remove(c fiber.Ctx) error {
	id := c.Params("id")
	caller := getCaller(c)

	result, err := h.userService.Remove(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromUser(result)))
}

// Restore handles bringing a soft-deleted user back
func (h *UserHandler) Restore(c fiber.Ctx) error {
	id := c.Params("id")
	caller := getCaller(c)

	result, err := h.userService.Restore(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(dto.FromUser(result)))
}
