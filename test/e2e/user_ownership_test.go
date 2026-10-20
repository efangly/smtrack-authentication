//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestUserOwnership_SelfAccess_Allowed verifies a non-admin caller can read
// and update their own record.
func TestUserOwnership_SelfAccess_Allowed(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	user := createUser(t, admin, wardID)
	login := loginAs(t, user.Username, user.Password)

	getResp, getEnv := doJSON(t, http.MethodGet, "/user/"+user.ID, nil, login.Token)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("self GET: status %d, message %q", getResp.StatusCode, getEnv.Message)
	}

	display := "my own display name"
	putResp, putEnv := doJSON(t, http.MethodPut, "/user/"+user.ID, map[string]any{
		"display": display,
	}, login.Token)
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("self PUT: status %d, message %q", putResp.StatusCode, putEnv.Message)
	}
}

// TestUserOwnership_OtherUserAccess_Forbidden verifies a non-admin caller
// cannot read or update someone else's record.
func TestUserOwnership_OtherUserAccess_Forbidden(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	self := createUser(t, admin, wardID)
	other := createUser(t, admin, wardID)
	login := loginAs(t, self.Username, self.Password)

	getResp, _ := doJSON(t, http.MethodGet, "/user/"+other.ID, nil, login.Token)
	if getResp.StatusCode != http.StatusForbidden {
		t.Errorf("GET another user: status = %d, want 403", getResp.StatusCode)
	}

	putResp, _ := doJSON(t, http.MethodPut, "/user/"+other.ID, map[string]any{
		"display": "hijacked",
	}, login.Token)
	if putResp.StatusCode != http.StatusForbidden {
		t.Errorf("PUT another user: status = %d, want 403", putResp.StatusCode)
	}
}

// TestUserOwnership_SelfUpdate_StripsPrivilegedFields verifies a non-admin
// caller cannot self-grant role/status/wardId changes even though the
// request itself succeeds (fields are silently stripped, not rejected).
func TestUserOwnership_SelfUpdate_StripsPrivilegedFields(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	otherWardID := createWard(t, admin, hosID)
	user := createUser(t, admin, wardID)
	login := loginAs(t, user.Username, user.Password)

	putResp, putEnv := doJSON(t, http.MethodPut, "/user/"+user.ID, map[string]any{
		"role":   "SUPER",
		"status": false,
		"wardId": otherWardID,
	}, login.Token)
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("self PUT with privileged fields: status %d, message %q", putResp.StatusCode, putEnv.Message)
	}

	getResp, getEnv := doJSON(t, http.MethodGet, "/user/"+user.ID, nil, login.Token)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("self GET after update: status %d, message %q", getResp.StatusCode, getEnv.Message)
	}
	var current struct {
		Role   string `json:"role"`
		Status bool   `json:"status"`
		WardID string `json:"wardId"`
	}
	if err := json.Unmarshal(getEnv.Data, &current); err != nil {
		t.Fatalf("decode user: %v", err)
	}

	if current.Role == "SUPER" {
		t.Error("role was self-granted; handler must strip it for non-admin callers")
	}
	if !current.Status {
		t.Error("status was self-disabled; handler must strip it for non-admin callers")
	}
	if current.WardID == otherWardID {
		t.Error("wardId was self-changed; handler must strip it for non-admin callers")
	}
}

// TestUserOwnership_Delete_RequiresAdminTier verifies DELETE is
// RolesGuard-only now: even the account owner cannot delete themselves, and
// an admin-tier caller can delete it.
func TestUserOwnership_Delete_RequiresAdminTier(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	user := createUser(t, admin, wardID)
	login := loginAs(t, user.Username, user.Password)

	selfDeleteResp, _ := doJSON(t, http.MethodDelete, "/user/"+user.ID, nil, login.Token)
	if selfDeleteResp.StatusCode != http.StatusForbidden {
		t.Errorf("self DELETE: status = %d, want 403 (delete is admin-tier only, no ownership exception)", selfDeleteResp.StatusCode)
	}

	adminDeleteResp, adminDeleteEnv := doJSON(t, http.MethodDelete, "/user/"+user.ID, nil, admin)
	if adminDeleteResp.StatusCode != http.StatusOK {
		t.Fatalf("admin DELETE: status %d, message %q", adminDeleteResp.StatusCode, adminDeleteEnv.Message)
	}
}
