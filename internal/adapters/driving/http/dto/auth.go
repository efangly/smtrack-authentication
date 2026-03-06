package dto

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
