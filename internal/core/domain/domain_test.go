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

func TestHosType_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		ht    HosType
		valid bool
	}{
		{"HOSPITAL", HosTypeHospital, true},
		{"LEGACY", HosTypeLegacy, true},
		{"CLINIC", HosTypeClinic, true},
		{"PHARMACY", HosTypePharmacy, true},
		{"LAB", HosTypeLab, true},
		{"OTHER", HosTypeOther, true},
		{"empty", HosType(""), false},
		{"INVALID", HosType("INVALID"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.ht.IsValid())
		})
	}
}

func TestWardType_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		wt    WardType
		valid bool
	}{
		{"NEW", WardTypeNew, true},
		{"LEGACY", WardTypeLegacy, true},
		{"empty", WardType(""), false},
		{"INVALID", WardType("INVALID"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.wt.IsValid())
		})
	}
}

func TestUser_TableName(t *testing.T) {
	assert.Equal(t, "Users", User{}.TableName())
}

func TestWard_TableName(t *testing.T) {
	assert.Equal(t, "Wards", Ward{}.TableName())
}

func TestHospital_TableName(t *testing.T) {
	assert.Equal(t, "Hospitals", Hospital{}.TableName())
}
