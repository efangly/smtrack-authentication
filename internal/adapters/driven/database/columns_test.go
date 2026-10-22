package database

import (
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// withoutStamp checks updated_at is a fresh time and removes it for comparison.
func withoutStamp(t *testing.T, cols map[string]any) map[string]any {
	t.Helper()
	stamp, ok := cols["updated_at"].(time.Time)
	require.True(t, ok, "updated_at must always be set")
	assert.WithinDuration(t, time.Now(), stamp, time.Minute)
	delete(cols, "updated_at")
	return cols
}

func TestUserColumns(t *testing.T) {
	assert.Empty(t, withoutStamp(t, userColumns(domain.UserPatch{})))

	cols := withoutStamp(t, userColumns(domain.UserPatch{
		TenantID: ptr("T1"), Username: ptr("u"), Status: ptr(false), Role: ptr(domain.RoleAdmin),
		Display: ptr("d"), Pic: ptr("p"), Comment: ptr("c"), Password: ptr("hash"),
		// Department links are not a column; the repository rewrites the link table.
		DepartmentIDs: &[]string{"D1"},
	}))
	assert.Equal(t, map[string]any{
		"tenant_id": "T1", "username": "u", "status": false, "role": "ADMIN",
		"display": "d", "pic": "p", "comment": "c", "password": "hash",
	}, cols)
}

func TestDepartmentColumns(t *testing.T) {
	cols := withoutStamp(t, departmentColumns(domain.DepartmentPatch{Name: ptr("A"), Legacy: ptr(true)}))
	assert.Equal(t, map[string]any{"name": "A", "legacy": true}, cols)
}

func TestTenantColumns(t *testing.T) {
	cols := withoutStamp(t, tenantColumns(domain.TenantPatch{
		Name: ptr("T"), Kind: ptr(domain.TenantKindFactory), Legacy: ptr(false), Hidden: ptr(true),
		Address: ptr("a"), Tel: ptr("t"), ContactName: ptr("cn"), ContactTel: ptr("ct"),
		Latitude: ptr("1"), Longitude: ptr("2"), Pic: ptr("p"),
	}))
	assert.Equal(t, map[string]any{
		"name": "T", "kind": "FACTORY", "legacy": false, "hidden": true,
		"address": "a", "tel": "t", "contact_name": "cn", "contact_tel": "ct",
		"latitude": "1", "longitude": "2", "pic": "p",
	}, cols)
}
