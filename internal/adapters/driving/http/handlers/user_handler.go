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

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse(result))
}

// FindAll handles getting all users
func (h *UserHandler) FindAll(c fiber.Ctx) error {
	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	users, err := h.userService.FindAll(c.Context(), caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(users))
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

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(user))
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

	data := req.ToMap()
	if !caller.IsAdminTier() {
		if caller.ID != id {
			return c.Status(fiber.StatusForbidden).JSON(dto.FailResponse("You can only update your own account"))
		}
		// A non-admin must not be able to grant itself a role, reactivate a
		// disabled account, or move itself into another ward.
		delete(data, "role")
		delete(data, "status")
		delete(data, "wardId")
	}

	file, _ := c.FormFile("image")

	result, err := h.userService.Update(c.Context(), id, data, file, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}

// Remove handles user deletion
func (h *UserHandler) Remove(c fiber.Ctx) error {
	id := c.Params("id")
	caller := getCaller(c)

	result, err := h.userService.Remove(c.Context(), id, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}
