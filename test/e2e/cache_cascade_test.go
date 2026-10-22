//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestCacheCascade_DepartmentRename_InvalidatesUserListing is the regression
// check CLAUDE.md calls out: anything that changes a tenant or department must
// also invalidate `user`, because user listings embed their departments.
func TestCacheCascade_DepartmentRename_InvalidatesUserListing(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	deptID := createDepartment(t, admin, tenantID)
	user := createUser(t, admin, tenantID, deptID)

	findInListing := func() userView {
		for _, u := range listUsers(t, admin) {
			if u.ID == user.ID {
				return u
			}
		}
		t.Fatalf("user %s not found in listing", user.ID)
		return userView{}
	}

	before := findInListing() // populates the user-list cache
	if len(before.Departments) != 1 {
		t.Fatalf("expected the listing to embed the user's department, got %+v", before.Departments)
	}

	newName := e2eName("renamed-dept")
	updResp, updEnv := doJSON(t, http.MethodPut, "/department/"+deptID, map[string]any{"name": newName}, admin)
	if updResp.StatusCode != http.StatusOK {
		t.Fatalf("rename department: status %d, message %q", updResp.StatusCode, updEnv.Message)
	}

	after := findInListing()
	if len(after.Departments) != 1 || after.Departments[0].Name != newName {
		t.Errorf("user listing shows departments %+v, want name %q (stale user cache after department update)",
			after.Departments, newName)
	}
}

// TestCacheCascade_TenantRename_InvalidatesUserDetail checks the same cascade
// for the single-user cache: GET /auth/user/:id embeds the tenant.
func TestCacheCascade_TenantRename_InvalidatesUserDetail(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)

	before := getUser(t, admin, user.ID) // populates the user:<id> cache
	if before.Tenant == nil {
		t.Fatal("expected user detail to embed the tenant")
	}

	newName := e2eName("renamed-tenant")
	updResp, updEnv := doJSON(t, http.MethodPut, "/tenant/"+tenantID, map[string]any{"name": newName}, admin)
	if updResp.StatusCode != http.StatusOK {
		t.Fatalf("rename tenant: status %d, message %q", updResp.StatusCode, updEnv.Message)
	}

	after := getUser(t, admin, user.ID)
	if after.Tenant == nil || after.Tenant.Name != newName {
		t.Errorf("user detail shows tenant %+v, want name %q (stale user cache after tenant update)", after.Tenant, newName)
	}
}

// TestCacheCascade_LabelChange_ReachesTenantRead verifies a label edit shows up
// in the next read of the tenant.
func TestCacheCascade_LabelChange_ReachesTenantRead(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	_ = createUser(t, admin, tenantID)

	read := func() string {
		resp, env := doJSON(t, http.MethodGet, "/tenant/"+tenantID, nil, admin)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("get tenant: status %d, message %q", resp.StatusCode, env.Message)
		}
		var tn struct {
			Labels map[string]map[string]string `json:"labels"`
		}
		if err := json.Unmarshal(env.Data, &tn); err != nil {
			t.Fatalf("decode tenant: %v", err)
		}
		return tn.Labels["department"]["en"]
	}
	if got := read(); got != "Ward" {
		t.Fatalf("default department label for a hospital = %q, want Ward", got)
	}
	resp, env := doJSON(t, http.MethodPut, "/tenant/"+tenantID+"/labels", map[string]any{
		"labels": map[string]any{"department": map[string]string{"en": "Unit"}},
	}, admin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set labels: status %d, message %q", resp.StatusCode, env.Message)
	}
	if got := read(); got != "Unit" {
		t.Errorf("department label after edit = %q, want Unit (stale tenant cache)", got)
	}
}
