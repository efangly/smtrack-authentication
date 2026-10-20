//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

type userListEntry struct {
	ID   string `json:"id"`
	Ward *struct {
		WardName string `json:"wardName"`
		HosID    string `json:"hosId"`
		Hospital *struct {
			HosName string `json:"hosName"`
		} `json:"hospital,omitempty"`
	} `json:"ward,omitempty"`
}

func fetchUserListEntry(t *testing.T, callerToken, userID string) userListEntry {
	t.Helper()
	resp, env := doJSON(t, http.MethodGet, "/user", nil, callerToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list users: status %d, message %q", resp.StatusCode, env.Message)
	}
	var users []userListEntry
	if err := json.Unmarshal(env.Data, &users); err != nil {
		t.Fatalf("decode user list: %v", err)
	}
	for _, u := range users {
		if u.ID == userID {
			return u
		}
	}
	t.Fatalf("user %s not found in listing", userID)
	return userListEntry{}
}

// TestCacheCascade_WardRename_InvalidatesUserListing is the regression check
// CLAUDE.md calls out: "Anything that changes a ward or hospital must also
// invalidate `user`, because user listings preload their ward." Before this
// round's fix, renaming a ward left the cached /auth/user listing showing
// the old ward name.
func TestCacheCascade_WardRename_InvalidatesUserListing(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	user := createUser(t, admin, wardID)

	before := fetchUserListEntry(t, admin, user.ID) // populates the user-list cache
	if before.Ward == nil {
		t.Fatal("expected the listing to embed the user's ward")
	}

	newWardName := e2eName("renamed-ward")
	updResp, updEnv := doJSON(t, http.MethodPut, "/ward/"+wardID, map[string]any{
		"wardName": newWardName,
	}, admin)
	if updResp.StatusCode != http.StatusOK {
		t.Fatalf("rename ward: status %d, message %q", updResp.StatusCode, updEnv.Message)
	}

	after := fetchUserListEntry(t, admin, user.ID)
	if after.Ward == nil {
		t.Fatal("expected the listing to embed the user's ward after rename")
	}
	if after.Ward.WardName != newWardName {
		t.Errorf("user listing shows ward name %q, want %q (stale user cache after ward update)",
			after.Ward.WardName, newWardName)
	}
}

// TestCacheCascade_HospitalRename_InvalidatesUserDetail checks the same
// cascade one level further out: a hospital-name change should also bust the
// single-user cache, since GET /auth/user/:id preloads ward -> hospital.
// (The list endpoint only preloads "Ward", not "Ward.Hospital", so this uses
// the detail endpoint instead of the listing used above.)
func TestCacheCascade_HospitalRename_InvalidatesUserDetail(t *testing.T) {
	admin := adminToken(t)
	hosID := createHospital(t, admin)
	wardID := createWard(t, admin, hosID)
	user := createUser(t, admin, wardID)

	fetchDetail := func() userListEntry {
		resp, env := doJSON(t, http.MethodGet, "/user/"+user.ID, nil, admin)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("get user detail: status %d, message %q", resp.StatusCode, env.Message)
		}
		var detail userListEntry
		if err := json.Unmarshal(env.Data, &detail); err != nil {
			t.Fatalf("decode user detail: %v", err)
		}
		return detail
	}

	before := fetchDetail() // populates the user:<id> cache
	if before.Ward == nil || before.Ward.Hospital == nil {
		t.Fatal("expected user detail to embed ward -> hospital")
	}

	newHosName := e2eName("renamed-hospital")
	updResp, updEnv := doJSON(t, http.MethodPut, "/hospital/"+hosID, map[string]any{
		"hosName": newHosName,
	}, admin)
	if updResp.StatusCode != http.StatusOK {
		t.Fatalf("rename hospital: status %d, message %q", updResp.StatusCode, updEnv.Message)
	}

	after := fetchDetail()
	if after.Ward == nil || after.Ward.Hospital == nil {
		t.Fatal("expected user detail to embed ward -> hospital after rename")
	}
	if after.Ward.Hospital.HosName != newHosName {
		t.Errorf("user detail shows hospital name %q, want %q (stale user cache after hospital update)",
			after.Ward.Hospital.HosName, newHosName)
	}
}
