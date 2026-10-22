package domain

// The *Patch types are partial updates: a nil field is left unchanged, so an
// update can never zero a field the client did not send.

// UserPatch is a partial update of a user.
type UserPatch struct {
	TenantID *string
	// DepartmentIDs replaces the user's department links when set; an empty
	// non-nil slice removes them all.
	DepartmentIDs *[]string
	Username      *string
	Status        *bool
	Role          *Role
	Display       *string
	Pic           *string
	Comment       *string
	// Password holds the plain value; userService.Update hashes it before it
	// reaches the repository.
	Password *string
}

// DepartmentPatch is a partial update of a department.
type DepartmentPatch struct {
	Name   *string
	Legacy *bool
}

// TenantPatch is a partial update of a tenant.
type TenantPatch struct {
	Name        *string
	Kind        *TenantKind
	Legacy      *bool
	Hidden      *bool
	Address     *string
	Tel         *string
	ContactName *string
	ContactTel  *string
	Latitude    *string
	Longitude   *string
	Pic         *string
}
