package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tng-coop/auth-service/internal/core/domain"
)

func TestCreateUserRequest_ToUser(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		req := CreateUserRequest{Username: "admin", Password: "1234", WardID: "W1"}
		user := req.ToUser()
		assert.Equal(t, "W1", user.WardID)
		assert.Equal(t, "admin", user.Username)
		assert.Equal(t, "1234", user.Password)
		assert.Equal(t, domain.RoleGuest, user.Role)
		assert.True(t, user.Status)
	})

	t.Run("uses provided values", func(t *testing.T) {
		status := false
		display := "Admin User"
		req := CreateUserRequest{
			ID:       "U1",
			WardID:   "W1",
			Username: "admin",
			Password: "1234",
			Status:   &status,
			Role:     domain.RoleAdmin,
			Display:  &display,
		}
		user := req.ToUser()
		assert.Equal(t, "U1", user.ID)
		assert.Equal(t, domain.RoleAdmin, user.Role)
		assert.False(t, user.Status)
		assert.Equal(t, &display, user.Display)
	})
}

func TestUpdateUserRequest_ToMap(t *testing.T) {
	t.Run("empty request yields empty map", func(t *testing.T) {
		req := UpdateUserRequest{}
		m := req.ToMap()
		assert.Empty(t, m)
	})

	t.Run("only set fields appear in map", func(t *testing.T) {
		username := "newuser"
		status := true
		role := domain.RoleUser
		req := UpdateUserRequest{
			Username: &username,
			Status:   &status,
			Role:     &role,
		}
		m := req.ToMap()
		assert.Len(t, m, 3)
		assert.Equal(t, "newuser", m["username"])
		assert.Equal(t, true, m["status"])
		assert.Equal(t, "USER", m["role"])
	})
}

func TestCreateHospitalRequest_ToHospital(t *testing.T) {
	t.Run("defaults hosType to HOSPITAL", func(t *testing.T) {
		req := CreateHospitalRequest{HosName: "Hospital A"}
		h := req.ToHospital()
		assert.Equal(t, domain.HosTypeHospital, h.HosType)
		assert.Equal(t, "Hospital A", h.HosName)
	})

	t.Run("uses provided hosType", func(t *testing.T) {
		req := CreateHospitalRequest{HosName: "Lab X", HosType: domain.HosTypeLab}
		h := req.ToHospital()
		assert.Equal(t, domain.HosTypeLab, h.HosType)
	})
}

func TestUpdateHospitalRequest_ToMap(t *testing.T) {
	name := "Updated Hospital"
	req := UpdateHospitalRequest{HosName: &name}
	m := req.ToMap()
	assert.Len(t, m, 1)
	assert.Equal(t, "Updated Hospital", m["hosName"])
}

func TestCreateWardRequest_ToWard(t *testing.T) {
	t.Run("defaults type to NEW", func(t *testing.T) {
		req := CreateWardRequest{WardName: "Ward 1", HosID: "H1"}
		w := req.ToWard()
		assert.Equal(t, "Ward 1", w.WardName)
		assert.Equal(t, "H1", w.HosID)
		assert.NotNil(t, w.Type)
		assert.Equal(t, domain.WardTypeNew, *w.Type)
	})

	t.Run("uses provided type", func(t *testing.T) {
		wt := domain.WardTypeLegacy
		req := CreateWardRequest{WardName: "Ward 2", HosID: "H1", Type: &wt}
		w := req.ToWard()
		assert.Equal(t, domain.WardTypeLegacy, *w.Type)
	})

	t.Run("sets WardSeq when provided", func(t *testing.T) {
		seq := 5
		req := CreateWardRequest{WardName: "Ward 3", HosID: "H1", WardSeq: &seq}
		w := req.ToWard()
		assert.Equal(t, 5, w.WardSeq)
	})
}

func TestUpdateWardRequest_ToMap(t *testing.T) {
	name := "Updated Ward"
	hosID := "H2"
	req := UpdateWardRequest{WardName: &name, HosID: &hosID}
	m := req.ToMap()
	assert.Equal(t, "Updated Ward", m["wardName"])
	assert.Equal(t, "H2", m["hosId"])
	assert.Len(t, m, 2)
}

func TestSuccessResponse(t *testing.T) {
	resp := SuccessResponse("hello")
	assert.True(t, resp.Success)
	assert.Equal(t, "Success", resp.Message)
	assert.Equal(t, "hello", resp.Data)
}

func TestFailResponse(t *testing.T) {
	resp := FailResponse("bad request")
	assert.False(t, resp.Success)
	assert.Equal(t, "bad request", resp.Message)
	assert.Nil(t, resp.Data)
}
