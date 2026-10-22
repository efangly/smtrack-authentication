//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

type securityEvent struct {
	Action    string `json:"action"`
	EntityID  string `json:"entityId"`
	ActorID   string `json:"actorId"`
	TenantID  string `json:"tenantId"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	RequestID string `json:"requestId"`
	Changes   string `json:"changes"`
}

func auditFor(t *testing.T, token, query string) []securityEvent {
	t.Helper()
	resp, env := doJSON(t, http.MethodGet, "/audit?limit=200&"+query, nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit %s: status %d, message %q", query, resp.StatusCode, env.Message)
	}
	var events []securityEvent
	if err := json.Unmarshal(env.Data, &events); err != nil {
		t.Fatalf("decode audit: %v", err)
	}
	return events
}

func hasEvent(events []securityEvent, action string) *securityEvent {
	for i := range events {
		if events[i].Action == action {
			return &events[i]
		}
	}
	return nil
}

// TestLockout_FiveFailures_BlockEvenTheRightPassword verifies the per-username
// lockout: after five wrong passwords the right one is refused too, with the
// same answer a wrong password gets.
func TestLockout_FiveFailures_BlockEvenTheRightPassword(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)

	var wrongMessage string
	for i := 0; i < 5; i++ {
		resp, env := doJSON(t, http.MethodPost, "/login", map[string]string{"username": user.Username, "password": "wrong-password-1"}, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("wrong password #%d: status = %d, want 401", i+1, resp.StatusCode)
		}
		wrongMessage = env.Message
	}

	resp, env := doJSON(t, http.MethodPost, "/login", map[string]string{"username": user.Username, "password": user.Password}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("right password while locked: status = %d, want 401", resp.StatusCode)
	}
	if env.Message != wrongMessage {
		t.Errorf("a locked account answers %q, a wrong password %q; the difference reveals the lock", env.Message, wrongMessage)
	}

	events := auditFor(t, admin, "entity=USER&action=LOGIN_FAILURE&entityId="+user.Username)
	var locked bool
	for _, e := range events {
		if e.Changes == `{"reason":"locked"}` {
			locked = true
		}
	}
	if !locked {
		t.Errorf("no LOGIN_FAILURE with reason locked for %s: %+v", user.Username, events)
	}
}

// TestLogout_RevokesRefreshToken_AndRefusesSomeoneElsesToken covers POST
// /auth/logout.
func TestLogout_RevokesRefreshToken_AndRefusesSomeoneElsesToken(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)
	other := createUser(t, admin, tenantID)
	login := loginAs(t, user.Username, user.Password)
	otherLogin := loginAs(t, other.Username, other.Password)

	if resp, _ := doJSON(t, http.MethodPost, "/logout", map[string]string{"token": otherLogin.RefreshToken}, login.Token); resp.StatusCode == http.StatusOK {
		t.Error("logged out with somebody else's refresh token")
	}
	if resp, _ := doJSON(t, http.MethodPost, "/refresh", map[string]string{"token": otherLogin.RefreshToken}, ""); resp.StatusCode != http.StatusOK {
		t.Error("the other user's token was revoked by a rejected logout")
	}

	if resp, env := doJSON(t, http.MethodPost, "/logout", map[string]string{"token": login.RefreshToken}, login.Token); resp.StatusCode != http.StatusOK {
		t.Fatalf("logout: status %d, message %q", resp.StatusCode, env.Message)
	}
	if resp, _ := doJSON(t, http.MethodPost, "/refresh", map[string]string{"token": login.RefreshToken}, ""); resp.StatusCode == http.StatusOK {
		t.Error("refresh succeeded after logout")
	}
	if resp, _ := doJSON(t, http.MethodPost, "/logout", map[string]string{"token": login.RefreshToken}, ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("logout without an access token: status = %d, want 401", resp.StatusCode)
	}
	if hasEvent(auditFor(t, admin, "entity=USER&action=LOGOUT&entityId="+user.ID), "LOGOUT") == nil {
		t.Error("no LOGOUT audit entry")
	}
}

// TestAudit_LoginRecordsRequestMetadata verifies who, from where and which
// request: IP, user agent, the gateway's request id and the user's tenant.
func TestAudit_LoginRecordsRequestMetadata(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)

	requestID := e2eName("req")
	publicRouteGate.wait()
	resp, env := doJSONWith(t, http.MethodPost, "/login", map[string]string{"username": user.Username, "password": user.Password}, "",
		map[string]string{"X-Request-ID": requestID, "User-Agent": "e2e-suite/1.0"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: status %d, message %q", resp.StatusCode, env.Message)
	}
	if got := resp.Header.Get("X-Request-ID"); got != requestID {
		t.Errorf("response X-Request-ID = %q, want %q", got, requestID)
	}

	e := hasEvent(auditFor(t, admin, "entity=USER&action=LOGIN_SUCCESS&entityId="+user.ID), "LOGIN_SUCCESS")
	if e == nil {
		t.Fatal("no LOGIN_SUCCESS audit entry")
	}
	if e.RequestID != requestID || e.UserAgent != "e2e-suite/1.0" || e.IP == "" || e.TenantID != tenantID || e.ActorID != user.ID {
		t.Errorf("LOGIN_SUCCESS entry lacks its context: %+v", *e)
	}
}

// TestAudit_AccessDeniedIsRecorded verifies a 403 given to a signed-in caller
// leaves an ACCESS_DENIED entry.
func TestAudit_AccessDeniedIsRecorded(t *testing.T) {
	admin := adminToken(t)
	tenantID := createTenant(t, admin, "HOSPITAL")
	user := createUser(t, admin, tenantID)
	login := loginAs(t, user.Username, user.Password)

	if resp, _ := doJSON(t, http.MethodGet, "/audit", nil, login.Token); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GUEST GET /audit: status = %d, want 403", resp.StatusCode)
	}
	if hasEvent(auditFor(t, admin, "entity=USER&action=ACCESS_DENIED&entityId="+user.ID), "ACCESS_DENIED") == nil {
		t.Error("no ACCESS_DENIED audit entry")
	}
}

// TestAccessReport_ScopedAndAudited verifies GET /auth/report/access.
func TestAccessReport_ScopedAndAudited(t *testing.T) {
	root := adminToken(t)
	tenA := createTenant(t, root, "HOSPITAL")
	tenB := createTenant(t, root, "LAB")
	admin := createAdmin(t, root, tenA)
	member := createUser(t, root, tenA)
	foreign := createUser(t, root, tenB)
	adminLogin := loginAs(t, admin.Username, admin.Password)
	memberLogin := loginAs(t, member.Username, member.Password)

	if resp, _ := doJSON(t, http.MethodGet, "/report/access", nil, memberLogin.Token); resp.StatusCode != http.StatusForbidden {
		t.Errorf("GUEST access report: status = %d, want 403", resp.StatusCode)
	}

	resp, env := doJSON(t, http.MethodGet, "/report/access", nil, adminLogin.Token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ADMIN access report: status %d, message %q", resp.StatusCode, env.Message)
	}
	var rows []userView
	if err := json.Unmarshal(env.Data, &rows); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	var sawMember bool
	for _, r := range rows {
		if r.TenantID != tenA {
			t.Errorf("tenant admin's report contains a user of tenant %s", r.TenantID)
		}
		if r.ID == foreign.ID {
			t.Error("tenant admin's report contains another tenant's user")
		}
		if r.ID == member.ID {
			sawMember = true
			if r.LastLoginAt == nil {
				t.Error("the report does not show the member's last login")
			}
		}
	}
	if !sawMember {
		t.Error("the report is missing a user of the admin's own tenant")
	}

	if hasEvent(auditFor(t, root, "entity=USER&action=ACCESS_REPORT&entityId="+admin.ID), "ACCESS_REPORT") == nil {
		t.Error("the report call was not audited")
	}
}
