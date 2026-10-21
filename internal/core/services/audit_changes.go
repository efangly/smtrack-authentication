package services

import "github.com/efangly/smtrack-authentication/internal/core/domain"

// The *PatchChanges helpers turn a partial update into the "changes" an audit
// entry records: only the fields that were set, under the same names the API
// uses. A password is never included, not even hashed.

func userPatchChanges(p domain.UserPatch) map[string]any {
	changes := map[string]any{}
	set(changes, "wardId", p.WardID)
	set(changes, "username", p.Username)
	set(changes, "status", p.Status)
	set(changes, "role", p.Role)
	set(changes, "display", p.Display)
	set(changes, "pic", p.Pic)
	set(changes, "comment", p.Comment)
	return changes
}

func wardPatchChanges(p domain.WardPatch) map[string]any {
	changes := map[string]any{}
	set(changes, "wardName", p.WardName)
	set(changes, "wardSeq", p.WardSeq)
	set(changes, "type", p.Type)
	set(changes, "hosId", p.HosID)
	return changes
}

func hospitalPatchChanges(p domain.HospitalPatch) map[string]any {
	changes := map[string]any{}
	set(changes, "hosName", p.HosName)
	set(changes, "hosAddress", p.HosAddress)
	set(changes, "hosTel", p.HosTel)
	set(changes, "userContact", p.UserContact)
	set(changes, "userTel", p.UserTel)
	set(changes, "hosLatitude", p.HosLatitude)
	set(changes, "hosLongitude", p.HosLongitude)
	set(changes, "hosPic", p.HosPic)
	set(changes, "hosType", p.HosType)
	return changes
}

func set[T any](m map[string]any, key string, v *T) {
	if v != nil {
		m[key] = *v
	}
}
