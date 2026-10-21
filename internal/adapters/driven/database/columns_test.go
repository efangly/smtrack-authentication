package database

import (
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

// withoutStamp checks updateAt is a fresh time and removes it for comparison.
func withoutStamp(t *testing.T, cols map[string]any) map[string]any {
	t.Helper()
	stamp, ok := cols["updateAt"].(time.Time)
	require.True(t, ok, "updateAt must always be set")
	assert.WithinDuration(t, time.Now(), stamp, time.Minute)
	delete(cols, "updateAt")
	return cols
}

func TestUserColumns(t *testing.T) {
	assert.Empty(t, withoutStamp(t, userColumns(domain.UserPatch{})))

	cols := withoutStamp(t, userColumns(domain.UserPatch{
		WardID: ptr("W1"), Username: ptr("u"), Status: ptr(false), Role: ptr(domain.RoleAdmin),
		Display: ptr("d"), Pic: ptr("p"), Comment: ptr("c"), Password: ptr("hash"),
	}))
	assert.Equal(t, map[string]any{
		"wardId": "W1", "username": "u", "status": false, "role": "ADMIN",
		"display": "d", "pic": "p", "comment": "c", "password": "hash",
	}, cols)
}

func TestWardColumns(t *testing.T) {
	cols := withoutStamp(t, wardColumns(domain.WardPatch{
		WardName: ptr("A"), WardSeq: ptr(2), Type: ptr(domain.WardTypeLegacy), HosID: ptr("H1"),
	}))
	assert.Equal(t, map[string]any{"wardName": "A", "wardSeq": 2, "type": "LEGACY", "hosId": "H1"}, cols)
}

func TestHospitalColumns(t *testing.T) {
	cols := withoutStamp(t, hospitalColumns(domain.HospitalPatch{
		HosName: ptr("H"), HosAddress: ptr("a"), HosTel: ptr("t"), UserContact: ptr("uc"),
		UserTel: ptr("ut"), HosLatitude: ptr("1"), HosLongitude: ptr("2"), HosPic: ptr("p"),
		HosType: ptr(domain.HosTypeClinic),
	}))
	assert.Equal(t, map[string]any{
		"hosName": "H", "hosAddress": "a", "hosTel": "t", "userContact": "uc", "userTel": "ut",
		"hosLatitude": "1", "hosLongitude": "2", "hosPic": "p", "hosType": "CLINIC",
	}, cols)
}
