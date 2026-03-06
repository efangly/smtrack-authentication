package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/dto"
	"github.com/tng-coop/auth-service/internal/core/ports"
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
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse("Invalid request body"))
	}

	user := req.ToUser()
	result, err := h.userService.Create(c.Context(), user)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
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
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(users))
}

// FindOne handles getting a single user
func (h *UserHandler) FindOne(c fiber.Ctx) error {
	id := c.Params("id")

	user, err := h.userService.FindOne(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(user))
}

// Update handles user update
func (h *UserHandler) Update(c fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse("Invalid request body"))
	}

	file, _ := c.FormFile("image")

	result, err := h.userService.Update(c.Context(), id, req.ToMap(), file)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}

// Remove handles user deletion
func (h *UserHandler) Remove(c fiber.Ctx) error {
	id := c.Params("id")

	result, err := h.userService.Remove(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}
