//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// envelope mirrors dto.StandardResponse / dto.ErrorResponse.
type envelope struct {
	Message string          `json:"message"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

// publicRouteGate paces requests against /login and /register, which share a
// single 10-req/min-per-IP limiter instance in router.go (the same
// publicLimiter is mounted on both routes, so the budget is combined, not 10
// each). Without pacing, this suite's fixture setup alone blows through the
// budget in under a second and every subsequent request gets 429'd for the
// rest of the window. The gate keeps a rolling window of the last N call
// timestamps and sleeps just long enough to stay under the limit.
var publicRouteGate = newRateGate(8, 65*time.Second) // headroom under the server's 10/min

type rateGate struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	calls  []time.Time
}

func newRateGate(max int, window time.Duration) *rateGate {
	return &rateGate{max: max, window: window}
}

func (g *rateGate) wait() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for {
		now := time.Now()
		cutoff := now.Add(-g.window)
		kept := g.calls[:0]
		for _, c := range g.calls {
			if c.After(cutoff) {
				kept = append(kept, c)
			}
		}
		g.calls = kept

		if len(g.calls) < g.max {
			g.calls = append(g.calls, now)
			return
		}

		sleepFor := g.calls[0].Add(g.window).Sub(now)
		g.mu.Unlock()
		time.Sleep(sleepFor)
		g.mu.Lock()
	}
}

// doJSON sends a JSON request against the running service and decodes the
// standard response envelope. body may be nil for GET/DELETE. It is paced by
// publicRouteGate on /login and /register so fixture setup elsewhere in the
// suite doesn't trip the server's shared rate limit; TestLogin_RateLimited
// deliberately wants to trip that limit, so it calls doJSONUnthrottled
// instead.
func doJSON(t *testing.T, method, path string, body any, token string) (*http.Response, envelope) {
	t.Helper()
	if method == http.MethodPost && (path == "/login" || path == "/register") {
		publicRouteGate.wait()
	}
	return doJSONUnthrottled(t, method, path, body, token)
}

// doJSONUnthrottled is doJSON without the publicRouteGate pacing. Only
// TestLogin_RateLimited should call this directly — everything else should
// go through doJSON so it doesn't burn the shared /login+/register budget
// for other tests.
func doJSONUnthrottled(t *testing.T, method, path string, body any, token string) (*http.Response, envelope) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var env envelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("%s %s: decode response %q: %v", method, path, raw, err)
		}
	}
	// resp.Body is already drained/closed; hand back a resp with status/headers
	// intact for callers that only check resp.StatusCode.
	return resp, env
}

// e2eName returns a unique, easily-greppable name for throwaway fixtures.
func e2eName(prefix string) string {
	return fmt.Sprintf("e2e-%s-%s", prefix, uuid.New().String()[:8])
}

type authResult struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	ID           string `json:"id"`
}

// loginAs logs in with username/password and returns the access + refresh
// tokens. Fails the test on any non-2xx response.
func loginAs(t *testing.T, username, password string) authResult {
	t.Helper()
	resp, env := doJSON(t, http.MethodPost, "/login", map[string]string{
		"username": username,
		"password": password,
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login as %s: status %d, message %q", username, resp.StatusCode, env.Message)
	}
	var result authResult
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("decode login result: %v", err)
	}
	return result
}

var (
	adminTokenOnce   sync.Once
	adminTokenCached string
)

// adminToken logs in as the seeded admin account once per test run and
// caches the token — every test needs one, and re-logging-in per test would
// needlessly eat into the shared /login+/register rate-limit budget.
func adminToken(t *testing.T) string {
	t.Helper()
	adminTokenOnce.Do(func() {
		adminTokenCached = loginAs(t, adminUsername, adminPassword).Token
	})
	if adminTokenCached == "" {
		t.Fatal("admin login failed on first attempt; see earlier test output")
	}
	return adminTokenCached
}

// createHospital creates a throwaway hospital as the given (admin) caller and
// registers cleanup. Returns the new hospital's id.
func createHospital(t *testing.T, callerToken string) string {
	t.Helper()
	name := e2eName("hospital")
	resp, env := doJSON(t, http.MethodPost, "/hospital", map[string]any{
		"hosName": name,
	}, callerToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create hospital: status %d, message %q", resp.StatusCode, env.Message)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatalf("decode created hospital: %v", err)
	}

	t.Cleanup(func() {
		doJSON(t, http.MethodDelete, "/hospital/"+created.ID, nil, callerToken)
	})
	return created.ID
}

// createWard creates a throwaway ward under hosID as the given (admin)
// caller and registers cleanup. Returns the new ward's id.
func createWard(t *testing.T, callerToken, hosID string) string {
	t.Helper()
	name := e2eName("ward")
	resp, env := doJSON(t, http.MethodPost, "/ward", map[string]any{
		"wardName": name,
		"hosId":    hosID,
	}, callerToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create ward: status %d, message %q", resp.StatusCode, env.Message)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatalf("decode created ward: %v", err)
	}

	t.Cleanup(func() {
		doJSON(t, http.MethodDelete, "/ward/"+created.ID, nil, callerToken)
	})
	return created.ID
}

// registeredUser is a throwaway GUEST user's id, username, and the plaintext
// password used to create it (needed later for login/reset-password
// scenarios).
type registeredUser struct {
	ID       string
	Username string
	Password string
}

// createUser creates a throwaway GUEST user via the authenticated admin
// route (POST /auth/user), which carries no rate limit — unlike the public
// /auth/register route exercised directly in auth_register_test.go. Prefer
// this for generic fixture setup so the shared /login+/register rate-limit
// budget (see publicRouteGate) is spent on requests tests actually care
// about, not on incidental setup.
func createUser(t *testing.T, callerToken, wardID string) registeredUser {
	t.Helper()
	username := e2eName("user")
	password := "TestPass123"

	resp, env := doJSON(t, http.MethodPost, "/user", map[string]any{
		"wardId":   wardID,
		"username": username,
		"password": password,
	}, callerToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create user: status %d, message %q", resp.StatusCode, env.Message)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &created); err != nil {
		t.Fatalf("decode created user: %v", err)
	}

	t.Cleanup(func() {
		doJSON(t, http.MethodDelete, "/user/"+created.ID, nil, callerToken)
	})
	return registeredUser{ID: created.ID, Username: username, Password: password}
}
