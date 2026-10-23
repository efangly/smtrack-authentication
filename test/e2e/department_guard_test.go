//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// TestDepartmentGuard_GuestCannotMutateDepartment verifies a GUEST — the role
// anyone can obtain through public registration — can neither rename nor
// delete nor restore a department, nor touch its tenant.
func TestDepartmentGuard_GuestCannotMutateDepartment(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	deptID := createDepartment(t, admin, tenantID)
	guest := createUser(t, admin, tenantID, deptID)
	login := loginAs(t, guest.Username, guest.Password)

	checks := []struct {
		name, method, path string
		body               any
	}{
		{"PUT department", http.MethodPut, "/department/" + deptID, map[string]any{"name": "hijacked"}},
		{"DELETE department", http.MethodDelete, "/department/" + deptID, nil},
		{"restore department", http.MethodPost, "/department/" + deptID + "/restore", nil},
		{"PUT tenant", http.MethodPut, "/tenant/" + tenantID, map[string]any{"name": "hijacked"}},
		{"DELETE tenant", http.MethodDelete, "/tenant/" + tenantID, nil},
		{"PUT tenant labels", http.MethodPut, "/tenant/" + tenantID + "/labels", map[string]any{"labels": map[string]any{"tenant": map[string]string{"en": "Hijacked"}}}},
	}
	for _, c := range checks {
		if resp, _ := doJSON(t, c.method, c.path, c.body, login.Token); resp.StatusCode != http.StatusForbidden {
			t.Errorf("guest %s: status = %d, want 403", c.name, resp.StatusCode)
		}
	}

	// The department must still exist and be unchanged.
	getResp, getEnv := doJSON(t, http.MethodGet, "/department/"+deptID, nil, admin)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("admin GET department after guest attempts: status %d, message %q", getResp.StatusCode, getEnv.Message)
	}
}
