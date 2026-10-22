// Command purge permanently removes records soft-deleted longer ago than
// SOFT_DELETE_RETENTION (default 365d), with their pictures. Run it from a scheduled job.
//
//	DATABASE_URL=... UPLOAD_PATH=... purge [--dry-run]
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/adapters/driven/upload"
	"github.com/efangly/smtrack-authentication/internal/purge"
)

func main() {
	dry := flag.Bool("dry-run", false, "report what would be purged without changing anything")
	flag.Parse()
	if err := run(*dry); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(dry bool) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	retention, err := parseRetention(os.Getenv("SOFT_DELETE_RETENTION"))
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	files := upload.NewFileUploadAdapter(&config.Config{UploadPath: os.Getenv("UPLOAD_PATH")})
	cutoff := time.Now().Add(-retention)
	res, err := purge.Run(context.Background(), db, files, cutoff, dry)
	if err != nil {
		return err
	}
	verb := "purged"
	if dry {
		verb = "would purge"
	}
	fmt.Printf("%s: %d users, %d departments, %d tenants (deleted before %s)\n",
		verb, res.Users, res.Departments, res.Tenants, cutoff.Format(time.RFC3339))
	for _, s := range res.Skipped {
		fmt.Println(" - skipped", s)
	}
	if len(res.Skipped) > 0 {
		return fmt.Errorf("%d record(s) skipped", len(res.Skipped))
	}
	return nil
}

// parseRetention accepts Go durations plus a "d" suffix for days; empty means 365d.
func parseRetention(v string) (time.Duration, error) {
	if v == "" {
		return 365 * 24 * time.Hour, nil
	}
	if days, ok := strings.CutSuffix(v, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n < 1 {
			return 0, fmt.Errorf("SOFT_DELETE_RETENTION %q is not a valid number of days", v)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < time.Hour {
		return 0, fmt.Errorf("SOFT_DELETE_RETENTION %q must be like 365d or 720h", v)
	}
	return d, nil
}
