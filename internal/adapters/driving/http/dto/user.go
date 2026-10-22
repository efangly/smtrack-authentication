package dto

import "github.com/efangly/smtrack-authentication/internal/core/domain"

// CreateUserRequest represents the create user request body
type CreateUserRequest struct {
	ID            string      `json:"id,omitempty"`
	TenantID      string      `json:"tenantId" validate:"required,max=100"`
	DepartmentIDs []string    `json:"departmentIds,omitempty" validate:"omitempty,max=100,dive,max=100"`
	Username      string      `json:"username" validate:"required,max=40"`
	Password      string      `json:"password" validate:"required,min=4,max=20"`
	Status        *bool       `json:"status,omitempty"`
	Role          domain.Role `json:"role,omitempty"`
	Display       *string     `json:"display,omitempty" validate:"omitempty,min=1,max=50"`
	Comment       *string     `json:"comment,omitempty" validate:"omitempty,max=255"`
	CreateBy      *string     `json:"createBy,omitempty" validate:"omitempty,max=50"`
}

// ToUser converts the DTO to a domain User
func (r *CreateUserRequest) ToUser() *domain.User {
	user := &domain.User{
		ID:            r.ID,
		TenantID:      r.TenantID,
		DepartmentIDs: r.DepartmentIDs,
		Username:      r.Username,
		Password:      r.Password,
		Role:          r.Role,
		Display:       r.Display,
		Comment:       r.Comment,
		CreateBy:      r.CreateBy,
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

// ToPublicUser converts the DTO to a domain User for the public /auth/register
// endpoint. Privileged fields (id, role, status, createBy) are never taken from
// the request body — an unauthenticated caller must not be able to choose them,
// and departments are assigned later by an admin of the tenant.
func (r *CreateUserRequest) ToPublicUser() *domain.User {
	return &domain.User{
		TenantID: r.TenantID,
		Username: r.Username,
		Password: r.Password,
		Role:     domain.RoleGuest,
		Status:   true,
		Display:  r.Display,
		Comment:  r.Comment,
	}
}

// The picture is not settable from a request body: it is only ever the URL
// the upload service returned for an uploaded "image" file. Accepting a URL
// here let a user point their record at someone else's file, which the next
// upload then deleted.

// UpdateUserRequest represents the update user request body.
// Password is intentionally absent: it would be written to the database
// unhashed. Use PATCH /auth/reset/:username instead.
type UpdateUserRequest struct {
	TenantID      *string      `json:"tenantId,omitempty"`
	DepartmentIDs *[]string    `json:"departmentIds,omitempty" validate:"omitempty,max=100,dive,max=100"`
	Username      *string      `json:"username,omitempty"`
	Status        *bool        `json:"status,omitempty"`
	Role          *domain.Role `json:"role,omitempty"`
	Display       *string      `json:"display,omitempty"`
	Comment       *string      `json:"comment,omitempty"`
}

// ToPatch converts the update DTO to a partial update; unset fields stay nil.
func (r *UpdateUserRequest) ToPatch() domain.UserPatch {
	return domain.UserPatch{
		TenantID:      r.TenantID,
		DepartmentIDs: r.DepartmentIDs,
		Username:      r.Username,
		Status:        r.Status,
		Role:          r.Role,
		Display:       r.Display,
		Comment:       r.Comment,
	}
}
