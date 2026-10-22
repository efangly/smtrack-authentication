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

// CanAssign reports whether a caller holding role r may give target to a
// user — and, by the same rule, may modify or delete a user who already holds
// target. This is the role ceiling: nobody can hand out (or act on) a role
// above what their own tier allows.
func (r Role) CanAssign(target Role) bool {
	if !target.IsValid() {
		return false
	}
	switch r {
	case RoleSuper:
		return true
	case RoleService:
		return target != RoleSuper
	case RoleAdmin, RoleLegacyAdmin:
		switch target {
		case RoleUser, RoleLegacyUser, RoleGuest:
			return true
		}
	}
	return false
}

// TenantKind is what sort of organisation a tenant is.
type TenantKind string

const (
	TenantKindHospital TenantKind = "HOSPITAL"
	TenantKindLab      TenantKind = "LAB"
	TenantKindFactory  TenantKind = "FACTORY"
	TenantKindClinic   TenantKind = "CLINIC"
	TenantKindPharmacy TenantKind = "PHARMACY"
	TenantKindOther    TenantKind = "OTHER"
)

func (k TenantKind) IsValid() bool {
	switch k {
	case TenantKindHospital, TenantKindLab, TenantKindFactory, TenantKindClinic, TenantKindPharmacy, TenantKindOther:
		return true
	}
	return false
}
