//go:build integration

package legacyimport

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/efangly/smtrack-authentication/internal/adapters/driven/database"
)

const oldSchema = `
CREATE TABLE "Hospitals" ("id" varchar(100) PRIMARY KEY, "hosName" varchar(155) NOT NULL, "hosSeq" serial UNIQUE,
 "hosAddress" varchar(155), "hosTel" varchar(100), "userContact" varchar(155), "userTel" varchar(100),
 "hosLatitude" varchar(155), "hosLongitude" varchar(155), "hosPic" varchar(255), "hosType" varchar(20) DEFAULT 'HOSPITAL',
 "createAt" timestamptz, "updateAt" timestamptz);
CREATE TABLE "Wards" ("id" varchar(100) PRIMARY KEY, "wardName" varchar(250) NOT NULL, "wardSeq" serial UNIQUE,
 "type" varchar(20) DEFAULT 'NEW', "hosId" varchar(100) NOT NULL, "createAt" timestamptz, "updateAt" timestamptz);
CREATE TABLE "Users" ("id" varchar(100) PRIMARY KEY, "wardId" varchar(100) NOT NULL, "username" varchar(155) UNIQUE,
 "password" varchar(155), "status" boolean DEFAULT true, "role" varchar(20) DEFAULT 'GUEST', "display" varchar(150),
 "pic" varchar(255), "comment" varchar(255), "createBy" varchar(155), "createAt" timestamptz, "updateAt" timestamptz);
CREATE TABLE "AuditLogs" ("id" varchar(100) PRIMARY KEY, "entity" varchar(20) NOT NULL, "entityId" varchar(100) NOT NULL,
 "action" varchar(10) NOT NULL, "actorId" varchar(100), "actorName" varchar(150), "changes" text, "createAt" timestamptz);
INSERT INTO "Hospitals" ("id","hosName","hosSeq","hosType","createAt","updateAt") VALUES
 ('h1','Hospital One',7,'HOSPITAL',now(),now()), ('h2','Old Clinic',9,'LEGACY',now(),now()), ('HID-DEVELOPMENT','Dev',11,'LAB',now(),now());
INSERT INTO "Wards" ("id","wardName","wardSeq","type","hosId","createAt","updateAt") VALUES
 ('w1','ICU',3,'NEW','h1',now(),now()), ('w2','ER',5,'LEGACY','h2',now(),now()), ('w3','ICU',6,'NEW','h2',now(),now());
INSERT INTO "Users" ("id","wardId","username","password","role","createAt","updateAt") VALUES
 ('u1','w1','alice','$2a$10$hashhashhash','ADMIN',now(),now()), ('u2','w2','bob','$2a$10$other','USER',now(),now());
INSERT INTO "AuditLogs" ("id","entity","entityId","action","createAt") VALUES ('a1','WARD','w1','CREATE',now());
`

func open(t *testing.T, env string) *sql.DB {
	url := os.Getenv(env)
	if url == "" {
		t.Skip(env + " not set")
	}
	db, err := sql.Open("pgx", url)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestImportAndVerify(t *testing.T) {
	ctx := context.Background()
	old, dst := open(t, "LEGACY_TEST_DATABASE_URL"), open(t, "TEST_DATABASE_URL")
	_, err := old.ExecContext(ctx, oldSchema)
	require.NoError(t, err)
	require.NoError(t, database.Migrate(ctx, dst))

	rep, err := Import(ctx, old, dst)
	require.NoError(t, err)
	require.Empty(t, rep.Problems)
	assert.Equal(t, 3, rep.Tenants)
	assert.Equal(t, 2, rep.Users)

	// Twice: idempotent.
	rep, err = Import(ctx, old, dst)
	require.NoError(t, err)
	require.Empty(t, rep.Problems)

	v, err := Verify(ctx, old, dst)
	require.NoError(t, err)
	assert.Empty(t, v.Problems)

	var kind string
	var legacy, hidden bool
	require.NoError(t, dst.QueryRow(`SELECT kind, legacy, hidden FROM tenants WHERE id='h2'`).Scan(&kind, &legacy, &hidden))
	assert.Equal(t, "HOSPITAL", kind)
	assert.True(t, legacy)
	assert.False(t, hidden)
	require.NoError(t, dst.QueryRow(`SELECT kind, legacy, hidden FROM tenants WHERE id='HID-DEVELOPMENT'`).Scan(&kind, &legacy, &hidden))
	assert.Equal(t, "LAB", kind)
	assert.True(t, hidden)

	// The next generated seq continues after the imported ones.
	var seq int64
	require.NoError(t, dst.QueryRow(`INSERT INTO tenants (id,name) VALUES ('new','New') RETURNING seq`).Scan(&seq))
	assert.Equal(t, int64(12), seq)

	// A change in the old database is flagged by verify, then fixed by another import.
	_, err = old.ExecContext(ctx, `UPDATE "Users" SET "password"='$2a$10$changed' WHERE "id"='u1'`)
	require.NoError(t, err)
	v, err = Verify(ctx, old, dst)
	require.NoError(t, err)
	assert.Len(t, v.Problems, 1)
	_, err = Import(ctx, old, dst)
	require.NoError(t, err)
	v, err = Verify(ctx, old, dst)
	require.NoError(t, err)
	assert.Empty(t, v.Problems)

	// Duplicate ward names stop the import before anything is written.
	_, err = old.ExecContext(ctx, `INSERT INTO "Wards" ("id","wardName","hosId") VALUES ('w4','ICU','h1')`)
	require.NoError(t, err)
	rep, err = Import(ctx, old, dst)
	require.NoError(t, err)
	require.Len(t, rep.Problems, 1)
	assert.Contains(t, rep.Problems[0], "ICU")
}
