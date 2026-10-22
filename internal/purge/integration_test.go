//go:build integration

package purge

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/efangly/smtrack-authentication/internal/adapters/driven/database"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

type fakeFiles struct {
	deleted []string
	fail    bool
}

func (f *fakeFiles) Upload(context.Context, *domain.File, string) (string, error) { return "", nil }
func (f *fakeFiles) Delete(path, name string) error {
	if f.fail {
		return errors.New("upload service down")
	}
	f.deleted = append(f.deleted, path+"/"+name)
	return nil
}

func TestPurge(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", url)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.Migrate(ctx, db))

	old := time.Now().Add(-400 * 24 * time.Hour)
	recent := time.Now().Add(-10 * 24 * time.Hour)
	exec := func(q string, a ...any) { _, err := db.ExecContext(ctx, q, a...); require.NoError(t, err) }
	exec(`INSERT INTO tenants (id,name,pic,deleted_at) VALUES ('t-old','Old','t.png',$1),('t-live','Live',NULL,NULL),('t-recent','Recent',NULL,$2)`, old, recent)
	exec(`INSERT INTO departments (id,tenant_id,name,deleted_at) VALUES ('d-old','t-old','A',$1),('d-live','t-live','B',NULL)`, old)
	exec(`INSERT INTO users (id,tenant_id,username,password,pic,deleted_at) VALUES ('u-old','t-old','x','h','u.png',$1),('u-recent','t-live','y','h',NULL,$2),('u-live','t-live','z','h',NULL,NULL)`, old, recent)
	exec(`INSERT INTO user_departments (user_id,department_id) VALUES ('u-old','d-old')`)

	cutoff := time.Now().Add(-365 * 24 * time.Hour)
	files := &fakeFiles{}

	dry, err := Run(ctx, db, files, cutoff, true)
	require.NoError(t, err)
	assert.Equal(t, 1, dry.Users)
	assert.Empty(t, files.deleted)

	files.fail = true
	res, err := Run(ctx, db, files, cutoff, false)
	require.NoError(t, err)
	assert.Equal(t, 0, res.Users+res.Tenants)
	assert.Equal(t, 1, res.Departments, "a department has no picture to fail on")
	assert.NotEmpty(t, res.Skipped)

	files.fail = false
	res, err = Run(ctx, db, files, cutoff, false)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Users)
	assert.Equal(t, 1, res.Tenants)
	assert.Empty(t, res.Skipped)
	assert.ElementsMatch(t, []string{"user/u.png", "tenant/t.png"}, files.deleted)

	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM users`).Scan(&n))
	assert.Equal(t, 2, n)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM tenants`).Scan(&n))
	assert.Equal(t, 2, n)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM audit_logs WHERE action='PURGE'`).Scan(&n))
	assert.Equal(t, 3, n)
}
