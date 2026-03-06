package ports

import (
	"context"
	"mime/multipart"

	"github.com/tng-coop/auth-service/internal/core/domain"
)

// JwtPayload represents the JWT token payload
type JwtPayload struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	HosID  string `json:"hosId"`
	WardID string `json:"wardId"`
}

// LoginResult represents the result of a login operation
type LoginResult struct {
	Token        string  `json:"token"`
	RefreshToken string  `json:"refreshToken"`
	ID           string  `json:"id"`
	Name         *string `json:"name"`
	HosID        string  `json:"hosId"`
	WardID       string  `json:"wardId"`
	Role         string  `json:"role"`
	Pic          *string `json:"pic"`
}

// RefreshResult represents the result of a token refresh
type RefreshResult struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

// AuthService defines the driving port for authentication use cases
type AuthService interface {
	Register(ctx context.Context, data *domain.User, file *multipart.FileHeader) (*domain.User, error)
	ValidateUser(ctx context.Context, username string, password string) (*domain.User, error)
	Login(ctx context.Context, user *domain.User) (*LoginResult, error)
	RefreshTokens(token string) (*RefreshResult, error)
	ResetPassword(ctx context.Context, username string, password string, oldPassword string, caller *JwtPayload) (string, error)
}

// UserService defines the driving port for user use cases
type UserService interface {
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	FindAll(ctx context.Context, caller *JwtPayload) ([]domain.User, error)
	FindOne(ctx context.Context, id string) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader) (*domain.User, error)
	Remove(ctx context.Context, id string) (*domain.User, error)
}

// HospitalService defines the driving port for hospital use cases
type HospitalService interface {
	Create(ctx context.Context, hospital *domain.Hospital, file *multipart.FileHeader) (*domain.Hospital, error)
	FindAll(ctx context.Context, caller *JwtPayload) ([]domain.Hospital, error)
	FindOne(ctx context.Context, id string) (*domain.Hospital, error)
	Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader) (*domain.Hospital, error)
	Remove(ctx context.Context, id string) (*domain.Hospital, error)
}

// WardService defines the driving port for ward use cases
type WardService interface {
	Create(ctx context.Context, ward *domain.Ward) (*domain.Ward, error)
	FindAll(ctx context.Context, caller *JwtPayload) ([]domain.Ward, error)
	FindOne(ctx context.Context, id string) (*domain.Ward, error)
	Update(ctx context.Context, id string, data map[string]any) (*domain.Ward, error)
	Remove(ctx context.Context, id string) (string, error)
}
