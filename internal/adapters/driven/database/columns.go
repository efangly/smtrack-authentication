package database

import (
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// The *Columns functions turn a domain patch into the column map GORM
// updates. Only set fields are included, and "updated_at" is always stamped.

func userColumns(p domain.UserPatch) map[string]any {
	cols := map[string]any{"updated_at": time.Now()}
	put(cols, "tenant_id", p.TenantID)
	put(cols, "username", p.Username)
	put(cols, "status", p.Status)
	if p.Role != nil {
		cols["role"] = string(*p.Role)
	}
	put(cols, "display", p.Display)
	put(cols, "pic", p.Pic)
	put(cols, "comment", p.Comment)
	put(cols, "password", p.Password)
	return cols
}

func departmentColumns(p domain.DepartmentPatch) map[string]any {
	cols := map[string]any{"updated_at": time.Now()}
	put(cols, "name", p.Name)
	put(cols, "legacy", p.Legacy)
	return cols
}

func tenantColumns(p domain.TenantPatch) map[string]any {
	cols := map[string]any{"updated_at": time.Now()}
	put(cols, "name", p.Name)
	if p.Kind != nil {
		cols["kind"] = string(*p.Kind)
	}
	put(cols, "legacy", p.Legacy)
	put(cols, "hidden", p.Hidden)
	put(cols, "address", p.Address)
	put(cols, "tel", p.Tel)
	put(cols, "contact_name", p.ContactName)
	put(cols, "contact_tel", p.ContactTel)
	put(cols, "latitude", p.Latitude)
	put(cols, "longitude", p.Longitude)
	put(cols, "pic", p.Pic)
	return cols
}

func put[T any](m map[string]any, column string, v *T) {
	if v != nil {
		m[column] = *v
	}
}
