// Package purge permanently removes users, departments and tenants that were soft-deleted
// longer ago than the retention period, together with their pictures. Each purge is audited
// (PURGE). It is the only place a row is really deleted.
package purge

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
)

const actorName = "purge"

// Result counts what a Run removed, and what it left for the next run.
type Result struct {
	Users, Departments, Tenants int
	Skipped                     []string
}

// Run purges everything soft-deleted before cutoff. Users go first, then departments, then
// tenants, so a tenant that was emptied by earlier purges can go in the same run. A row
// whose picture cannot be deleted is skipped and retried on the next run; so is a tenant
// that still has rows. With dryRun nothing is changed and the counts are what would go.
func Run(ctx context.Context, db *sql.DB, files ports.FileUploadPort, cutoff time.Time, dryRun bool) (*Result, error) {
	res := &Result{}
	steps := []struct {
		table, entity, pic, tenantCol string
		count                         *int
	}{
		{"users", string(domain.AuditEntityUser), "pic", "tenant_id", &res.Users},
		{"departments", string(domain.AuditEntityDepartment), "", "tenant_id", &res.Departments},
		{"tenants", string(domain.AuditEntityTenant), "pic", "id", &res.Tenants},
	}
	for _, st := range steps {
		picCol := "NULL::text"
		if st.pic != "" {
			picCol = st.pic
		}
		rows, err := db.QueryContext(ctx, fmt.Sprintf(
			`SELECT id, %s, %s FROM %s WHERE deleted_at IS NOT NULL AND deleted_at < $1 ORDER BY deleted_at`, picCol, st.tenantCol, st.table), cutoff)
		if err != nil {
			return res, fmt.Errorf("list %s to purge: %w", st.table, err)
		}
		type item struct {
			id, tenantID string
			pic          sql.NullString
		}
		var items []item
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.id, &it.pic, &it.tenantID); err != nil {
				rows.Close()
				return res, fmt.Errorf("scan %s: %w", st.table, err)
			}
			items = append(items, it)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return res, fmt.Errorf("list %s to purge: %w", st.table, err)
		}
		rows.Close()

		for _, it := range items {
			if dryRun {
				*st.count++
				continue
			}
			if it.pic.Valid && it.pic.String != "" && files != nil {
				if err := files.Delete(map[string]string{"users": "user", "tenants": "tenant"}[st.table], it.pic.String); err != nil {
					res.Skipped = append(res.Skipped, fmt.Sprintf("%s %s: picture not deleted: %v", st.table, it.id, err))
					continue
				}
			}
			if err := purgeOne(ctx, db, st.table, st.entity, it.id, it.tenantID); err != nil {
				res.Skipped = append(res.Skipped, fmt.Sprintf("%s %s: %v", st.table, it.id, err))
				continue
			}
			*st.count++
		}
	}
	return res, nil
}

func purgeOne(ctx context.Context, db *sql.DB, table, entity, id, tenantID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// The deleted_at guard repeats the selection, so a row restored meanwhile is kept.
	r, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id=$1 AND deleted_at IS NOT NULL`, table), id)
	if err != nil {
		return err // e.g. a tenant that still has rows: RESTRICT
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO audit_logs (id, entity, entity_id, action, actor_name, tenant_id, created_at)
VALUES ($1,$2,$3,$4,$5,$6,now())`, uuid.NewString(), entity, id, string(domain.AuditActionPurge), actorName, tenantID); err != nil {
		return err
	}
	return tx.Commit()
}
