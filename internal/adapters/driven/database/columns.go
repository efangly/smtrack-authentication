package database

import (
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// The *Columns functions turn a domain patch into the column map GORM
// updates. Keys are the real (camelCase, Prisma-era) column names; only set
// fields are included, and "updateAt" is always stamped.

func userColumns(p domain.UserPatch) map[string]any {
	cols := map[string]any{"updateAt": time.Now()}
	put(cols, "wardId", p.WardID)
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

func wardColumns(p domain.WardPatch) map[string]any {
	cols := map[string]any{"updateAt": time.Now()}
	put(cols, "wardName", p.WardName)
	put(cols, "wardSeq", p.WardSeq)
	if p.Type != nil {
		cols["type"] = string(*p.Type)
	}
	put(cols, "hosId", p.HosID)
	return cols
}

func hospitalColumns(p domain.HospitalPatch) map[string]any {
	cols := map[string]any{"updateAt": time.Now()}
	put(cols, "hosName", p.HosName)
	put(cols, "hosAddress", p.HosAddress)
	put(cols, "hosTel", p.HosTel)
	put(cols, "userContact", p.UserContact)
	put(cols, "userTel", p.UserTel)
	put(cols, "hosLatitude", p.HosLatitude)
	put(cols, "hosLongitude", p.HosLongitude)
	put(cols, "hosPic", p.HosPic)
	if p.HosType != nil {
		cols["hosType"] = string(*p.HosType)
	}
	return cols
}

func put[T any](m map[string]any, column string, v *T) {
	if v != nil {
		m[column] = *v
	}
}
