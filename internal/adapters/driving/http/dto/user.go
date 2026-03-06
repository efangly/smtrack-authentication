package dto

import "github.com/tng-coop/auth-service/internal/core/domain"

// CreateUserRequest represents the create user request body
type CreateUserRequest struct {
	ID       string      `json:"id,omitempty"`
	WardID   string      `json:"wardId" validate:"required,max=100"`
	Username string      `json:"username" validate:"required,max=40"`
	Password string      `json:"password" validate:"required,min=4,max=20"`
	Status   *bool       `json:"status,omitempty"`
	Role     domain.Role `json:"role,omitempty"`
	Display  *string     `json:"display,omitempty" validate:"omitempty,min=1,max=50"`
	Pic      *string     `json:"pic,omitempty" validate:"omitempty,max=255"`
	Comment  *string     `json:"comment,omitempty" validate:"omitempty,max=255"`
	CreateBy *string     `json:"createBy,omitempty" validate:"omitempty,max=50"`
}

// ToUser converts the DTO to a domain User
func (r *CreateUserRequest) ToUser() *domain.User {
	user := &domain.User{
		ID:       r.ID,
		WardID:   r.WardID,
		Username: r.Username,
		Password: r.Password,
		Role:     r.Role,
		Display:  r.Display,
		Pic:      r.Pic,
		Comment:  r.Comment,
		CreateBy: r.CreateBy,
	}
	if r.Status != nil {
		user.Status = *r.Status
	} else {
		user.Status = true
	}
	if r.Role == "" {
		user.Role = domain.RoleGuest
	}
	return user
}

// UpdateUserRequest represents the update user request body
type UpdateUserRequest struct {
	WardID   *string      `json:"wardId,omitempty"`
	Username *string      `json:"username,omitempty"`
	Password *string      `json:"password,omitempty"`
	Status   *bool        `json:"status,omitempty"`
	Role     *domain.Role `json:"role,omitempty"`
	Display  *string      `json:"display,omitempty"`
	Pic      *string      `json:"pic,omitempty"`
	Comment  *string      `json:"comment,omitempty"`
}

// ToMap converts the update DTO to a map for partial update
func (r *UpdateUserRequest) ToMap() map[string]any {
	data := make(map[string]any)
	if r.WardID != nil {
		data["wardId"] = *r.WardID
	}
	if r.Username != nil {
		data["username"] = *r.Username
	}
	if r.Password != nil {
		data["password"] = *r.Password
	}
	if r.Status != nil {
		data["status"] = *r.Status
	}
	if r.Role != nil {
		data["role"] = string(*r.Role)
	}
	if r.Display != nil {
		data["display"] = *r.Display
	}
	if r.Pic != nil {
		data["pic"] = *r.Pic
	}
	if r.Comment != nil {
		data["comment"] = *r.Comment
	}
	return data
}
