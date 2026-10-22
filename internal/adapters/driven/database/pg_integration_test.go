//go:build integration

package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Run with: TEST_DATABASE_URL=... go test -tags integration ./internal/adapters/driven/database/
func openTestDB(t *testing.T) (*gorm.DB, *sql.DB) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, Migrate(context.Background(), sqlDB))
	return db, sqlDB
}

func TestPostgres_MigrateTwiceAndRoundTrip(t *testing.T) {
	db, sqlDB := openTestDB(t)
	ctx := context.Background()
	require.NoError(t, Migrate(ctx, sqlDB), "second run must be a no-op")

	tenants := NewTenantRepository(db)
	depts := NewDepartmentRepository(db)
	users := NewUserRepository(db)

	ten, err := tenants.Create(ctx, &domain.Tenant{ID: "t1", Name: "Lab A", Kind: domain.TenantKind("LAB")})
	require.NoError(t, err)
	assert.NotZero(t, ten.Seq)

	require.NoError(t, tenants.SaveLabels(ctx, "t1", domain.Labels{domain.LabelTenant: {"th": "ห้องแล็บ", "en": "Lab"}}))
	got, err := tenants.FindByID(ctx, "t1")
	require.NoError(t, err)
	assert.Equal(t, "ห้องแล็บ", got.Labels[domain.LabelTenant]["th"])

	d1, err := depts.Create(ctx, &domain.Department{ID: "d1", TenantID: "t1", Name: "QC"})
	require.NoError(t, err)
	taken, err := depts.NameTaken(ctx, "t1", "QC", "")
	require.NoError(t, err)
	assert.True(t, taken)

	u, err := users.Create(ctx, &domain.User{ID: "u1", TenantID: "t1", Username: "alice", Password: "x", Role: domain.RoleUser, Status: true, DepartmentIDs: []string{d1.ID}})
	require.NoError(t, err)
	found, err := users.FindByID(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, found.Departments, 1)
	assert.Equal(t, "Lab A", found.Tenant.Name)

	// last login is stamped without counting as an edit
	before := found.UpdatedAt
	stamp := time.Now().Add(-time.Minute).Truncate(time.Second)
	require.NoError(t, users.RecordLogin(ctx, "u1", stamp))
	again, err := users.FindByID(ctx, "u1")
	require.NoError(t, err)
	require.NotNil(t, again.LastLoginAt)
	assert.True(t, again.LastLoginAt.Equal(stamp))
	assert.True(t, again.UpdatedAt.Equal(before), "updated_at must not move on login")

	// audit rows keep the request metadata
	audits := NewAuditRepository(db)
	_, err = audits.Create(ctx, &domain.AuditLog{ID: "a-meta", Entity: domain.AuditEntityUser, EntityID: "u1",
		Action: domain.AuditActionLoginSuccess, TenantID: "t1", IP: "203.0.113.9", UserAgent: "curl/8", RequestID: "req-1"})
	require.NoError(t, err)
	logs, err := audits.FindAll(ctx, ports.AuditFilter{Action: "LOGIN_SUCCESS"})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "203.0.113.9", logs[0].IP)
	assert.Equal(t, "req-1", logs[0].RequestID)

	// tenant not empty
	has, err := tenants.HasMembers(ctx, "t1")
	require.NoError(t, err)
	assert.True(t, has)

	// soft delete user: hidden, username reserved, restorable
	_, err = users.Delete(ctx, "u1", "admin")
	require.NoError(t, err)
	gone, err := users.FindByID(ctx, "u1")
	require.NoError(t, err)
	assert.Nil(t, gone)
	reserved, err := users.UsernameTaken(ctx, "alice")
	require.NoError(t, err)
	assert.True(t, reserved)
	_, err = users.FindAnyByID(ctx, "u1")
	require.NoError(t, err)
	_, err = users.Restore(ctx, "u1")
	require.NoError(t, err)
	_, err = users.FindByID(ctx, "u1")
	require.NoError(t, err)

	// department name unique among live rows only
	_, err = depts.Delete(ctx, "d1", "admin")
	require.NoError(t, err)
	_, err = depts.Create(ctx, &domain.Department{ID: "d2", TenantID: "t1", Name: "QC"})
	require.NoError(t, err, "name reusable after soft delete")

	// listing with hidden filter
	list, err := tenants.FindAll(ctx, ports.TenantFilter{ExcludeHidden: true})
	require.NoError(t, err)
	assert.NotEmpty(t, list)

	// audit_logs is append-only
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO audit_logs (id, entity, entity_id, action) VALUES ('a1','USER','u1','CREATE')`)
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx, `UPDATE audit_logs SET action='X'`)
	assert.Error(t, err)
	_, err = sqlDB.ExecContext(ctx, `DELETE FROM audit_logs`)
	assert.Error(t, err)
}
