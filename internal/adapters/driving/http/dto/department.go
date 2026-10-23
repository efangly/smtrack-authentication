package dto

import "github.com/efangly/smtrack-authentication/internal/core/domain"

// CreateDepartmentRequest represents the create department request body
type CreateDepartmentRequest struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name" validate:"required,max=150"`
	TenantID string `json:"tenantId" validate:"required,max=100"`
	Legacy   bool   `json:"legacy,omitempty"`
}

// ToDepartment converts the DTO to a domain Department
func (r *CreateDepartmentRequest) ToDepartment() *domain.Department {
	return &domain.Department{
		ID:       r.ID,
		Name:     r.Name,
		TenantID: r.TenantID,
		Legacy:   r.Legacy,
	}
}

// UpdateDepartmentRequest represents the update department request body. A
// department cannot move to another tenant: the users linked to it belong to
// its tenant.
type UpdateDepartmentRequest struct {
	Name   *string `json:"name,omitempty" validate:"omitempty,max=150"`
	Legacy *bool   `json:"legacy,omitempty"`
}

// ToPatch converts the update DTO to a partial update; unset fields stay nil.
func (r *UpdateDepartmentRequest) ToPatch() domain.DepartmentPatch {
	return domain.DepartmentPatch{Name: r.Name, Legacy: r.Legacy}
}
