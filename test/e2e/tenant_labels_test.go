//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type tenantView struct {
	ID     string                       `json:"id"`
	Kind   string                       `json:"kind"`
	Labels map[string]map[string]string `json:"labels"`
}

func getTenant(t *testing.T, token, id string) tenantView {
	t.Helper()
	resp, env := doJSON(t, http.MethodGet, "/tenant/"+id, nil, token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get tenant %s: status %d, message %q", id, resp.StatusCode, env.Message)
	}
	var tn tenantView
	if err := json.Unmarshal(env.Data, &tn); err != nil {
		t.Fatalf("decode tenant: %v", err)
	}
	return tn
}

// TestLabels_DefaultsFollowTenantKind verifies a tenant that never set a
// label profile still gets sensible names for its kind.
func TestLabels_DefaultsFollowTenantKind(t *testing.T) {
	admin := adminToken(t)
	cases := []struct{ kind, tenantEN, deptEN string }{
		{"HOSPITAL", "Hospital", "Ward"},
		{"LAB", "Laboratory", "Department"},
		{"FACTORY", "Factory", "Department"},
	}
	for _, c := range cases {
		tn := getTenant(t, admin, createTenant(t, admin, c.kind))
		if got := tn.Labels["tenant"]["en"]; got != c.tenantEN {
			t.Errorf("%s tenant label = %q, want %q", c.kind, got, c.tenantEN)
		}
		if got := tn.Labels["department"]["en"]; got != c.deptEN {
			t.Errorf("%s department label = %q, want %q", c.kind, got, c.deptEN)
		}
		if tn.Labels["tenant"]["th"] == "" || tn.Labels["department"]["th"] == "" {
			t.Errorf("%s defaults are missing the Thai text: %v", c.kind, tn.Labels)
		}
	}
}

// TestLabels_TenantAdminMaySetOwnLabels_AndValidationHolds covers who may edit
// a label profile and what it must contain.
func TestLabels_TenantAdminMaySetOwnLabels_AndValidationHolds(t *testing.T) {
	root := adminToken(t)
	tenantID := createTenant(t, root, "FACTORY")
	otherTenant := createTenant(t, root, "FACTORY")
	admin := createAdmin(t, root, tenantID)
	login := loginAs(t, admin.Username, admin.Password)

	set := func(token, id string, labels map[string]any) (*http.Response, envelope) {
		return doJSON(t, http.MethodPut, "/tenant/"+id+"/labels", map[string]any{"labels": labels}, token)
	}

	resp, env := set(login.Token, tenantID, map[string]any{
		"tenant":     map[string]string{"th": "โรงงาน A", "en": "Plant A"},
		"department": map[string]string{"en": "Line"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tenant admin sets own labels: status %d, message %q", resp.StatusCode, env.Message)
	}
	tn := getTenant(t, login.Token, tenantID)
	if tn.Labels["tenant"]["th"] != "โรงงาน A" || tn.Labels["department"]["en"] != "Line" {
		t.Errorf("labels after edit = %v", tn.Labels)
	}

	// Another tenant's labels are out of reach.
	if resp, _ := set(login.Token, otherTenant, map[string]any{"tenant": map[string]string{"en": "Mine"}}); resp.StatusCode == http.StatusOK {
		t.Error("a tenant admin changed another tenant's labels")
	}

	invalid := map[string]map[string]any{
		"unknown key":      {"colour": map[string]string{"en": "Red"}},
		"no th or en text": {"tenant": map[string]string{"fr": "Usine"}},
		"too long":         {"tenant": map[string]string{"en": strings.Repeat("x", 101)}},
		"empty text":       {"tenant": map[string]string{"en": ""}},
	}
	for name, labels := range invalid {
		if resp, _ := set(root, tenantID, labels); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, resp.StatusCode)
		}
	}

	// The edit is audited against the tenant.
	_, logs := doJSON(t, http.MethodGet, "/audit?entity=TENANT&entityId="+tenantID, nil, root)
	if !strings.Contains(string(logs.Data), "UPDATE") {
		t.Errorf("no UPDATE audit entry for the label change: %s", logs.Data)
	}
}
