package dto

import "github.com/efangly/smtrack-authentication/internal/core/domain"

// CreateTenantRequest represents the create tenant request body
type CreateTenantRequest struct {
	ID          string            `json:"id,omitempty"`
	Name        string            `json:"name" validate:"required,max=150"`
	Kind        domain.TenantKind `json:"kind,omitempty"`
	Legacy      bool              `json:"legacy,omitempty"`
	Hidden      bool              `json:"hidden,omitempty"`
	Address     *string           `json:"address,omitempty" validate:"omitempty,max=255"`
	Tel         *string           `json:"tel,omitempty" validate:"omitempty,max=100"`
	ContactName *string           `json:"contactName,omitempty" validate:"omitempty,max=155"`
	ContactTel  *string           `json:"contactTel,omitempty" validate:"omitempty,min=9,max=20"`
	Latitude    *string           `json:"latitude,omitempty" validate:"omitempty,max=155"`
	Longitude   *string           `json:"longitude,omitempty" validate:"omitempty,max=155"`
}

// ToTenant converts the DTO to a domain Tenant. The picture is set only by
// uploading an image (see UpdateUserRequest).
func (r *CreateTenantRequest) ToTenant() *domain.Tenant {
	return &domain.Tenant{
		ID:          r.ID,
		Name:        r.Name,
		Kind:        r.Kind,
		Legacy:      r.Legacy,
		Hidden:      r.Hidden,
		Address:     r.Address,
		Tel:         r.Tel,
		ContactName: r.ContactName,
		ContactTel:  r.ContactTel,
		Latitude:    r.Latitude,
		Longitude:   r.Longitude,
	}
}

// UpdateTenantRequest represents the update tenant request body
type UpdateTenantRequest struct {
	Name        *string            `json:"name,omitempty" validate:"omitempty,max=150"`
	Kind        *domain.TenantKind `json:"kind,omitempty"`
	Legacy      *bool              `json:"legacy,omitempty"`
	Hidden      *bool              `json:"hidden,omitempty"`
	Address     *string            `json:"address,omitempty" validate:"omitempty,max=255"`
	Tel         *string            `json:"tel,omitempty" validate:"omitempty,max=100"`
	ContactName *string            `json:"contactName,omitempty" validate:"omitempty,max=155"`
	ContactTel  *string            `json:"contactTel,omitempty" validate:"omitempty,min=9,max=20"`
	Latitude    *string            `json:"latitude,omitempty" validate:"omitempty,max=155"`
	Longitude   *string            `json:"longitude,omitempty" validate:"omitempty,max=155"`
}

// ToPatch converts the update DTO to a partial update; unset fields stay nil.
// The picture is set only by uploading an image (see UpdateUserRequest).
func (r *UpdateTenantRequest) ToPatch() domain.TenantPatch {
	return domain.TenantPatch{
		Name:        r.Name,
		Kind:        r.Kind,
		Legacy:      r.Legacy,
		Hidden:      r.Hidden,
		Address:     r.Address,
		Tel:         r.Tel,
		ContactName: r.ContactName,
		ContactTel:  r.ContactTel,
		Latitude:    r.Latitude,
		Longitude:   r.Longitude,
	}
}

// UpdateLabelsRequest replaces a tenant's label profile: label -> language -> text.
type UpdateLabelsRequest struct {
	Labels domain.Labels `json:"labels" validate:"required"`
}
