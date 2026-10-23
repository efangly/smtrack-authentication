package dto

import (
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
)

// LoginRequest represents the login request body
type LoginRequest struct {
	Username string `json:"username" validate:"required,max=50"`
	Password string `json:"password" validate:"required,min=4,max=20"`
}

// RefreshRequest represents the refresh token request body
type RefreshRequest struct {
	Token string `json:"token" validate:"required"`
}

// ResetPasswordRequest represents the reset password request body
type ResetPasswordRequest struct {
	Password    string `json:"password" validate:"required,min=4,max=20"`
	OldPassword string `json:"oldPassword,omitempty" validate:"omitempty,min=4,max=20"`
}

// LoginResponse is the JSON body of a successful login.
type LoginResponse struct {
	Token         string      `json:"token"`
	RefreshToken  string      `json:"refreshToken"`
	ID            string      `json:"id"`
	Name          *string     `json:"name"`
	TenantID      string      `json:"tenantId"`
	DepartmentIDs []string    `json:"departmentIds"`
	Role          domain.Role `json:"role"`
	Pic           *string     `json:"pic"`
}

// FromLoginResult converts the core login result to its JSON shape.
func FromLoginResult(r *ports.LoginResult) LoginResponse {
	return LoginResponse{
		Token:         r.Token,
		RefreshToken:  r.RefreshToken,
		ID:            r.ID,
		Name:          r.Name,
		TenantID:      r.TenantID,
		DepartmentIDs: nonNil(r.DepartmentIDs),
		Role:          r.Role,
		Pic:           r.Pic,
	}
}

// RefreshResponse is the JSON body of a successful token refresh.
type RefreshResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

// FromRefreshResult converts the core refresh result to its JSON shape.
func FromRefreshResult(r *ports.RefreshResult) RefreshResponse {
	return RefreshResponse{Token: r.Token, RefreshToken: r.RefreshToken}
}
