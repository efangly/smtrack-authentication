package domain

// Role represents user roles in the system
type Role string

const (
	RoleSuper       Role = "SUPER"
	RoleService     Role = "SERVICE"
	RoleAdmin       Role = "ADMIN"
	RoleUser        Role = "USER"
	RoleLegacyAdmin Role = "LEGACY_ADMIN"
	RoleLegacyUser  Role = "LEGACY_USER"
	RoleGuest       Role = "GUEST"
)

func (r Role) IsValid() bool {
	switch r {
	case RoleSuper, RoleService, RoleAdmin, RoleUser, RoleLegacyAdmin, RoleLegacyUser, RoleGuest:
		return true
	}
	return false
}

// HosType represents hospital types
type HosType string

const (
	HosTypeHospital HosType = "HOSPITAL"
	HosTypeLegacy   HosType = "LEGACY"
	HosTypeClinic   HosType = "CLINIC"
	HosTypePharmacy HosType = "PHARMACY"
	HosTypeLab      HosType = "LAB"
	HosTypeOther    HosType = "OTHER"
)

func (h HosType) IsValid() bool {
	switch h {
	case HosTypeHospital, HosTypeLegacy, HosTypeClinic, HosTypePharmacy, HosTypeLab, HosTypeOther:
		return true
	}
	return false
}

// WardType represents ward types
type WardType string

const (
	WardTypeNew    WardType = "NEW"
	WardTypeLegacy WardType = "LEGACY"
)

func (w WardType) IsValid() bool {
	switch w {
	case WardTypeNew, WardTypeLegacy:
		return true
	}
	return false
}
