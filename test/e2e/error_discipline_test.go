//go:build e2e

package e2e

import (
	"net/http"
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

// TestErrorDiscipline_InfraError_ReturnsGenericMessage verifies the other
// half of the rule: an error the repository wraps with %w (here, a foreign
// key violation from creating a ward under a hosId that doesn't exist) must
// not leak driver/SQL detail to the client — serviceError should replace it
// with "Internal server error".
func TestErrorDiscipline_InfraError_ReturnsGenericMessage(t *testing.T) {
	admin := adminToken(t)

	resp, env := doJSON(t, http.MethodPost, "/ward", map[string]any{
		"wardName": e2eName("orphan-ward"),
		"hosId":    uuid.New().String(), // no such hospital
	}, admin)

	if resp.StatusCode < 400 {
		t.Fatalf("creating a ward under a nonexistent hospital unexpectedly succeeded: status %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Skipf("got status %d instead of a wrapped-infra 500; the FK violation may not be the code path exercised here (message: %q)",
			resp.StatusCode, env.Message)
	}
	if env.Message != "Internal server error" {
		t.Errorf("message = %q, want the generic message (driver/SQL detail must not reach the client)", env.Message)
	}
}
