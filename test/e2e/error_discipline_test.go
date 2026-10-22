//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestErrorDiscipline_DomainNotFound_PassesMessageThrough verifies
// serviceError's rule: an unwrapped domain error (userService.FindOne
// returns errors.New("user not found") when the row genuinely doesn't
// exist) is safe to return verbatim, not replaced with a generic message.
func TestErrorDiscipline_DomainNotFound_PassesMessageThrough(t *testing.T) {
	admin := adminToken(t)

	resp, env := doJSON(t, http.MethodGet, "/user/"+uuid.New().String(), nil, admin)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if env.Message != "user not found" {
		t.Errorf("message = %q, want the domain error text verbatim (\"user not found\")", env.Message)
	}
}

// TestErrorDiscipline_NoSQLDetailReachesClient creates a department under a
// tenant that does not exist. Whichever layer rejects it, the client must get
// a plain message: no driver or SQL detail (serviceError replaces anything that
// wraps an infrastructure cause with "Internal server error").
func TestErrorDiscipline_NoSQLDetailReachesClient(t *testing.T) {
	admin := adminToken(t)

	resp, env := doJSON(t, http.MethodPost, "/department", map[string]any{
		"name":     e2eName("orphan-dept"),
		"tenantId": uuid.New().String(), // no such tenant
	}, admin)

	if resp.StatusCode < 400 {
		t.Fatalf("creating a department under a nonexistent tenant unexpectedly succeeded: status %d", resp.StatusCode)
	}
	for _, leak := range []string{"SQLSTATE", "violates", "constraint", "pq:", "pgx", "gorm"} {
		if strings.Contains(env.Message, leak) {
			t.Errorf("message %q leaks %q", env.Message, leak)
		}
	}
	if resp.StatusCode == http.StatusInternalServerError && env.Message != "Internal server error" {
		t.Errorf("a 500 must carry the generic message, got %q", env.Message)
	}
}
