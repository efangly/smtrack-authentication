//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestRoleCeiling_AdminCannotEscalate verifies an ADMIN can neither promote
// itself nor create a user above its role ceiling.
func TestRoleCeiling_AdminCannotEscalate(t *testing.T) {
	root := adminToken(t)
	tenantID := createTenant(t, root, "HOSPITAL")
	admin := createAdmin(t, root, tenantID)
	login := loginAs(t, admin.Username, admin.Password)

	selfResp, selfEnv := doJSON(t, http.MethodPut, "/user/"+admin.ID, map[string]any{
		"role": "SUPER",
	}, login.Token)
	if selfResp.StatusCode != http.StatusBadRequest {
		t.Errorf("ADMIN self-promote: status = %d (%q), want 400", selfResp.StatusCode, selfEnv.Message)
	}

	createResp, createEnv := doJSON(t, http.MethodPost, "/user", map[string]any{
		"tenantId": tenantID,
		"username": e2eName("escalate"),
		"password": "TestPass123",
		"role":     "SUPER",
	}, login.Token)
	if createResp.StatusCode != http.StatusBadRequest {
		t.Errorf("ADMIN create SUPER: status = %d (%q), want 400", createResp.StatusCode, createEnv.Message)
	}

	// Still allowed: an ADMIN creating a plain USER.
	okResp, okEnv := doJSON(t, http.MethodPost, "/user", map[string]any{
		"tenantId": tenantID,
		"username": e2eName("plain"),
		"password": "TestPass123",
		"role":     "USER",
	}, login.Token)
	if okResp.StatusCode != http.StatusCreated {
		t.Fatalf("ADMIN create USER: status = %d (%q), want 201", okResp.StatusCode, okEnv.Message)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(okEnv.Data, &created); err == nil && created.ID != "" {
		t.Cleanup(func() { doJSON(t, http.MethodDelete, "/user/"+created.ID, nil, root) })
	}
}

// TestRoleCeiling_AdminCannotTouchPeerAdmin verifies an ADMIN cannot modify
// or delete another ADMIN.
func TestRoleCeiling_AdminCannotTouchPeerAdmin(t *testing.T) {
	root := adminToken(t)
	tenantID := createTenant(t, root, "HOSPITAL")
	self := createAdmin(t, root, tenantID)
	peer := createAdmin(t, root, tenantID)
	login := loginAs(t, self.Username, self.Password)

	putResp, _ := doJSON(t, http.MethodPut, "/user/"+peer.ID, map[string]any{
		"display": "hijacked",
	}, login.Token)
	if putResp.StatusCode != http.StatusBadRequest {
		t.Errorf("ADMIN PUT peer ADMIN: status = %d, want 400", putResp.StatusCode)
	}

	delResp, _ := doJSON(t, http.MethodDelete, "/user/"+peer.ID, nil, login.Token)
	if delResp.StatusCode != http.StatusBadRequest {
		t.Errorf("ADMIN DELETE peer ADMIN: status = %d, want 400", delResp.StatusCode)
	}
}
