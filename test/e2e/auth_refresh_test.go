//go:build e2e

package e2e

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// decodeJWTClaims reads the claims segment of a JWT without verifying its
// signature — fine here since these are tokens the test itself just
// received from a trusted server, and we only want to inspect the payload.
func decodeJWTClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT: %q", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode JWT payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("unmarshal JWT claims: %v", err)
	}
	return claims
}

// TestRefresh_DeactivatedUser_LosesAccess verifies RefreshTokens rebuilds
// claims from the database rather than copying the old token's claims: once
// an admin deactivates the user, their existing refresh token must stop
// working immediately instead of remaining valid for its full lifetime.
func TestRefresh_DeactivatedUser_LosesAccess(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)

	login := loginAs(t, user.Username, user.Password)

	resp, env := doJSON(t, http.MethodPut, "/user/"+user.ID, map[string]any{
		"status": false,
	}, admin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deactivate user: status %d, message %q", resp.StatusCode, env.Message)
	}

	refreshResp, refreshEnv := doJSON(t, http.MethodPost, "/refresh", map[string]string{
		"token": login.RefreshToken,
	}, "")
	if refreshResp.StatusCode == http.StatusOK {
		t.Fatal("refresh succeeded for a deactivated user; claims were not rebuilt from the database")
	}
	if refreshEnv.Message == "" {
		t.Error("expected a rejection message on refresh of a deactivated user's token")
	}
}

// TestRefresh_RoleChange_ReflectsNewRoleInClaims verifies a promoted/demoted
// user's next refresh carries the *current* role, not the one recorded when
// the refresh token was issued.
func TestRefresh_RoleChange_ReflectsNewRoleInClaims(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)

	login := loginAs(t, user.Username, user.Password)
	issuedClaims := decodeJWTClaims(t, login.RefreshToken)
	if issuedClaims["role"] != "GUEST" {
		t.Fatalf("issuedClaims[role] = %v, want GUEST", issuedClaims["role"])
	}

	resp, env := doJSON(t, http.MethodPut, "/user/"+user.ID, map[string]any{
		"role": "USER",
	}, admin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("promote user: status %d, message %q", resp.StatusCode, env.Message)
	}

	refreshResp, refreshEnv := doJSON(t, http.MethodPost, "/refresh", map[string]string{
		"token": login.RefreshToken,
	}, "")
	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("refresh: status %d, message %q", refreshResp.StatusCode, refreshEnv.Message)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(refreshEnv.Data, &result); err != nil {
		t.Fatalf("decode refresh result: %v", err)
	}
	newClaims := decodeJWTClaims(t, result.Token)
	if newClaims["role"] != "USER" {
		t.Errorf("refreshed token role = %v, want USER (rebuilt from DB, not copied from old token)", newClaims["role"])
	}
}

// TestRefresh_DepartmentChange_ReflectsNewDepartmentsInClaims verifies the
// department list in the token follows the database at the next refresh.
func TestRefresh_DepartmentChange_ReflectsNewDepartmentsInClaims(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "FACTORY")
	line1 := createDepartment(t, admin, tenantID)
	line2 := createDepartment(t, admin, tenantID)
	user := createUser(t, admin, tenantID, line1)

	login := loginAs(t, user.Username, user.Password)
	claims := decodeJWTClaims(t, login.Token)
	if claims["tenantId"] != tenantID {
		t.Fatalf("claims tenantId = %v, want %s", claims["tenantId"], tenantID)
	}
	if ids, _ := claims["departmentIds"].([]any); len(ids) != 1 || ids[0] != line1 {
		t.Fatalf("claims departmentIds = %v, want [%s]", claims["departmentIds"], line1)
	}

	if resp, env := doJSON(t, http.MethodPut, "/user/"+user.ID, map[string]any{
		"departmentIds": []string{line1, line2},
	}, admin); resp.StatusCode != http.StatusOK {
		t.Fatalf("link second department: status %d, message %q", resp.StatusCode, env.Message)
	}

	resp, env := doJSON(t, http.MethodPost, "/refresh", map[string]string{"token": login.RefreshToken}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh: status %d, message %q", resp.StatusCode, env.Message)
	}
	var refreshed struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &refreshed); err != nil {
		t.Fatalf("decode refresh result: %v", err)
	}
	ids, _ := decodeJWTClaims(t, refreshed.Token)["departmentIds"].([]any)
	if len(ids) != 2 {
		t.Errorf("refreshed departmentIds = %v, want both departments", ids)
	}
}
