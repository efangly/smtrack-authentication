//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"
)

// TestResetPassword_Self_WithCorrectOldPassword_Succeeds verifies the normal
// self-service flow: reset with the correct old password, then the new
// password logs in.
func TestResetPassword_Self_WithCorrectOldPassword_Succeeds(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)
	login := loginAs(t, user.Username, user.Password)

	newPassword := "NewPass456"
	resp, env := doJSON(t, http.MethodPatch, "/reset/"+user.Username, map[string]string{
		"password":    newPassword,
		"oldPassword": user.Password,
	}, login.Token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reset with correct old password: status %d, message %q", resp.StatusCode, env.Message)
	}

	loginAs(t, user.Username, newPassword) // fails the test itself if this doesn't work
}

// TestResetPassword_Self_WithWrongOldPassword_Rejected verifies a missing or
// incorrect oldPassword is rejected for a non-SUPER caller.
func TestResetPassword_Self_WithWrongOldPassword_Rejected(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)
	login := loginAs(t, user.Username, user.Password)

	wrongResp, _ := doJSON(t, http.MethodPatch, "/reset/"+user.Username, map[string]string{
		"password":    "NewPass456",
		"oldPassword": "not-the-real-password",
	}, login.Token)
	if wrongResp.StatusCode == http.StatusOK {
		t.Error("reset succeeded with a wrong old password")
	}

	missingResp, _ := doJSON(t, http.MethodPatch, "/reset/"+user.Username, map[string]string{
		"password": "NewPass456",
	}, login.Token)
	if missingResp.StatusCode == http.StatusOK {
		t.Error("reset succeeded with no old password at all")
	}
}

// TestResetPassword_NonAdminOnOtherAccount_Rejected verifies a non-admin
// caller cannot reset a different user's password, even if it happens to
// know that user's current password (i.e. this is an ownership check, not
// just a proof-of-old-password check).
func TestResetPassword_NonAdminOnOtherAccount_Rejected(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	self := createUser(t, admin, tenantID)
	other := createUser(t, admin, tenantID)
	login := loginAs(t, self.Username, self.Password)

	resp, _ := doJSON(t, http.MethodPatch, "/reset/"+other.Username, map[string]string{
		"password":    "NewPass456",
		"oldPassword": other.Password,
	}, login.Token)
	if resp.StatusCode == http.StatusOK {
		t.Error("non-admin caller was able to reset a different account's password")
	}
}

// TestResetPassword_SuperOnOtherAccount_NoOldPasswordRequired verifies the
// caller.Role != SUPER gate: a SUPER-tier caller can reset someone else's
// password without proving the old one. Assumes E2E_ADMIN_USERNAME names a
// SUPER account specifically (not merely admin-tier); if it is ADMIN or
// SERVICE instead, this test will fail at the old-password check and should
// be skipped/adjusted rather than treated as a real regression.
func TestResetPassword_SuperOnOtherAccount_NoOldPasswordRequired(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	target := createUser(t, admin, tenantID)

	resp, env := doJSON(t, http.MethodPatch, "/reset/"+target.Username, map[string]string{
		"password": "NewPass456",
	}, admin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SUPER reset without old password: status %d, message %q "+
			"(if E2E_ADMIN_USERNAME is ADMIN/SERVICE rather than SUPER, this failure is expected)",
			resp.StatusCode, env.Message)
	}

	loginAs(t, target.Username, "NewPass456")
}

// TestResetPassword_InvalidatesEarlierRefreshTokens verifies a password change
// ends the sessions started with the old password: the refresh token issued
// before the reset stops working, while signing in again does.
func TestResetPassword_InvalidatesEarlierRefreshTokens(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)
	before := loginAs(t, user.Username, user.Password)

	// Token timestamps have one-second resolution; make sure the reset is later.
	time.Sleep(1100 * time.Millisecond)
	resp, env := doJSON(t, http.MethodPatch, "/reset/"+user.Username, map[string]string{
		"password": "NewPass456",
	}, admin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reset: status %d, message %q", resp.StatusCode, env.Message)
	}

	if resp, _ := doJSON(t, http.MethodPost, "/refresh", map[string]string{"token": before.RefreshToken}, ""); resp.StatusCode == http.StatusOK {
		t.Error("a refresh token issued before the password change still works")
	}
	after := loginAs(t, user.Username, "NewPass456")
	if resp, env := doJSON(t, http.MethodPost, "/refresh", map[string]string{"token": after.RefreshToken}, ""); resp.StatusCode != http.StatusOK {
		t.Errorf("refresh token issued after the change: status %d, message %q", resp.StatusCode, env.Message)
	}
}
