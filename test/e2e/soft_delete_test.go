//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// TestSoftDelete_UserCannotLogIn_UsernameStaysReserved_RestoreBringsItBack
// walks a user through delete and restore.
func TestSoftDelete_UserCannotLogIn_UsernameStaysReserved_RestoreBringsItBack(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	deptID := createDepartment(t, admin, tenantID)
	user := createUser(t, admin, tenantID, deptID)
	login := loginAs(t, user.Username, user.Password)

	if resp, env := doJSON(t, http.MethodDelete, "/user/"+user.ID, nil, admin); resp.StatusCode != http.StatusOK {
		t.Fatalf("delete user: status %d, message %q", resp.StatusCode, env.Message)
	}

	if resp, _ := doJSON(t, http.MethodGet, "/user/"+user.ID, nil, admin); resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET deleted user: status = %d, want 404", resp.StatusCode)
	}
	if resp, _ := doJSON(t, http.MethodPost, "/login", map[string]string{"username": user.Username, "password": user.Password}, ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("login as deleted user: status = %d, want 401", resp.StatusCode)
	}
	if resp, _ := doJSON(t, http.MethodPost, "/refresh", map[string]string{"token": login.RefreshToken}, ""); resp.StatusCode == http.StatusOK {
		t.Error("a deleted user's refresh token still works")
	}
	for _, u := range listUsers(t, admin) {
		if u.ID == user.ID {
			t.Error("a deleted user still appears in the listing")
		}
	}

	// The username cannot be taken over by somebody else.
	resp, env := doJSON(t, http.MethodPost, "/user", map[string]any{
		"tenantId": tenantID, "username": user.Username, "password": testPassword,
	}, admin)
	if resp.StatusCode == http.StatusCreated {
		t.Errorf("a deleted user's username was handed out again")
	} else if env.Message == "" {
		t.Error("expected a rejection message")
	}

	// Restore brings back the account, its departments and the ability to sign in.
	if resp, env := doJSON(t, http.MethodPost, "/user/"+user.ID+"/restore", nil, admin); resp.StatusCode != http.StatusOK {
		t.Fatalf("restore user: status %d, message %q", resp.StatusCode, env.Message)
	}
	restored := getUser(t, admin, user.ID)
	if !containsID(restored.DepartmentIDs, deptID) {
		t.Errorf("restored user lost its department link: %v", restored.DepartmentIDs)
	}
	loginAs(t, user.Username, user.Password)
}

// TestSoftDelete_TenantMustBeEmpty_ThenRestorable covers the tenant rules.
func TestSoftDelete_TenantMustBeEmpty_ThenRestorable(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "LAB")
	deptID := createDepartment(t, admin, tenantID)
	user := createUser(t, admin, tenantID, deptID)

	if resp, _ := doJSON(t, http.MethodDelete, "/tenant/"+tenantID, nil, admin); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("delete tenant with users and departments: status = %d, want 400", resp.StatusCode)
	}

	doJSON(t, http.MethodDelete, "/user/"+user.ID, nil, admin)
	if resp, _ := doJSON(t, http.MethodDelete, "/tenant/"+tenantID, nil, admin); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("delete tenant that still has a department: status = %d, want 400", resp.StatusCode)
	}
	doJSON(t, http.MethodDelete, "/department/"+deptID, nil, admin)

	if resp, env := doJSON(t, http.MethodDelete, "/tenant/"+tenantID, nil, admin); resp.StatusCode != http.StatusOK {
		t.Fatalf("delete empty tenant: status %d, message %q", resp.StatusCode, env.Message)
	}
	if resp, _ := doJSON(t, http.MethodGet, "/tenant/"+tenantID, nil, admin); resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET deleted tenant: status = %d, want 404", resp.StatusCode)
	}
	if resp, env := doJSON(t, http.MethodPost, "/tenant/"+tenantID+"/restore", nil, admin); resp.StatusCode != http.StatusOK {
		t.Fatalf("restore tenant: status %d, message %q", resp.StatusCode, env.Message)
	}
	getTenant(t, admin, tenantID)
}

// TestSoftDelete_DepartmentNameFreedUntilRestore verifies a deleted
// department's name can be reused, and that restoring it then re-checks the name.
func TestSoftDelete_DepartmentNameFreedUntilRestore(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "FACTORY")
	name := e2eName("line")

	create := func() (*http.Response, envelope) {
		return doJSON(t, http.MethodPost, "/department", map[string]any{"name": name, "tenantId": tenantID}, admin)
	}
	first, env := create()
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("create department: status %d, message %q", first.StatusCode, env.Message)
	}
	firstID := idOf(t, env)
	t.Cleanup(func() { doJSON(t, http.MethodDelete, "/department/"+firstID, nil, admin) })

	if resp, _ := create(); resp.StatusCode == http.StatusCreated {
		t.Fatal("two live departments of one tenant share a name")
	}

	doJSON(t, http.MethodDelete, "/department/"+firstID, nil, admin)
	second, env := create()
	if second.StatusCode != http.StatusCreated {
		t.Fatalf("reuse the name after delete: status %d, message %q", second.StatusCode, env.Message)
	}
	secondID := idOf(t, env)
	t.Cleanup(func() { doJSON(t, http.MethodDelete, "/department/"+secondID, nil, admin) })

	if resp, _ := doJSON(t, http.MethodPost, "/department/"+firstID+"/restore", nil, admin); resp.StatusCode == http.StatusOK {
		t.Error("restored a department whose name is now taken by a live one")
	}
}
