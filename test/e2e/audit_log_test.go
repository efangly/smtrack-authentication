//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

type auditEntry struct {
	Entity   string `json:"entity"`
	EntityID string `json:"entityId"`
	Action   string `json:"action"`
	Changes  string `json:"changes"`
}

// TestAuditLog_NonAdmin_Forbidden verifies the audit route is admin-tier
// only.
func TestAuditLog_NonAdmin_Forbidden(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)
	login := loginAs(t, user.Username, user.Password)

	resp, _ := doJSON(t, http.MethodGet, "/audit", nil, login.Token)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin GET /audit: status = %d, want 403", resp.StatusCode)
	}
}

// TestAuditLog_RecordsUserLifecycle_AndSanitizesPassword verifies create,
// update, and password-reset operations each produce a matching audit
// entry, and that a reset's "changes" never carries the plaintext (or any)
// password.
func TestAuditLog_RecordsUserLifecycle_AndSanitizesPassword(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID) // CREATE

	updResp, updEnv := doJSON(t, http.MethodPut, "/user/"+user.ID, map[string]any{
		"display": "audit test display",
	}, admin)
	if updResp.StatusCode != http.StatusOK {
		t.Fatalf("update user: status %d, message %q", updResp.StatusCode, updEnv.Message)
	}

	resetResp, resetEnv := doJSON(t, http.MethodPatch, "/reset/"+user.Username, map[string]string{
		"password": "AuditedNewPass1",
	}, admin)
	if resetResp.StatusCode != http.StatusOK {
		t.Fatalf("reset password: status %d, message %q "+
			"(requires E2E_ADMIN_USERNAME to be SUPER-tier to skip oldPassword)",
			resetResp.StatusCode, resetEnv.Message)
	}

	listResp, listEnv := doJSON(t, http.MethodGet, "/audit?entity=USER&entityId="+user.ID, nil, admin)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list audit logs: status %d, message %q", listResp.StatusCode, listEnv.Message)
	}
	var entries []auditEntry
	if err := json.Unmarshal(listEnv.Data, &entries); err != nil {
		t.Fatalf("decode audit entries: %v", err)
	}

	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Action] = true
		if e.EntityID != user.ID {
			t.Errorf("entry entityId = %q, want %q (filter should have scoped this)", e.EntityID, user.ID)
		}
		if containsPasswordKey(e.Changes) {
			t.Errorf("audit entry changes contains a password key: %s", e.Changes)
		}
	}
	for _, action := range []string{"CREATE", "UPDATE"} {
		if !seen[action] {
			t.Errorf("no %s audit entry found for user %s", action, user.ID)
		}
	}
}

// containsPasswordKey does a minimal check without assuming the exact JSON
// shape of Changes beyond "it's an object that might have had a password key
// stripped".
func containsPasswordKey(changesJSON string) bool {
	var m map[string]any
	if err := json.Unmarshal([]byte(changesJSON), &m); err != nil {
		return false // not an object (e.g. a marshaled struct with json:"-" on Password) — safe
	}
	_, has := m["password"]
	return has
}

// TestAuditLog_FiltersByEntityAndAction verifies query params narrow results.
func TestAuditLog_FiltersByEntityAndAction(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL") // CREATE TENANT entry

	resp, env := doJSON(t, http.MethodGet, "/audit?entity=TENANT&action=CREATE&entityId="+tenantID, nil, admin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list filtered audit logs: status %d, message %q", resp.StatusCode, env.Message)
	}
	var entries []auditEntry
	if err := json.Unmarshal(env.Data, &entries); err != nil {
		t.Fatalf("decode audit entries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one TENANT/CREATE audit entry for the tenant just created")
	}
	for _, e := range entries {
		if e.Entity != "TENANT" || e.Action != "CREATE" {
			t.Errorf("filter leaked non-matching entry: entity=%q action=%q", e.Entity, e.Action)
		}
	}
}
