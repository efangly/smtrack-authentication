package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestCreateUserRequest_ToUser(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		req := CreateUserRequest{Username: "admin", Password: "1234", TenantID: "T1"}
		user := req.ToUser()
		assert.Equal(t, "T1", user.TenantID)
		assert.Equal(t, "admin", user.Username)
		assert.Equal(t, "1234", user.Password)
		assert.Equal(t, domain.RoleGuest, user.Role)
		assert.True(t, user.Status)
	})

	t.Run("uses provided values", func(t *testing.T) {
		status := false
		display := "Admin User"
		req := CreateUserRequest{
			ID:            "U1",
			TenantID:      "T1",
			DepartmentIDs: []string{"D1", "D2"},
			Username:      "admin",
			Password:      "1234",
			Status:        &status,
			Role:          domain.RoleAdmin,
			Display:       &display,
		}
		user := req.ToUser()
		assert.Equal(t, "U1", user.ID)
		assert.Equal(t, []string{"D1", "D2"}, user.DepartmentIDs)
		assert.Equal(t, domain.RoleAdmin, user.Role)
		assert.False(t, user.Status)
		assert.Equal(t, &display, user.Display)
	})
}

func TestUpdateUserRequest_ToPatch(t *testing.T) {
	assert.Equal(t, domain.UserPatch{}, (&UpdateUserRequest{}).ToPatch())

	username := "newuser"
	status := true
	role := domain.RoleUser
	patch := (&UpdateUserRequest{Username: &username, Status: &status, Role: &role}).ToPatch()
	assert.Equal(t, domain.UserPatch{Username: &username, Status: &status, Role: &role}, patch)

	// An empty list is a request to clear the departments; absent leaves them alone.
	var req UpdateUserRequest
	require.NoError(t, json.Unmarshal([]byte(`{"departmentIds":[]}`), &req))
	require.NotNil(t, req.ToPatch().DepartmentIDs)
	assert.Empty(t, *req.ToPatch().DepartmentIDs)
	require.NoError(t, json.Unmarshal([]byte(`{}`), &req))
}

func TestCreateTenantRequest_ToTenant(t *testing.T) {
	req := CreateTenantRequest{Name: "Plant", Kind: domain.TenantKindFactory, Legacy: true, ContactName: ptr("Bob")}
	tenant := req.ToTenant()
	assert.Equal(t, "Plant", tenant.Name)
	assert.Equal(t, domain.TenantKindFactory, tenant.Kind)
	assert.True(t, tenant.Legacy)
	assert.Equal(t, "Bob", *tenant.ContactName)

	// The service, not the DTO, picks the default kind.
	assert.Empty(t, (&CreateTenantRequest{Name: "x"}).ToTenant().Kind)
}

func TestUpdateTenantRequest_ToPatch(t *testing.T) {
	name := "Updated"
	assert.Equal(t, domain.TenantPatch{Name: &name}, (&UpdateTenantRequest{Name: &name}).ToPatch())
}

func TestCreateDepartmentRequest_ToDepartment(t *testing.T) {
	d := (&CreateDepartmentRequest{Name: "Line 1", TenantID: "T1", Legacy: true}).ToDepartment()
	assert.Equal(t, "Line 1", d.Name)
	assert.Equal(t, "T1", d.TenantID)
	assert.True(t, d.Legacy)
}

func TestUpdateDepartmentRequest_ToPatch(t *testing.T) {
	name := "Updated"
	assert.Equal(t, domain.DepartmentPatch{Name: &name}, (&UpdateDepartmentRequest{Name: &name}).ToPatch())

	// A department cannot be moved to another tenant through the API.
	var req UpdateDepartmentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"tenantId":"T9","name":"n"}`), &req))
	assert.Equal(t, domain.DepartmentPatch{Name: ptr("n")}, req.ToPatch())
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

func TestCreateUserRequest_ToPublicUser_ForcesGuestRole(t *testing.T) {
	status := true
	createBy := "attacker"
	req := &CreateUserRequest{
		ID:            "chosen-id",
		TenantID:      "T1",
		DepartmentIDs: []string{"D1"},
		Username:      "attacker",
		Password:      "pass1234",
		Role:          domain.RoleSuper,
		Status:        &status,
		CreateBy:      &createBy,
	}

	user := req.ToPublicUser()

	assert.Equal(t, domain.RoleGuest, user.Role, "role must never come from the body")
	assert.Empty(t, user.ID, "id must be generated server side")
	assert.Nil(t, user.CreateBy)
	assert.Empty(t, user.DepartmentIDs, "departments are assigned later by an admin of the tenant")
	assert.True(t, user.Status)
	assert.Equal(t, "T1", user.TenantID)
}

func TestUpdateUserRequest_HasNoPassword(t *testing.T) {
	// Password changes go through /auth/reset; a body field must not reach the patch.
	var req UpdateUserRequest
	require.NoError(t, json.Unmarshal([]byte(`{"username":"bob","password":"x"}`), &req))
	patch := req.ToPatch()
	assert.Nil(t, patch.Password)
	assert.Equal(t, "bob", *patch.Username)
}

// pic is set only by uploading an image. A URL in the body is ignored, so a
// user cannot point their record at someone else's file (which the next upload
// would then delete).
func TestPictureIsNotSettableFromBody(t *testing.T) {
	var upd UpdateUserRequest
	require.NoError(t, json.Unmarshal([]byte(`{"pic":"http://img/user/victim.png"}`), &upd))
	assert.Nil(t, upd.ToPatch().Pic)

	var create CreateUserRequest
	require.NoError(t, json.Unmarshal([]byte(`{"tenantId":"T1","username":"u","password":"pass","pic":"http://img/user/victim.png"}`), &create))
	assert.Nil(t, create.ToUser().Pic)
	assert.Nil(t, create.ToPublicUser().Pic)

	var tenUpd UpdateTenantRequest
	require.NoError(t, json.Unmarshal([]byte(`{"pic":"http://img/tenant/victim.png"}`), &tenUpd))
	assert.Nil(t, tenUpd.ToPatch().Pic)

	var tenCreate CreateTenantRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"T","pic":"http://img/tenant/victim.png"}`), &tenCreate))
	assert.Nil(t, tenCreate.ToTenant().Pic)
}

// --- responses ---

func TestFromUser_NeverCarriesThePasswordHash(t *testing.T) {
	user := &domain.User{ID: "U1", Username: "u", Password: "$2a$10$secret-hash"}
	raw, err := json.Marshal(FromUser(user))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "secret-hash")
	assert.NotContains(t, string(raw), "password")
}

func TestFromUser_ListsDepartmentsAndTenantLabels(t *testing.T) {
	user := &domain.User{
		ID: "U1", TenantID: "T1",
		Tenant:      &domain.Tenant{ID: "T1", Name: "Plant", Kind: domain.TenantKindFactory, Labels: domain.Labels{"department": {"en": "Line"}}},
		Departments: []domain.Department{{ID: "D1", Name: "A"}, {ID: "D2", Name: "B"}},
	}
	r := FromUser(user)
	assert.Equal(t, []string{"D1", "D2"}, r.DepartmentIDs)
	require.NotNil(t, r.Tenant)
	assert.Equal(t, "Line", r.Tenant.Labels["department"]["en"], "the tenant's own label")
	assert.Equal(t, "Factory", r.Tenant.Labels["tenant"]["en"], "the default for its kind")

	// A user without departments serializes an empty list, not null.
	raw, err := json.Marshal(FromUser(&domain.User{ID: "U2"}))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"departmentIds":[]`)
	assert.Contains(t, string(raw), `"departments":[]`)
}

func TestFromTenant_DeletedAtOnlyWhenDeleted(t *testing.T) {
	raw, err := json.Marshal(FromTenant(&domain.Tenant{ID: "T1"}))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "deletedAt")

	now := time.Now()
	raw, err = json.Marshal(FromTenant(&domain.Tenant{ID: "T1", DeletedAt: &now}))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "deletedAt")
}
