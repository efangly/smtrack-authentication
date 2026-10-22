package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRole_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		role  Role
		valid bool
	}{
		{"SUPER is valid", RoleSuper, true},
		{"SERVICE is valid", RoleService, true},
		{"ADMIN is valid", RoleAdmin, true},
		{"USER is valid", RoleUser, true},
		{"LEGACY_ADMIN is valid", RoleLegacyAdmin, true},
		{"LEGACY_USER is valid", RoleLegacyUser, true},
		{"GUEST is valid", RoleGuest, true},
		{"empty is invalid", Role(""), false},
		{"random is invalid", Role("RANDOM"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.role.IsValid())
		})
	}
}

func TestRole_CanAssign(t *testing.T) {
	tests := []struct {
		name   string
		caller Role
		target Role
		want   bool
	}{
		{"SUPER may assign SUPER", RoleSuper, RoleSuper, true},
		{"SUPER may assign GUEST", RoleSuper, RoleGuest, true},
		{"SERVICE may not assign SUPER", RoleService, RoleSuper, false},
		{"SERVICE may assign SERVICE", RoleService, RoleService, true},
		{"SERVICE may assign ADMIN", RoleService, RoleAdmin, true},
		{"ADMIN may not assign SUPER", RoleAdmin, RoleSuper, false},
		{"ADMIN may not assign SERVICE", RoleAdmin, RoleService, false},
		{"ADMIN may not assign ADMIN", RoleAdmin, RoleAdmin, false},
		{"ADMIN may not assign LEGACY_ADMIN", RoleAdmin, RoleLegacyAdmin, false},
		{"ADMIN may assign USER", RoleAdmin, RoleUser, true},
		{"LEGACY_ADMIN may assign LEGACY_USER", RoleLegacyAdmin, RoleLegacyUser, true},
		{"LEGACY_ADMIN may not assign ADMIN", RoleLegacyAdmin, RoleAdmin, false},
		{"USER may not assign GUEST", RoleUser, RoleGuest, false},
		{"GUEST may not assign GUEST", RoleGuest, RoleGuest, false},
		{"nobody may assign an unknown role", RoleSuper, Role("ROOT"), false},
		{"nobody may assign an empty role", RoleSuper, Role(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.caller.CanAssign(tt.target))
		})
	}
}

func TestTenantKind_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		kind  TenantKind
		valid bool
	}{
		{"HOSPITAL", TenantKindHospital, true},
		{"LAB", TenantKindLab, true},
		{"FACTORY", TenantKindFactory, true},
		{"CLINIC", TenantKindClinic, true},
		{"PHARMACY", TenantKindPharmacy, true},
		{"OTHER", TenantKindOther, true},
		{"LEGACY is a flag, not a kind", TenantKind("LEGACY"), false},
		{"empty", TenantKind(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.kind.IsValid())
		})
	}
}

func TestActor_InDepartment(t *testing.T) {
	a := &Actor{DepartmentIDs: []string{"D1", "D2"}}
	assert.True(t, a.InDepartment("D2"))
	assert.False(t, a.InDepartment("D3"))
	assert.False(t, (&Actor{}).InDepartment("D1"))
}

func TestUser_LinkedDepartmentIDs(t *testing.T) {
	u := User{Departments: []Department{{ID: "D1"}, {ID: "D2"}}}
	assert.Equal(t, []string{"D1", "D2"}, u.LinkedDepartmentIDs())
	assert.Empty(t, (&User{}).LinkedDepartmentIDs())
}

func TestActor_IsAdminTier(t *testing.T) {
	admin := map[Role]bool{RoleSuper: true, RoleService: true, RoleAdmin: true, RoleLegacyAdmin: true}
	for _, role := range []Role{RoleSuper, RoleService, RoleAdmin, RoleLegacyAdmin, RoleUser, RoleLegacyUser, RoleGuest} {
		t.Run(string(role), func(t *testing.T) {
			assert.Equal(t, admin[role], (&Actor{Role: role}).IsAdminTier())
		})
	}
}

func TestActor_NilIsUnprivileged(t *testing.T) {
	var a *Actor
	assert.False(t, a.IsAdminTier())
	assert.False(t, a.CanAssign(RoleGuest))
}

func TestActor_CanAssign_DelegatesToRole(t *testing.T) {
	assert.True(t, (&Actor{Role: RoleService}).CanAssign(RoleAdmin))
	assert.False(t, (&Actor{Role: RoleAdmin}).CanAssign(RoleAdmin))
}
