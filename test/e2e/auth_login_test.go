//go:build e2e

package e2e

import (
	"net/http"
	"os"
	"testing"
)

// TestLogin_Success verifies the happy path still issues both tokens.
func TestLogin_Success(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	user := createUser(t, admin, wardID)

	result := loginAs(t, user.Username, user.Password)
	if result.Token == "" || result.RefreshToken == "" {
		t.Fatalf("expected both tokens, got %+v", result)
	}
}

// TestLogin_WrongPasswordAndUnknownUser_SameGenericMessage checks that a
// wrong password and a nonexistent username are indistinguishable to the
// caller, which is what the dummy-hash timing-safety branch in
// authService.ValidateUser is there to preserve (message-wise, at least;
// exact timing is not asserted here since network jitter would make it
// flaky).
func TestLogin_WrongPasswordAndUnknownUser_SameGenericMessage(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	user := createUser(t, admin, wardID)

	respWrongPass, envWrongPass := doJSON(t, http.MethodPost, "/login", map[string]string{
		"username": user.Username,
		"password": "wrong-password-1234", // must stay <=20 chars (LoginRequest validate:"max=20")
	}, "")
	respUnknownUser, envUnknownUser := doJSON(t, http.MethodPost, "/login", map[string]string{
		"username": e2eName("nonexistent"),
		"password": "whatever-password",
	}, "")

	if respWrongPass.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password: status = %d, want 401", respWrongPass.StatusCode)
	}
	if respUnknownUser.StatusCode != http.StatusUnauthorized {
		t.Errorf("unknown user: status = %d, want 401", respUnknownUser.StatusCode)
	}
	if envWrongPass.Message != envUnknownUser.Message {
		t.Errorf("messages differ (%q vs %q); an attacker could use this to enumerate usernames",
			envWrongPass.Message, envUnknownUser.Message)
	}
}

// TestLogin_DisabledAccount_SameGenericMessage verifies a deactivated account
// cannot log in even with the right password, and gets the same response as a
// wrong password so its existence is not revealed.
func TestLogin_DisabledAccount_SameGenericMessage(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	user := createUser(t, admin, wardID)

	if resp, env := doJSON(t, http.MethodPut, "/user/"+user.ID, map[string]any{
		"status": false,
	}, admin); resp.StatusCode != http.StatusOK {
		t.Fatalf("deactivate user: status %d, message %q", resp.StatusCode, env.Message)
	}

	respDisabled, envDisabled := doJSON(t, http.MethodPost, "/login", map[string]string{
		"username": user.Username,
		"password": user.Password,
	}, "")
	respWrongPass, envWrongPass := doJSON(t, http.MethodPost, "/login", map[string]string{
		"username": user.Username,
		"password": "wrong-password-1234",
	}, "")

	if respDisabled.StatusCode != http.StatusUnauthorized {
		t.Errorf("disabled account: status = %d, want 401", respDisabled.StatusCode)
	}
	if envDisabled.Message != envWrongPass.Message {
		t.Errorf("messages differ (%q vs %q); a disabled account must look like a wrong password",
			envDisabled.Message, envWrongPass.Message)
	}
	if respWrongPass.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password: status = %d, want 401", respWrongPass.StatusCode)
	}
}

// TestLogin_RateLimited exhausts the per-IP limiter (10/min) on /auth/login
// and expects the 11th request within the window to be rejected with 429.
// Run in its own test (not a subtest sharing state with others) because it
// deliberately taints the limiter window for this process's IP; the suite
// should treat it as the last login-related test to run.
//
// It is gated behind E2E_RUN_RATE_LIMIT=1 and skipped by default: every
// other test in this suite calls loginAs, and this test's whole point is to
// exhaust the per-IP login limiter, which would 429 all of them for the rest
// of the window if it ran inline. Run it on its own:
//
//	E2E_RUN_RATE_LIMIT=1 go test -tags=e2e ./test/e2e/... -run TestLogin_RateLimited -v
func TestLogin_RateLimited(t *testing.T) {
	if os.Getenv("E2E_RUN_RATE_LIMIT") != "1" {
		t.Skip("set E2E_RUN_RATE_LIMIT=1 to run this in isolation; it exhausts the login rate limit for the rest of the suite")
	}

	body := map[string]string{
		"username": e2eName("rate-limit-probe"),
		"password": "whatever-password",
	}

	var last *http.Response
	for i := 0; i < 11; i++ {
		resp, _ := doJSONUnthrottled(t, http.MethodPost, "/login", body, "")
		last = resp
	}

	if last.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("11th request in the window: status = %d, want 429", last.StatusCode)
	}
}
