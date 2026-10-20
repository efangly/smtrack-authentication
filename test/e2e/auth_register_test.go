//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestRegister_ForcesGuestAndIgnoresPrivilegedFields verifies ToPublicUser is
// used on the public register route: an attacker-supplied role/status/id in
// the body must never be honored.
func TestRegister_ForcesGuestAndIgnoresPrivilegedFields(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)

	username := e2eName("user")
	resp, env := doJSON(t, http.MethodPost, "/register", map[string]any{
		"id":       "attacker-chosen-id",
		"wardId":   wardID,
		"username": username,
		"password": "TestPass123",
		"role":     "SUPER",
		"status":   false,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: status %d, message %q", resp.StatusCode, env.Message)
	}

	var created struct {
		ID     string `json:"id"`
		Role   string `json:"role"`
		Status bool   `json:"status"`
	}
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
}

// TestRegister_DuplicateUsername_ReturnsDomainMessage checks that a plain
// domain error (unique-constraint style rejection) is passed through
// verbatim, not replaced by a generic 500 (serviceError only masks errors
// that wrap an infra cause).
func TestRegister_DuplicateUsername_ReturnsDomainMessage(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	user := createUser(t, admin, wardID)

	resp, env := doJSON(t, http.MethodPost, "/register", map[string]any{
		"wardId":   wardID,
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
		"wardId": "some-ward",
		// username and password omitted
	}, "")

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if env.Message == "" || env.Message == "Internal server error" {
		t.Errorf("message = %q, want a validator-produced message", env.Message)
	}
}
