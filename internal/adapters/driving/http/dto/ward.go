package dto

import "github.com/efangly/smtrack-authentication/internal/core/domain"

// CreateWardRequest represents the create ward request body
type CreateWardRequest struct {
	ID       string           `json:"id,omitempty"`
	WardName string           `json:"wardName" validate:"required,max=150"`
	WardSeq  *int             `json:"wardSeq,omitempty"`
	Type     *domain.WardType `json:"type,omitempty"`
	HosID    string           `json:"hosId" validate:"required,max=100"`
}

// ToWard converts the DTO to a domain Ward
func (r *CreateWardRequest) ToWard() *domain.Ward {
	ward := &domain.Ward{
		ID:       r.ID,
		WardName: r.WardName,
		HosID:    r.HosID,
		Type:     r.Type,
	}
	if r.WardSeq != nil {
		ward.WardSeq = *r.WardSeq
	}
	if r.Type == nil {
		t := domain.WardTypeNew
		ward.Type = &t
	}
	return ward
}

// UpdateWardRequest represents the update ward request body
type UpdateWardRequest struct {
	ID       string           `json:"id,omitempty"`
	WardName *string          `json:"wardName,omitempty"`
	WardSeq  *int             `json:"wardSeq,omitempty"`
	Type     *domain.WardType `json:"type,omitempty"`
	HosID    *string          `json:"hosId,omitempty"`
}

// ToPatch converts the update DTO to a partial update; unset fields stay nil.
func (r *UpdateWardRequest) ToPatch() domain.WardPatch {
	return domain.WardPatch{
		WardName: r.WardName,
		WardSeq:  r.WardSeq,
		Type:     r.Type,
		HosID:    r.HosID,
	}
}
