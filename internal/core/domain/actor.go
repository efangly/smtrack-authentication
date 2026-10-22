package domain

// Actor is the user performing the current request, as identified by their
// access token. A nil *Actor means there is no authenticated user.
type Actor struct {
	ID            string
	Name          string
	Role          Role
	TenantID      string
	DepartmentIDs []string
}

// IsAdminTier reports whether the actor may act on records belonging to other
// users. Everyone else is restricted to their own account.
func (a *Actor) IsAdminTier() bool {
	if a == nil {
		return false
	}
	switch a.Role {
	case RoleSuper, RoleService, RoleAdmin, RoleLegacyAdmin:
		return true
	}
	return false
}

// CanAssign reports whether the actor may give role to a user, or modify or
// delete a user who already holds it. See Role.CanAssign.
func (a *Actor) CanAssign(role Role) bool {
	if a == nil {
		return false
	}
	return a.Role.CanAssign(role)
}

// InDepartment reports whether the actor is linked to the department.
func (a *Actor) InDepartment(id string) bool {
	for _, d := range a.DepartmentIDs {
		if d == id {
			return true
		}
	}
	return false
}
