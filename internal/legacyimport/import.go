package legacyimport

import (
	"context"
	"database/sql"
	"fmt"
)

// Import copies the old database into the new one in a single transaction. It refuses to
// write anything while the old data has problems the new schema cannot hold (the Report
// lists them). Running it again updates what changed and adds what is new; it never touches
// last_login_at, soft-delete columns, label profiles or audit rows already imported.
func Import(ctx context.Context, old, dst *sql.DB) (*Report, error) {
	s, problems, err := readLegacy(ctx, old)
	if err != nil {
		return nil, err
	}
	rep := &Report{Problems: problems}
	if len(problems) > 0 {
		return rep, nil
	}

	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin import: %w", err)
	}
	defer tx.Rollback()

	for _, t := range s.Tenants {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO tenants (id, seq, name, kind, legacy, hidden, address, tel, contact_name, contact_tel, latitude, longitude, pic, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
ON CONFLICT (id) DO UPDATE SET seq=EXCLUDED.seq, name=EXCLUDED.name, kind=EXCLUDED.kind, legacy=EXCLUDED.legacy,
  hidden=EXCLUDED.hidden, address=EXCLUDED.address, tel=EXCLUDED.tel, contact_name=EXCLUDED.contact_name,
  contact_tel=EXCLUDED.contact_tel, latitude=EXCLUDED.latitude, longitude=EXCLUDED.longitude, pic=EXCLUDED.pic,
  updated_at=EXCLUDED.updated_at`,
			t.ID, t.Seq, t.Name, t.Kind, t.Legacy, t.Hidden, t.Address, t.Tel, t.ContactName, t.ContactTel, t.Lat, t.Lng, t.Pic, t.CreatedAt, t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("import tenant %s: %w", t.ID, err)
		}
		rep.Tenants++
	}
	for _, d := range s.Departments {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO departments (id, seq, tenant_id, name, legacy, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (id) DO UPDATE SET seq=EXCLUDED.seq, tenant_id=EXCLUDED.tenant_id, name=EXCLUDED.name,
  legacy=EXCLUDED.legacy, updated_at=EXCLUDED.updated_at`,
			d.ID, d.Seq, d.TenantID, d.Name, d.Legacy, d.CreatedAt, d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("import department %s: %w", d.ID, err)
		}
		rep.Departments++
	}
	for _, u := range s.Users {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO users (id, tenant_id, username, password, status, role, display, pic, comment, created_by, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (id) DO UPDATE SET tenant_id=EXCLUDED.tenant_id, username=EXCLUDED.username, password=EXCLUDED.password,
  status=EXCLUDED.status, role=EXCLUDED.role, display=EXCLUDED.display, pic=EXCLUDED.pic, comment=EXCLUDED.comment,
  created_by=EXCLUDED.created_by, updated_at=EXCLUDED.updated_at`,
			u.ID, u.TenantID, u.Username, u.Password, u.Status, u.Role, u.Display, u.Pic, u.Comment, u.CreatedBy, u.CreatedAt, u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("import user %s: %w", u.ID, err)
		}
		rep.Users++
		// The old model had exactly one ward per user; keep it as that user's only link.
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_departments WHERE user_id=$1 AND department_id<>$2`, u.ID, u.DepartmentID); err != nil {
			return nil, fmt.Errorf("reset memberships of %s: %w", u.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_departments (user_id, department_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, u.ID, u.DepartmentID); err != nil {
			return nil, fmt.Errorf("link %s: %w", u.ID, err)
		}
		rep.Memberships++
	}
	for _, a := range s.Audit {
		// Old entity names (WARD, HOSPITAL) are kept: they are history, not the new vocabulary.
		if _, err := tx.ExecContext(ctx, `
INSERT INTO audit_logs (id, entity, entity_id, action, actor_id, actor_name, changes, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (id) DO NOTHING`,
			a.ID, a.Entity, a.EntityID, a.Action, a.ActorID, a.ActorName, a.Changes, a.CreatedAt); err != nil {
			return nil, fmt.Errorf("import audit log %s: %w", a.ID, err)
		}
		rep.AuditLogs++
	}

	// Identity sequences must continue after the highest imported seq.
	for _, table := range []string{"tenants", "departments"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(
			`SELECT setval(pg_get_serial_sequence('%[1]s','seq'), COALESCE(MAX(seq),1), MAX(seq) IS NOT NULL) FROM %[1]s`, table)); err != nil {
			return nil, fmt.Errorf("reset %s sequence: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit import: %w", err)
	}
	return rep, nil
}
