//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestRegister_ForcesGuestAndIgnoresPrivilegedFields verifies ToPublicUser is
// used on the public register route: an attacker-supplied role/status/id and
// any departments in the body must never be honored.
func TestRegister_ForcesGuestAndIgnoresPrivilegedFields(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	deptID := createDepartment(t, admin, tenantID)

	username := e2eName("user")
	resp, env := doJSON(t, http.MethodPost, "/register", map[string]any{
		"id":            "attacker-chosen-id",
		"tenantId":      tenantID,
		"departmentIds": []string{deptID},
		"username":      username,
		"password":      testPassword,
		"role":          "SUPER",
		"status":        false,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: status %d, message %q", resp.StatusCode, env.Message)
	}

	var created userView
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	t.Cleanup(func() {
		doJSON(t, http.MethodDelete, "/user/"+created.ID, nil, admin)
	})

	if created.ID == "attacker-chosen-id" {
		t.Error("server accepted a client-supplied id on the public register route")
	}
	if created.Role != "GUEST" {
		t.Errorf("role = %q, want GUEST (ToPublicUser must force it)", created.Role)
	}
	if !created.Status {
		t.Error("status = false, want true (ToPublicUser must force it)")
	}
	if created.TenantID != tenantID {
		t.Errorf("tenantId = %q, want %q", created.TenantID, tenantID)
	}
	if len(created.DepartmentIDs) != 0 {
		t.Errorf("departmentIds = %v, want none (a self-registered user is placed by an admin)", created.DepartmentIDs)
	}
}

// TestRegister_HiddenOrUnknownTenant_Refused verifies the public route cannot
// be used to join a hidden tenant, and that the answer does not tell a hidden
// tenant from one that does not exist.
func TestRegister_HiddenOrUnknownTenant_Refused(t *testing.T) {
	admin := adminToken(t)
	hidden := createTenantWith(t, admin, map[string]any{"name": e2eName("hidden"), "hidden": true})

	hiddenResp, hiddenEnv := doJSON(t, http.MethodPost, "/register", map[string]any{
		"tenantId": hidden, "username": e2eName("user"), "password": testPassword,
	}, "")
	unknownResp, unknownEnv := doJSON(t, http.MethodPost, "/register", map[string]any{
		"tenantId": e2eName("no-such-tenant"), "username": e2eName("user"), "password": testPassword,
	}, "")

	if hiddenResp.StatusCode < 400 || unknownResp.StatusCode < 400 {
		t.Fatalf("register into hidden/unknown tenant succeeded: hidden=%d unknown=%d", hiddenResp.StatusCode, unknownResp.StatusCode)
	}
	if hiddenResp.StatusCode != unknownResp.StatusCode || hiddenEnv.Message != unknownEnv.Message {
		t.Errorf("hidden tenant (%d %q) is distinguishable from an unknown one (%d %q)",
			hiddenResp.StatusCode, hiddenEnv.Message, unknownResp.StatusCode, unknownEnv.Message)
	}
}

// TestRegister_DuplicateUsername_ReturnsDomainMessage checks that a plain
// domain error (unique-constraint style rejection) is passed through
// verbatim, not replaced by a generic 500 (serviceError only masks errors
// that wrap an infra cause).
func TestRegister_DuplicateUsername_ReturnsDomainMessage(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)

	resp, env := doJSON(t, http.MethodPost, "/register", map[string]any{
		"tenantId": tenantID,
		"username": user.Username,
		"password": "AnotherPass123",
	}, "")

	if resp.StatusCode == http.StatusInternalServerError {
		t.Fatalf("duplicate username leaked as a 500 (message %q); should be a domain 4xx", env.Message)
	}
	if resp.StatusCode < 400 {
		t.Fatalf("registering a duplicate username unexpectedly succeeded: status %d", resp.StatusCode)
	}
	if env.Message == "Internal server error" {
		t.Error("got the generic infra-error message for what should be a domain rejection")
	}
}

// TestRegister_MissingRequiredField_ReturnsValidatorMessage confirms the
// StructValidator wired in main.go is enforced on this route.
func TestRegister_MissingRequiredField_ReturnsValidatorMessage(t *testing.T) {
	resp, env := doJSON(t, http.MethodPost, "/register", map[string]any{
		"tenantId": "some-tenant",
		// username and password omitted
	}, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if env.Message == "" || env.Message == "Internal server error" {
		t.Errorf("message = %q, want a validator-produced message", env.Message)
	}

	// The tenant is required too: there is no longer a way to derive it.
	resp, _ = doJSON(t, http.MethodPost, "/register", map[string]any{
		"username": e2eName("user"), "password": testPassword,
	}, "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("register without tenantId: status = %d, want 400", resp.StatusCode)
	}
}
