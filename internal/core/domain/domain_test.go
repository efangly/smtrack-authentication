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
