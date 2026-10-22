package services

import "github.com/efangly/smtrack-authentication/internal/core/domain"

// The *PatchChanges helpers turn a partial update into the "changes" an audit
// entry records: only the fields that were set, under the same names the API
// uses. A password is never included, not even hashed. The *Snapshot helpers
// do the same for a whole record on create, delete and restore.

func userPatchChanges(p domain.UserPatch) map[string]any {
	changes := map[string]any{}
	set(changes, "tenantId", p.TenantID)
	set(changes, "departmentIds", p.DepartmentIDs)
	set(changes, "username", p.Username)
	set(changes, "status", p.Status)
	set(changes, "role", p.Role)
	set(changes, "display", p.Display)
	set(changes, "pic", p.Pic)
	set(changes, "comment", p.Comment)
	return changes
}

func departmentPatchChanges(p domain.DepartmentPatch) map[string]any {
	changes := map[string]any{}
	set(changes, "name", p.Name)
	set(changes, "legacy", p.Legacy)
	return changes
}

func tenantPatchChanges(p domain.TenantPatch) map[string]any {
	changes := map[string]any{}
	set(changes, "name", p.Name)
	set(changes, "kind", p.Kind)
	set(changes, "legacy", p.Legacy)
	set(changes, "hidden", p.Hidden)
	set(changes, "address", p.Address)
	set(changes, "tel", p.Tel)
	set(changes, "contactName", p.ContactName)
	set(changes, "contactTel", p.ContactTel)
	set(changes, "latitude", p.Latitude)
	set(changes, "longitude", p.Longitude)
	set(changes, "pic", p.Pic)
	return changes
}

func userSnapshot(u *domain.User) map[string]any {
	return map[string]any{
		"id":            u.ID,
		"tenantId":      u.TenantID,
		"departmentIds": u.LinkedDepartmentIDs(),
		"username":      u.Username,
		"status":        u.Status,
		"role":          u.Role,
		"display":       u.Display,
		"comment":       u.Comment,
	}
}

func departmentSnapshot(d *domain.Department) map[string]any {
	return map[string]any{
		"id":       d.ID,
		"tenantId": d.TenantID,
		"name":     d.Name,
		"legacy":   d.Legacy,
	}
}

func tenantSnapshot(t *domain.Tenant) map[string]any {
	return map[string]any{
		"id":          t.ID,
		"name":        t.Name,
		"kind":        t.Kind,
		"legacy":      t.Legacy,
		"hidden":      t.Hidden,
		"address":     t.Address,
		"tel":         t.Tel,
		"contactName": t.ContactName,
		"contactTel":  t.ContactTel,
	}
}

func set[T any](m map[string]any, key string, v *T) {
	if v != nil {
		m[key] = *v
	}
}
