// Package legacyimport copies the old hospital/ward database into the tenant/department
// one (ADR 0004). It is re-runnable: every write is an upsert keyed by the old ID, and the
// old database is only ever read.
package legacyimport

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// hiddenTenantID is the hospital the old code hardcoded as invisible to non-SUPER roles.
const hiddenTenantID = "HID-DEVELOPMENT"

var validKinds = map[string]bool{"HOSPITAL": true, "LAB": true, "FACTORY": true, "CLINIC": true, "PHARMACY": true, "OTHER": true}

var validRoles = map[string]bool{"SUPER": true, "SERVICE": true, "ADMIN": true, "LEGACY_ADMIN": true, "USER": true, "LEGACY_USER": true, "GUEST": true}

type tenant struct {
	ID, Name, Kind                                       string
	Legacy, Hidden                                       bool
	Address, Tel, ContactName, ContactTel, Lat, Lng, Pic sql.NullString
	Seq                                                  int64
	CreatedAt, UpdatedAt                                 time.Time
}

type department struct {
	ID, TenantID, Name string
	Legacy             bool
	Seq                int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type user struct {
	ID, TenantID, Username, Password, Role string
	Status                                 bool
	Display, Pic, Comment, CreatedBy       sql.NullString
	DepartmentID                           string
	CreatedAt, UpdatedAt                   time.Time
}

type auditLog struct {
	ID, Entity, EntityID, Action string
	ActorID, ActorName, Changes  sql.NullString
	CreatedAt                    time.Time
}

// snapshot is everything read from the old database, already mapped to the new shape.
type snapshot struct {
	Tenants     []tenant
	Departments []department
	Users       []user
	Audit       []auditLog
}

// Report is what Import did, or what Verify found wrong.
type Report struct {
	Tenants, Departments, Users, Memberships, AuditLogs int
	Problems                                            []string
}

func (r *Report) problem(format string, a ...any) {
	r.Problems = append(r.Problems, fmt.Sprintf(format, a...))
}

// readLegacy loads the old database in one read-only transaction so the copy is consistent.
func readLegacy(ctx context.Context, old *sql.DB) (*snapshot, []string, error) {
	tx, err := old.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, nil, fmt.Errorf("begin legacy read: %w", err)
	}
	defer tx.Rollback()

	s := &snapshot{}
	var problems []string

	rows, err := tx.QueryContext(ctx, `SELECT "id","hosName","hosSeq","hosAddress","hosTel","userContact","userTel","hosLatitude","hosLongitude","hosPic","hosType",COALESCE("createAt", now()),COALESCE("updateAt","createAt", now()) FROM "Hospitals" ORDER BY "hosSeq"`)
	if err != nil {
		return nil, nil, fmt.Errorf("read hospitals: %w", err)
	}
	for rows.Next() {
		var t tenant
		var hosType sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Seq, &t.Address, &t.Tel, &t.ContactName, &t.ContactTel, &t.Lat, &t.Lng, &t.Pic, &hosType, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan hospital: %w", err)
		}
		t.Kind = strings.ToUpper(hosType.String)
		switch {
		case t.Kind == "" || t.Kind == "LEGACY":
			t.Legacy = t.Kind == "LEGACY"
			t.Kind = "HOSPITAL"
		case !validKinds[t.Kind]:
			problems = append(problems, fmt.Sprintf("hospital %s has unknown hosType %q", t.ID, hosType.String))
		}
		t.Hidden = t.ID == hiddenTenantID
		s.Tenants = append(s.Tenants, t)
	}
	if err := closeRows(rows); err != nil {
		return nil, nil, fmt.Errorf("read hospitals: %w", err)
	}

	rows, err = tx.QueryContext(ctx, `SELECT "id","wardName","wardSeq","type","hosId",COALESCE("createAt", now()),COALESCE("updateAt","createAt", now()) FROM "Wards" ORDER BY "wardSeq"`)
	if err != nil {
		return nil, nil, fmt.Errorf("read wards: %w", err)
	}
	for rows.Next() {
		var d department
		var typ sql.NullString
		if err := rows.Scan(&d.ID, &d.Name, &d.Seq, &typ, &d.TenantID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan ward: %w", err)
		}
		d.Legacy = strings.EqualFold(typ.String, "LEGACY")
		s.Departments = append(s.Departments, d)
	}
	if err := closeRows(rows); err != nil {
		return nil, nil, fmt.Errorf("read wards: %w", err)
	}

	rows, err = tx.QueryContext(ctx, `SELECT "id","wardId","username","password","status","role","display","pic","comment","createBy",COALESCE("createAt", now()),COALESCE("updateAt","createAt", now()) FROM "Users" ORDER BY "createAt","id"`)
	if err != nil {
		return nil, nil, fmt.Errorf("read users: %w", err)
	}
	for rows.Next() {
		var u user
		if err := rows.Scan(&u.ID, &u.DepartmentID, &u.Username, &u.Password, &u.Status, &u.Role, &u.Display, &u.Pic, &u.Comment, &u.CreatedBy, &u.CreatedAt, &u.UpdatedAt); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan user: %w", err)
		}
		s.Users = append(s.Users, u)
	}
	if err := closeRows(rows); err != nil {
		return nil, nil, fmt.Errorf("read users: %w", err)
	}

	rows, err = tx.QueryContext(ctx, `SELECT "id","entity","entityId","action","actorId","actorName","changes",COALESCE("createAt", now()) FROM "AuditLogs" ORDER BY "createAt","id"`)
	if err != nil {
		return nil, nil, fmt.Errorf("read audit logs: %w", err)
	}
	for rows.Next() {
		var a auditLog
		if err := rows.Scan(&a.ID, &a.Entity, &a.EntityID, &a.Action, &a.ActorID, &a.ActorName, &a.Changes, &a.CreatedAt); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan audit log: %w", err)
		}
		s.Audit = append(s.Audit, a)
	}
	if err := closeRows(rows); err != nil {
		return nil, nil, fmt.Errorf("read audit logs: %w", err)
	}

	// Resolve each user's tenant through its ward, and check what the new schema will enforce.
	tenantIDs := map[string]bool{}
	for _, t := range s.Tenants {
		tenantIDs[t.ID] = true
	}
	deptTenant := map[string]string{}
	seenName := map[string][]string{}
	for _, d := range s.Departments {
		if !tenantIDs[d.TenantID] {
			problems = append(problems, fmt.Sprintf("ward %s points at missing hospital %q", d.ID, d.TenantID))
		}
		deptTenant[d.ID] = d.TenantID
		key := d.TenantID + "\x00" + d.Name
		seenName[key] = append(seenName[key], d.ID)
	}
	for key, ids := range seenName {
		if len(ids) > 1 {
			problems = append(problems, fmt.Sprintf("hospital %s has %d wards named %q (%s); rename them in the old database", strings.SplitN(key, "\x00", 2)[0], len(ids), strings.SplitN(key, "\x00", 2)[1], strings.Join(ids, ", ")))
		}
	}
	for i := range s.Users {
		u := &s.Users[i]
		tid, ok := deptTenant[u.DepartmentID]
		if !ok {
			problems = append(problems, fmt.Sprintf("user %s (%s) points at missing ward %q", u.ID, u.Username, u.DepartmentID))
		}
		u.TenantID = tid
		if !validRoles[u.Role] {
			problems = append(problems, fmt.Sprintf("user %s (%s) has unknown role %q", u.ID, u.Username, u.Role))
		}
	}
	sort.Strings(problems)
	return s, problems, nil
}

func closeRows(rows *sql.Rows) error {
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	return rows.Close()
}
