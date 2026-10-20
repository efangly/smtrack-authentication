//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// TestTenantScope_AdminConfinedToOwnHospital verifies an ADMIN of hospital A
// can neither see nor change anything belonging to hospital B, and cannot
// read the cross-hospital audit log.
func TestTenantScope_AdminConfinedToOwnHospital(t *testing.T) {
	root := adminToken(t)
	hosA := createHospital(t, root)
	wardA := createWard(t, root, hosA)
	hosB := createHospital(t, root)
	wardB := createWard(t, root, hosB)

	admin := createAdmin(t, root, wardA)
	ownUser := createUser(t, root, wardA)
	otherUser := createUser(t, root, wardB)
	login := loginAs(t, admin.Username, admin.Password)

	// Own hospital still works.
	if resp, env := doJSON(t, http.MethodGet, "/user/"+ownUser.ID, nil, login.Token); resp.StatusCode != http.StatusOK {
		t.Fatalf("ADMIN GET own-hospital user: status %d, message %q", resp.StatusCode, env.Message)
	}
	if resp, env := doJSON(t, http.MethodGet, "/hospital/"+hosA, nil, login.Token); resp.StatusCode != http.StatusOK {
		t.Fatalf("ADMIN GET own hospital: status %d, message %q", resp.StatusCode, env.Message)
	}

	// Other hospital reads as not found / is refused.
	checks := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{"GET other-hospital user", http.MethodGet, "/user/" + otherUser.ID, nil, http.StatusNotFound},
		{"PUT other-hospital user", http.MethodPut, "/user/" + otherUser.ID, map[string]any{"display": "x"}, http.StatusBadRequest},
		{"DELETE other-hospital user", http.MethodDelete, "/user/" + otherUser.ID, nil, http.StatusBadRequest},
		{"move own user into other hospital", http.MethodPut, "/user/" + ownUser.ID, map[string]any{"wardId": wardB}, http.StatusBadRequest},
		{"GET other hospital", http.MethodGet, "/hospital/" + hosB, nil, http.StatusNotFound},
		{"GET other-hospital ward", http.MethodGet, "/ward/" + wardB, nil, http.StatusNotFound},
		{"GET audit log", http.MethodGet, "/audit", nil, http.StatusForbidden},
	}
	for _, c := range checks {
		resp, env := doJSON(t, c.method, c.path, c.body, login.Token)
		if resp.StatusCode != c.want {
			t.Errorf("%s: status = %d (%q), want %d", c.name, resp.StatusCode, env.Message, c.want)
		}
	}

	// Creating a user straight into the other hospital is refused too.
	resp, env := doJSON(t, http.MethodPost, "/user", map[string]any{
		"wardId":   wardB,
		"username": e2eName("crosstenant"),
		"password": "TestPass123",
		"role":     "USER",
	}, login.Token)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("ADMIN create user in other hospital: status = %d (%q), want 400", resp.StatusCode, env.Message)
	}
}
