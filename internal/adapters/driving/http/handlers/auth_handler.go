package handlers

import (
	"github.com/efangly/smtrack-authentication/internal/adapters/driving/http/dto"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/gofiber/fiber/v3"
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
		return badRequest(c, err)
	}

	file, _ := c.FormFile("image")

	// Public endpoint: role/status/id are forced, never taken from the body.
	user := req.ToPublicUser()
	result, err := h.authService.Register(c.Context(), user, file)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto.SuccessResponse(result))
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	user, err := h.authService.ValidateUser(c.Context(), req.Username, req.Password)
	if err != nil {
		return serviceError(c, fiber.StatusUnauthorized, err)
	}
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Invalid username or password"))
	}

	result, err := h.authService.Login(c.Context(), user)
	if err != nil {
		return serviceError(c, fiber.StatusInternalServerError, err)
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

	result, err := h.authService.RefreshTokens(c.Context(), body.Token)
	if err != nil {
		return serviceError(c, fiber.StatusUnauthorized, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}

func (h *AuthHandler) ResetPassword(c fiber.Ctx) error {
	username := c.Params("username")
	var req dto.ResetPasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return badRequest(c, err)
	}

	caller := getCaller(c)
	if caller == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(dto.FailResponse("Unauthorized"))
	}

	result, err := h.authService.ResetPassword(c.Context(), username, req.Password, req.OldPassword, caller)
	if err != nil {
		return serviceError(c, fiber.StatusBadRequest, err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.SuccessResponse(result))
}
