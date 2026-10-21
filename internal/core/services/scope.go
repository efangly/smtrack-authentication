package services

import (
	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// developmentHosID is the internal test hospital hidden from every role but
// SUPER in the listing queries (see build*Filter).
const developmentHosID = "HID-DEVELOPMENT"

// canManageUsersIn reports whether caller may read or change a user belonging
// to hosID. SUPER and SERVICE manage users system-wide; ADMIN and
// LEGACY_ADMIN only within their own hospital. Everyone else may only act on
// their own record, which callers check before reaching here.
func canManageUsersIn(caller *domain.Actor, hosID string) bool {
	switch caller.Role {
	case domain.RoleSuper, domain.RoleService:
		return true
	case domain.RoleAdmin, domain.RoleLegacyAdmin:
		// An admin token without a hospital must not match ward-less users.
		return caller.HosID != "" && hosID == caller.HosID
	}
	return false
}

// userHosID is the hospital a user belongs to through its ward, or "" when the
// ward was not loaded or does not exist.
func userHosID(user *domain.User) string {
	if user.Ward == nil {
		return ""
	}
	return user.Ward.HosID
}

// canSeeHospital mirrors buildHospitalFilter for a single hospital, so a
// lookup by id never reveals a hospital the caller's listing would hide.
func canSeeHospital(caller *domain.Actor, hospital *domain.Hospital) bool {
	switch caller.Role {
	case domain.RoleSuper:
		return true
	case domain.RoleService:
		return hospital.ID != developmentHosID
	case domain.RoleAdmin, domain.RoleLegacyAdmin:
		return caller.HosID != "" && hospital.ID == caller.HosID && hospital.ID != developmentHosID
	case domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest:
		if caller.WardID == "" {
			return false
		}
		for _, w := range hospital.Wards {
			if w.ID == caller.WardID {
				return true
			}
		}
	}
	return false
}

// canSeeWard mirrors buildWardFilter for a single ward.
func canSeeWard(caller *domain.Actor, ward *domain.Ward) bool {
	switch caller.Role {
	case domain.RoleSuper:
		return true
	case domain.RoleService:
		return ward.HosID != developmentHosID
	case domain.RoleAdmin, domain.RoleLegacyAdmin:
		return caller.HosID != "" && ward.HosID == caller.HosID && ward.HosID != developmentHosID
	case domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest:
		// Listing is unscoped for these roles today (buildWardFilter
		// default), so a lookup by id stays unscoped too.
		return true
	}
	return false
}
