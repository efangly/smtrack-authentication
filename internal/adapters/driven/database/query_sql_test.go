package database

import (
	"context"
	"strings"
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

func TestUserFindAll_SQL(t *testing.T) {
	tests := []struct {
		name    string
		filter  ports.UserFilter
		want    []string
		notWant []string
	}{
		{"no filter", ports.UserFilter{}, nil, []string{"JOIN", "WHERE"}},
		{
			// SERVICE: excluding hospitals alone must still join Wards.
			"exclude hospitals only", ports.UserFilter{ExcludeHosIDs: []string{"HID-DEVELOPMENT"}},
			[]string{`JOIN "Wards" ON "Wards"."id" = "Users"."wardId"`, `"Wards"."hosId" NOT IN ($1)`}, nil,
		},
		{
			"admin", ports.UserFilter{HosID: "H1", ExcludeHosIDs: []string{"HID-DEVELOPMENT"}, ExcludeRoles: []domain.Role{domain.RoleService}},
			[]string{`JOIN "Wards"`, `"Wards"."hosId" = $1`, `"Wards"."hosId" NOT IN ($2)`, `"Users"."role" NOT IN ($3)`}, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, sqls := dryRunDB(t)
			_, err := NewUserRepository(db).FindAll(context.Background(), tt.filter)
			require.NoError(t, err)
			require.NotEmpty(t, *sqls)
			sql := (*sqls)[0]
			for _, w := range tt.want {
				assert.Contains(t, sql, w)
			}
			for _, nw := range tt.notWant {
				assert.NotContains(t, sql, nw)
			}
			assert.LessOrEqual(t, strings.Count(sql, "JOIN"), 1, "Wards joined more than once: %s", sql)
		})
	}
}

func TestWardAndHospitalFindAll_SQL(t *testing.T) {
	db, sqls := dryRunDB(t)
	_, err := NewWardRepository(db).FindAll(context.Background(), ports.WardFilter{HosID: "H1", ExcludeHosIDs: []string{"HID-DEVELOPMENT"}})
	require.NoError(t, err)
	assert.Contains(t, (*sqls)[0], `"hosId" = $1`)
	assert.Contains(t, (*sqls)[0], `"hosId" NOT IN ($2)`)

	db, sqls = dryRunDB(t)
	_, err = NewHospitalRepository(db).FindAll(context.Background(), ports.HospitalFilter{WardID: "W1", ExcludeIDs: []string{"HID-DEVELOPMENT"}})
	require.NoError(t, err)
	assert.Contains(t, (*sqls)[0], `"Wards".id = $1`)
	assert.Contains(t, (*sqls)[0], `"id" NOT IN ($2)`)
}
