package dto

import "github.com/tng-coop/auth-service/internal/core/domain"

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
	HosPic       *string        `json:"hosPic,omitempty" validate:"omitempty,max=150"`
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
		HosPic:       r.HosPic,
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
	HosPic       *string         `json:"hosPic,omitempty"`
	HosType      *domain.HosType `json:"hosType,omitempty"`
}

// ToMap converts the update DTO to a map for partial update
func (r *UpdateHospitalRequest) ToMap() map[string]any {
	data := make(map[string]any)
	if r.HosName != nil {
		data["hosName"] = *r.HosName
	}
	if r.HosAddress != nil {
		data["hosAddress"] = *r.HosAddress
	}
	if r.HosTel != nil {
		data["hosTel"] = *r.HosTel
	}
	if r.UserContact != nil {
		data["userContact"] = *r.UserContact
	}
	if r.UserTel != nil {
		data["userTel"] = *r.UserTel
	}
	if r.HosLatitude != nil {
		data["hosLatitude"] = *r.HosLatitude
	}
	if r.HosLongitude != nil {
		data["hosLongitude"] = *r.HosLongitude
	}
	if r.HosPic != nil {
		data["hosPic"] = *r.HosPic
	}
	if r.HosType != nil {
		data["hosType"] = string(*r.HosType)
	}
	return data
}
