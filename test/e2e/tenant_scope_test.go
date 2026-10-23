//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestTenantScope_AdminConfinedToOwnTenant verifies an ADMIN of tenant A can
// neither see nor change anything belonging to tenant B, and cannot read the
// cross-tenant audit log.
func TestTenantScope_AdminConfinedToOwnTenant(t *testing.T) {
	root := adminToken(t)
	tenA := createTenant(t, root, "HOSPITAL")
	deptA := createDepartment(t, root, tenA)
	tenB := createTenant(t, root, "LAB")
	deptB := createDepartment(t, root, tenB)

	admin := createAdmin(t, root, tenA)
	ownUser := createUser(t, root, tenA, deptA)
	otherUser := createUser(t, root, tenB, deptB)
	login := loginAs(t, admin.Username, admin.Password)

	// Own tenant still works.
	if resp, env := doJSON(t, http.MethodGet, "/user/"+ownUser.ID, nil, login.Token); resp.StatusCode != http.StatusOK {
		t.Fatalf("ADMIN GET own-tenant user: status %d, message %q", resp.StatusCode, env.Message)
	}
	if resp, env := doJSON(t, http.MethodGet, "/tenant/"+tenA, nil, login.Token); resp.StatusCode != http.StatusOK {
		t.Fatalf("ADMIN GET own tenant: status %d, message %q", resp.StatusCode, env.Message)
	}
	if resp, env := doJSON(t, http.MethodGet, "/department/"+deptA, nil, login.Token); resp.StatusCode != http.StatusOK {
		t.Fatalf("ADMIN GET own department: status %d, message %q", resp.StatusCode, env.Message)
	}

	// Another tenant reads as not found / is refused.
	checks := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"GET other-tenant user", http.MethodGet, "/user/" + otherUser.ID, nil, http.StatusNotFound},
		{"PUT other-tenant user", http.MethodPut, "/user/" + otherUser.ID, map[string]any{"display": "x"}, http.StatusBadRequest},
		{"DELETE other-tenant user", http.MethodDelete, "/user/" + otherUser.ID, nil, http.StatusBadRequest},
		{"move own user into other tenant", http.MethodPut, "/user/" + ownUser.ID, map[string]any{"tenantId": tenB, "departmentIds": []string{deptB}}, http.StatusBadRequest},
		{"link own user to other tenant's department", http.MethodPut, "/user/" + ownUser.ID, map[string]any{"departmentIds": []string{deptB}}, http.StatusBadRequest},
		{"GET other tenant", http.MethodGet, "/tenant/" + tenB, nil, http.StatusNotFound},
		{"GET other-tenant department", http.MethodGet, "/department/" + deptB, nil, http.StatusNotFound},
		{"rename other tenant's labels", http.MethodPut, "/tenant/" + tenB + "/labels", map[string]any{"labels": map[string]any{"tenant": map[string]string{"en": "Mine"}}}, http.StatusBadRequest},
		{"GET audit log", http.MethodGet, "/audit", nil, http.StatusForbidden},
	}
	for _, c := range checks {
		resp, env := doJSON(t, c.method, c.path, c.body, login.Token)
		if resp.StatusCode != c.want {
			t.Errorf("%s: status = %d (%q), want %d", c.name, resp.StatusCode, env.Message, c.want)
		}
	}

	// Creating a user straight into the other tenant is refused too.
	resp, env := doJSON(t, http.MethodPost, "/user", map[string]any{
		"tenantId": tenB,
		"username": e2eName("crosstenant"),
		"password": testPassword,
		"role":     "USER",
	}, login.Token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("ADMIN create user in other tenant: status = %d (%q), want 400", resp.StatusCode, env.Message)
	}

	// Listings contain no foreign rows.
	for _, u := range listUsers(t, login.Token) {
		if u.TenantID != tenA {
			t.Errorf("ADMIN user listing contains a user of tenant %s", u.TenantID)
		}
	}
	_, listEnv := doJSON(t, http.MethodGet, "/tenant", nil, login.Token)
	var tenants []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(listEnv.Data, &tenants); err != nil {
		t.Fatalf("decode tenant list: %v", err)
	}
	if len(tenants) != 1 || tenants[0].ID != tenA {
		t.Errorf("ADMIN tenant listing = %v, want only %s", tenants, tenA)
	}
}

// TestTenantScope_MemberSeesOnlyLinkedDepartments verifies a plain user sees
// its own tenant and only the departments it is linked to, while the tenant's
// admin sees every department.
func TestTenantScope_MemberSeesOnlyLinkedDepartments(t *testing.T) {
	root := adminToken(t)
	tenantID := createTenant(t, root, "FACTORY")
	line1 := createDepartment(t, root, tenantID)
	line2 := createDepartment(t, root, tenantID)
	member := createUserWithRole(t, root, tenantID, "USER", line1)
	admin := createAdmin(t, root, tenantID)

	memberLogin := loginAs(t, member.Username, member.Password)
	if resp, _ := doJSON(t, http.MethodGet, "/department/"+line1, nil, memberLogin.Token); resp.StatusCode != http.StatusOK {
		t.Errorf("member GET linked department: status = %d, want 200", resp.StatusCode)
	}
	if resp, _ := doJSON(t, http.MethodGet, "/department/"+line2, nil, memberLogin.Token); resp.StatusCode != http.StatusNotFound {
		t.Errorf("member GET unlinked department: status = %d, want 404", resp.StatusCode)
	}

	adminLogin := loginAs(t, admin.Username, admin.Password)
	if resp, _ := doJSON(t, http.MethodGet, "/department/"+line2, nil, adminLogin.Token); resp.StatusCode != http.StatusOK {
		t.Errorf("tenant admin GET any department of its tenant: status = %d, want 200", resp.StatusCode)
	}
}

// TestTenantScope_HiddenTenantVisibleToSuperOnly verifies a hidden tenant is
// left out of every listing except SUPER's, and reads as not found.
func TestTenantScope_HiddenTenantVisibleToSuperOnly(t *testing.T) {
	root := adminToken(t)
	hidden := createTenantWith(t, root, map[string]any{"name": e2eName("hidden"), "hidden": true})
	visible := createTenant(t, root, "HOSPITAL")
	admin := createAdmin(t, root, visible)
	login := loginAs(t, admin.Username, admin.Password)

	if resp, _ := doJSON(t, http.MethodGet, "/tenant/"+hidden, nil, root); resp.StatusCode != http.StatusOK {
		t.Errorf("SUPER GET hidden tenant: status = %d, want 200", resp.StatusCode)
	}
	if resp, _ := doJSON(t, http.MethodGet, "/tenant/"+hidden, nil, login.Token); resp.StatusCode != http.StatusNotFound {
		t.Errorf("ADMIN GET hidden tenant: status = %d, want 404", resp.StatusCode)
	}

	// Only SUPER may create or reveal one: SERVICE is refused.
	service := createUserWithRole(t, root, visible, "SERVICE")
	serviceLogin := loginAs(t, service.Username, service.Password)
	resp, env := doJSON(t, http.MethodPost, "/tenant", map[string]any{"name": e2eName("hidden"), "hidden": true}, serviceLogin.Token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("SERVICE create hidden tenant: status = %d (%q), want 400", resp.StatusCode, env.Message)
	}
}
