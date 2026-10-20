//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// TestWardGuard_GuestCannotMutateWard verifies a GUEST — the role anyone can
// obtain through public registration — can neither rename nor delete a ward.
func TestWardGuard_GuestCannotMutateWard(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	guest := createUser(t, admin, wardID)
	login := loginAs(t, guest.Username, guest.Password)

	putResp, _ := doJSON(t, http.MethodPut, "/ward/"+wardID, map[string]any{
		"wardName": "hijacked",
	}, login.Token)
	if putResp.StatusCode != http.StatusForbidden {
		t.Errorf("guest PUT ward: status = %d, want 403", putResp.StatusCode)
	}

	delResp, _ := doJSON(t, http.MethodDelete, "/ward/"+wardID, nil, login.Token)
	if delResp.StatusCode != http.StatusForbidden {
		t.Errorf("guest DELETE ward: status = %d, want 403", delResp.StatusCode)
	}

	// The ward must still exist and be unchanged.
	getResp, getEnv := doJSON(t, http.MethodGet, "/ward/"+wardID, nil, admin)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("admin GET ward after guest attempts: status %d, message %q", getResp.StatusCode, getEnv.Message)
	}
}
