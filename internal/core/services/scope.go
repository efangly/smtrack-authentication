package services

import (
	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// canAdminTenant reports whether caller may read or change the users and
// settings of tenantID. SUPER and SERVICE act system-wide; ADMIN and
// LEGACY_ADMIN only within their own tenant. Everyone else may only act on
// their own record, which callers check before reaching here.
func canAdminTenant(caller *domain.Actor, tenantID string) bool {
	switch caller.Role {
	case domain.RoleSuper, domain.RoleService:
		return true
	case domain.RoleAdmin, domain.RoleLegacyAdmin:
		// An admin token without a tenant must not match tenant-less records.
		return caller.TenantID != "" && tenantID == caller.TenantID
	}
	return false
}

// canSeeTenant mirrors buildTenantFilter for a single tenant, so a lookup by
// id never reveals a tenant the caller's listing would hide.
func canSeeTenant(caller *domain.Actor, tenant *domain.Tenant) bool {
	switch caller.Role {
	case domain.RoleSuper:
		return true
	case domain.RoleService:
		return !tenant.Hidden
	case domain.RoleAdmin, domain.RoleLegacyAdmin:
		return caller.TenantID != "" && tenant.ID == caller.TenantID && !tenant.Hidden
	case domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest:
		return caller.TenantID != "" && tenant.ID == caller.TenantID
	}
	return false
}

// canSeeDepartment mirrors buildDepartmentFilter for a single department.
// department.Tenant must be loaded for the hidden-tenant rule to apply.
func canSeeDepartment(caller *domain.Actor, department *domain.Department) bool {
	hidden := department.Tenant != nil && department.Tenant.Hidden
	switch caller.Role {
	case domain.RoleSuper:
		return true
	case domain.RoleService:
		return !hidden
	case domain.RoleAdmin, domain.RoleLegacyAdmin:
		return caller.TenantID != "" && department.TenantID == caller.TenantID && !hidden
	case domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest:
		return caller.TenantID != "" && department.TenantID == caller.TenantID && caller.InDepartment(department.ID)
	}
	return false
}

// visibleDepartments narrows the departments embedded in a tenant to the ones
// caller may see: a tenant-scoped admin and above see all, anyone else only
// the departments they are linked to. It returns a new slice.
func visibleDepartments(caller *domain.Actor, departments []domain.Department) []domain.Department {
	if caller.IsAdminTier() {
		return departments
	}
	out := make([]domain.Department, 0, len(departments))
	for _, d := range departments {
		if caller.InDepartment(d.ID) {
			out = append(out, d)
		}
	}
	return out
}
