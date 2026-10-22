package domain

import "time"

// User is an account that can sign in. It belongs to one tenant and to any
// number of that tenant's departments.
type User struct {
	ID          string
	TenantID    string
	Username    string
	Password    string
	Status      bool
	Role        Role
	Display     *string
	Pic         *string
	Comment     *string
	CreateBy    *string
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
	DeletedBy   *string

	// DepartmentIDs is the set of departments to link on Create. Reads fill
	// Departments instead.
	DepartmentIDs []string

	// Tenant and Departments are filled by reads that load them.
	Tenant      *Tenant
	Departments []Department
}

// LinkedDepartmentIDs lists the ids of the departments a read loaded.
func (u *User) LinkedDepartmentIDs() []string {
	ids := make([]string, len(u.Departments))
	for i, d := range u.Departments {
		ids[i] = d.ID
	}
	return ids
}

// WithoutSecrets returns a copy of u that is safe to store or hand on: the
// password hash is cleared. Use it before caching a user.
func (u User) WithoutSecrets() User {
	u.Password = ""
	return u
}

// UsersWithoutSecrets applies WithoutSecrets to every user in a new slice.
func UsersWithoutSecrets(users []User) []User {
	out := make([]User, len(users))
	for i, u := range users {
		out[i] = u.WithoutSecrets()
	}
	return out
}
