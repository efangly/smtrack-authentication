package legacyimport

import (
	"context"
	"database/sql"
	"fmt"
)

// Verify compares the old database with the new one and reports every difference. It is the
// cutover precondition: an empty Problems list means the new database holds all of the old.
func Verify(ctx context.Context, old, dst *sql.DB) (*Report, error) {
	s, problems, err := readLegacy(ctx, old)
	if err != nil {
		return nil, err
	}
	rep := &Report{Problems: problems, Tenants: len(s.Tenants), Departments: len(s.Departments), Users: len(s.Users), Memberships: len(s.Users), AuditLogs: len(s.Audit)}

	type tRow struct {
		name, kind     string
		legacy, hidden bool
		seq            int64
	}
	gotT := map[string]tRow{}
	if err := scan(ctx, dst, `SELECT id, name, kind, legacy, hidden, seq FROM tenants`, func(r *sql.Rows) error {
		var id string
		var t tRow
		if err := r.Scan(&id, &t.name, &t.kind, &t.legacy, &t.hidden, &t.seq); err != nil {
			return err
		}
		gotT[id] = t
		return nil
	}); err != nil {
		return nil, err
	}
	for _, t := range s.Tenants {
		g, ok := gotT[t.ID]
		switch {
		case !ok:
			rep.problem("tenant %s missing", t.ID)
		case g.name != t.Name || g.kind != t.Kind || g.legacy != t.Legacy || g.hidden != t.Hidden || g.seq != t.Seq:
			rep.problem("tenant %s differs: have %+v, want name=%q kind=%s legacy=%v hidden=%v seq=%d", t.ID, g, t.Name, t.Kind, t.Legacy, t.Hidden, t.Seq)
		}
	}

	type dRow struct {
		tenant, name string
		legacy       bool
		seq          int64
	}
	gotD := map[string]dRow{}
	if err := scan(ctx, dst, `SELECT id, tenant_id, name, legacy, seq FROM departments`, func(r *sql.Rows) error {
		var id string
		var d dRow
		if err := r.Scan(&id, &d.tenant, &d.name, &d.legacy, &d.seq); err != nil {
			return err
		}
		gotD[id] = d
		return nil
	}); err != nil {
		return nil, err
	}
	for _, d := range s.Departments {
		g, ok := gotD[d.ID]
		switch {
		case !ok:
			rep.problem("department %s missing", d.ID)
		case g.tenant != d.TenantID || g.name != d.Name || g.legacy != d.Legacy || g.seq != d.Seq:
			rep.problem("department %s differs: have %+v, want tenant=%s name=%q legacy=%v seq=%d", d.ID, g, d.TenantID, d.Name, d.Legacy, d.Seq)
		}
	}

	type uRow struct {
		tenant, username, password, role string
		status                           bool
	}
	gotU := map[string]uRow{}
	if err := scan(ctx, dst, `SELECT id, tenant_id, username, password, role, status FROM users`, func(r *sql.Rows) error {
		var id string
		var u uRow
		if err := r.Scan(&id, &u.tenant, &u.username, &u.password, &u.role, &u.status); err != nil {
			return err
		}
		gotU[id] = u
		return nil
	}); err != nil {
		return nil, err
	}
	links := map[string][]string{}
	if err := scan(ctx, dst, `SELECT user_id, department_id FROM user_departments`, func(r *sql.Rows) error {
		var uid, did string
		if err := r.Scan(&uid, &did); err != nil {
			return err
		}
		links[uid] = append(links[uid], did)
		return nil
	}); err != nil {
		return nil, err
	}
	for _, u := range s.Users {
		g, ok := gotU[u.ID]
		if !ok {
			rep.problem("user %s (%s) missing", u.ID, u.Username)
			continue
		}
		if g.tenant != u.TenantID || g.username != u.Username || g.role != u.Role || g.status != u.Status {
			rep.problem("user %s differs: have tenant=%s username=%s role=%s status=%v, want tenant=%s username=%s role=%s status=%v", u.ID, g.tenant, g.username, g.role, g.status, u.TenantID, u.Username, u.Role, u.Status)
		}
		if g.password != u.Password {
			rep.problem("user %s (%s) password hash differs", u.ID, u.Username)
		}
		if l := links[u.ID]; len(l) != 1 || l[0] != u.DepartmentID {
			rep.problem("user %s memberships are %v, want [%s]", u.ID, l, u.DepartmentID)
		}
	}

	gotA := map[string]bool{}
	if err := scan(ctx, dst, `SELECT id FROM audit_logs`, func(r *sql.Rows) error {
		var id string
		if err := r.Scan(&id); err != nil {
			return err
		}
		gotA[id] = true
		return nil
	}); err != nil {
		return nil, err
	}
	missing := 0
	for _, a := range s.Audit {
		if !gotA[a.ID] {
			missing++
		}
	}
	if missing > 0 {
		rep.problem("%d audit logs missing", missing)
	}
	return rep, nil
}

func scan(ctx context.Context, db *sql.DB, query string, each func(*sql.Rows) error) error {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("verify query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := each(rows); err != nil {
			return fmt.Errorf("verify scan: %w", err)
		}
	}
	return rows.Err()
}
