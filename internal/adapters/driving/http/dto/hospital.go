package dto

import "github.com/efangly/smtrack-authentication/internal/core/domain"

// CreateHospitalRequest represents the create hospital request body
type CreateHospitalRequest struct {
	ID           string         `json:"id,omitempty"`
	HosName      string         `json:"hosName" validate:"required,max=150"`
	HosSeq       *int           `json:"hosSeq,omitempty"`
	HosAddress   *string        `json:"hosAddress,omitempty" validate:"omitempty,max=255"`
	HosTel       *string        `json:"hosTel,omitempty" validate:"omitempty,max=255"`
	UserContact  *string        `json:"userContact,omitempty" validate:"omitempty,max=255"`
	UserTel      *string        `json:"userTel,omitempty" validate:"omitempty,min=9,max=20"`
	HosLatitude  *string        `json:"hosLatitude,omitempty" validate:"omitempty,max=255"`
	HosLongitude *string        `json:"hosLongitude,omitempty" validate:"omitempty,max=255"`
	HosType      domain.HosType `json:"hosType,omitempty"`
}

// ToHospital converts the DTO to a domain Hospital
func (r *CreateHospitalRequest) ToHospital() *domain.Hospital {
	hospital := &domain.Hospital{
		ID:           r.ID,
		HosName:      r.HosName,
		HosAddress:   r.HosAddress,
		HosTel:       r.HosTel,
		UserContact:  r.UserContact,
		UserTel:      r.UserTel,
		HosLatitude:  r.HosLatitude,
		HosLongitude: r.HosLongitude,
		HosType:      r.HosType,
	}
	if r.HosType == "" {
		hospital.HosType = domain.HosTypeHospital
	}
	return hospital
}

// UpdateHospitalRequest represents the update hospital request body
type UpdateHospitalRequest struct {
	HosName      *string         `json:"hosName,omitempty"`
	HosAddress   *string         `json:"hosAddress,omitempty"`
	HosTel       *string         `json:"hosTel,omitempty"`
	UserContact  *string         `json:"userContact,omitempty"`
	UserTel      *string         `json:"userTel,omitempty"`
	HosLatitude  *string         `json:"hosLatitude,omitempty"`
	HosLongitude *string         `json:"hosLongitude,omitempty"`
	HosType      *domain.HosType `json:"hosType,omitempty"`
}

// ToPatch converts the update DTO to a partial update; unset fields stay nil.
// hosPic is set only by uploading an image (see UpdateUserRequest).
func (r *UpdateHospitalRequest) ToPatch() domain.HospitalPatch {
	return domain.HospitalPatch{
		HosName:      r.HosName,
		HosAddress:   r.HosAddress,
		HosTel:       r.HosTel,
		UserContact:  r.UserContact,
		UserTel:      r.UserTel,
		HosLatitude:  r.HosLatitude,
		HosLongitude: r.HosLongitude,
		HosType:      r.HosType,
	}
}
