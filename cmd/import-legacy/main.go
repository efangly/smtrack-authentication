// Command import-legacy copies the old hospital/ward database into the tenant/department
// one. Usage:
//
//	LEGACY_DATABASE_URL=... DATABASE_URL=... import-legacy [--verify]
//
// The old database is only read. Run it again for the final import at cutover, then run it
// with --verify; a clean verify is the cutover precondition (docs/adr/0004).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/efangly/smtrack-authentication/internal/adapters/driven/database"
	"github.com/efangly/smtrack-authentication/internal/legacyimport"
)

func main() {
	verify := flag.Bool("verify", false, "compare the two databases instead of importing")
	flag.Parse()
	if err := run(*verify); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(verify bool) error {
	oldURL, newURL := os.Getenv("LEGACY_DATABASE_URL"), os.Getenv("DATABASE_URL")
	if oldURL == "" || newURL == "" {
		return fmt.Errorf("LEGACY_DATABASE_URL and DATABASE_URL are required")
	}
	if oldURL == newURL {
		return fmt.Errorf("LEGACY_DATABASE_URL and DATABASE_URL must be different databases")
	}
	ctx := context.Background()
	old, err := sql.Open("pgx", oldURL)
	if err != nil {
		return fmt.Errorf("open legacy database: %w", err)
	}
	defer old.Close()
	dst, err := sql.Open("pgx", newURL)
	if err != nil {
		return fmt.Errorf("open new database: %w", err)
	}
	defer dst.Close()
	if err := database.Migrate(ctx, dst); err != nil {
		return err
	}

	var rep *legacyimport.Report
	if verify {
		rep, err = legacyimport.Verify(ctx, old, dst)
	} else {
		rep, err = legacyimport.Import(ctx, old, dst)
	}
	if err != nil {
		return err
	}
	verb := "imported"
	if verify {
		verb = "checked"
	}
	fmt.Printf("%s: %d tenants, %d departments, %d users, %d memberships, %d audit logs\n",
		verb, rep.Tenants, rep.Departments, rep.Users, rep.Memberships, rep.AuditLogs)
	if len(rep.Problems) > 0 {
		fmt.Printf("%d problem(s):\n", len(rep.Problems))
		for _, p := range rep.Problems {
			fmt.Println(" -", p)
		}
		return fmt.Errorf("not clean")
	}
	fmt.Println("ok")
	return nil
}
