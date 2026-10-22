package database

import (
	"context"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// dryRunDB builds SQL without a database: DryRun stops before execution and
// the callback records each query statement.
func dryRunDB(t *testing.T) (*gorm.DB, *[]string) {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               logger.Discard,
	})
	require.NoError(t, err)
	var sqls []string
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:capture", func(tx *gorm.DB) {
		sqls = append(sqls, tx.Statement.SQL.String())
	}))
	return db, &sqls
}

// mainSQL is the statement the repository ran. A subquery passed to Where is
// executed (in dry-run) first and recorded too, so the main query is the last.
func mainSQL(sqls *[]string) string { return (*sqls)[len(*sqls)-1] }

func TestUserFindAll_SQL(t *testing.T) {
	tests := []struct {
		name    string
		filter  ports.UserFilter
		want    []string
		notWant []string
	}{
		{"no filter", ports.UserFilter{}, []string{`"users"."deleted_at" IS NULL`}, []string{"tenant_id", "NOT IN"}},
		{
			"tenant", ports.UserFilter{TenantID: "T1"},
			[]string{`users.tenant_id = $1`, `"users"."deleted_at" IS NULL`}, nil,
		},
		{
			// SERVICE: hidden tenants are excluded by a subquery, so no join is needed.
			"exclude hidden tenants", ports.UserFilter{ExcludeHiddenTenants: true},
			[]string{`users.tenant_id NOT IN (SELECT "id" FROM "tenants" WHERE hidden = $1)`}, []string{"JOIN"},
		},
		{
			"admin", ports.UserFilter{TenantID: "T1", ExcludeHiddenTenants: true, ExcludeRoles: []domain.Role{domain.RoleService}},
			[]string{`users.tenant_id = $1`, `users.tenant_id NOT IN (SELECT`, `users.role NOT IN ($3)`}, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, sqls := dryRunDB(t)
			_, err := NewUserRepository(db).FindAll(context.Background(), tt.filter)
			require.NoError(t, err)
			require.NotEmpty(t, *sqls)
			sql := mainSQL(sqls)
			for _, w := range tt.want {
				assert.Contains(t, sql, w)
			}
			for _, nw := range tt.notWant {
				assert.NotContains(t, sql, nw)
			}
		})
	}
}

// Soft-deleted rows are invisible to every ordinary read, and only the
// FindAny* lookups see them.
func TestSoftDeleteScope_SQL(t *testing.T) {
	ctx := context.Background()

	db, sqls := dryRunDB(t)
	_, err := NewUserRepository(db).FindByID(ctx, "U1")
	require.NoError(t, err)
	assert.Contains(t, (*sqls)[0], `"users"."deleted_at" IS NULL`)

	db, sqls = dryRunDB(t)
	_, err = NewUserRepository(db).FindByUsername(ctx, "bob")
	require.NoError(t, err)
	assert.Contains(t, (*sqls)[0], `"users"."deleted_at" IS NULL`)

	db, sqls = dryRunDB(t)
	_, err = NewUserRepository(db).FindAnyByID(ctx, "U1")
	require.NoError(t, err)
	assert.NotContains(t, (*sqls)[0], "deleted_at")

	// A username stays reserved after a soft delete.
	db, sqls = dryRunDB(t)
	_, err = NewUserRepository(db).UsernameTaken(ctx, "bob")
	require.NoError(t, err)
	assert.NotContains(t, (*sqls)[0], "deleted_at")

	db, sqls = dryRunDB(t)
	_, err = NewTenantRepository(db).FindByID(ctx, "T1")
	require.NoError(t, err)
	assert.Contains(t, (*sqls)[0], `"tenants"."deleted_at" IS NULL`)

	db, sqls = dryRunDB(t)
	_, err = NewTenantRepository(db).FindAnyByID(ctx, "T1")
	require.NoError(t, err)
	assert.NotContains(t, (*sqls)[0], "deleted_at")

	// The department name uniqueness only covers live departments.
	db, sqls = dryRunDB(t)
	_, err = NewDepartmentRepository(db).NameTaken(ctx, "T1", "Line", "")
	require.NoError(t, err)
	assert.Contains(t, (*sqls)[0], `"departments"."deleted_at" IS NULL`)
}

func TestDepartmentAndTenantFindAll_SQL(t *testing.T) {
	ctx := context.Background()

	db, sqls := dryRunDB(t)
	_, err := NewDepartmentRepository(db).FindAll(ctx, ports.DepartmentFilter{
		TenantID: "T1", IDs: []string{"D1", "D2"}, LimitToIDs: true, ExcludeHiddenTenants: true,
	})
	require.NoError(t, err)
	assert.Contains(t, mainSQL(sqls), `departments.tenant_id = $1`)
	assert.Contains(t, mainSQL(sqls), `departments.id IN ($2,$3)`)
	assert.Contains(t, mainSQL(sqls), `departments.tenant_id NOT IN (SELECT`)

	// Limited to an empty set of ids matches nothing and does not query at all.
	db, sqls = dryRunDB(t)
	got, err := NewDepartmentRepository(db).FindAll(ctx, ports.DepartmentFilter{LimitToIDs: true})
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, *sqls)

	db, sqls = dryRunDB(t)
	_, err = NewTenantRepository(db).FindAll(ctx, ports.TenantFilter{ID: "T1", ExcludeHidden: true})
	require.NoError(t, err)
	assert.Contains(t, (*sqls)[0], `tenants.id = $1`)
	assert.Contains(t, (*sqls)[0], `tenants.hidden = $2`)
}

func TestLabelsJSON_RoundTrip(t *testing.T) {
	in := labelsJSON{"tenant": {"th": "โรงงาน", "en": "Factory"}}
	value, err := in.Value()
	require.NoError(t, err)

	var out labelsJSON
	require.NoError(t, out.Scan([]byte(value.(string))))
	assert.Equal(t, in, out)

	var empty labelsJSON
	v, err := empty.Value()
	require.NoError(t, err)
	assert.Equal(t, "{}", v)
	require.NoError(t, empty.Scan(nil))
	assert.Nil(t, empty)
}
