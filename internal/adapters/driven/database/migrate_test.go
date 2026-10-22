package database

import (
	"database/sql"
	"testing"

	"github.com/efangly/smtrack-authentication/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

// The embedded migrations must parse and be numbered without gaps. Applying them needs a real
// PostgreSQL, which `go test ./...` does not assume; sql.Open is lazy so nothing connects here.
func TestEmbeddedMigrationsParse(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://unused@localhost/unused")
	require.NoError(t, err)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	require.NoError(t, err)

	sources := provider.ListSources()
	require.Len(t, sources, 2)
	for i, s := range sources {
		require.EqualValues(t, i+1, s.Version)
	}
}
