package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/tng-coop/auth-service/internal/adapters/driving/http/dto"
	"github.com/tng-coop/auth-service/internal/core/ports"
)

type AuthHandler struct {
	authService ports.AuthService
}

func NewAuthHandler(authService ports.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req dto.CreateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse("Invalid request body"))
	}

	file, _ := c.FormFile("image")

	user := req.ToUser()
	result, err := h.authService.Register(c.Context(), user, file)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse(result))
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse("Invalid request body"))
	}

	user, err := h.authService.ValidateUser(c.Context(), req.Username, req.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse(err.Error()))
	}
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Invalid username or password"))
	}

	result, err := h.authService.Login(c.Context(), user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}

func (h *AuthHandler) RefreshToken(c fiber.Ctx) error {
	var body struct {
		Token string `json:"token"`
	}
	if err := c.Bind().Body(&body); err != nil || body.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse("Missing refresh token"))
	}

	result, err := h.authService.RefreshTokens(body.Token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}

func (h *AuthHandler) ResetPassword(c fiber.Ctx) error {
	id := c.Params("id")
	var req dto.ResetPasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse("Invalid request body"))
	}

	caller := getCaller(c)
	result, err := h.authService.ResetPassword(c.Context(), id, req.Password, req.OldPassword, caller)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.FailResponse(err.Error()))
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}
