package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/validator"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// The golden tests pin the exact JSON every endpoint writes — status, envelope,
// field names, nulls and omitted fields — so refactors that move serialization
// out of the domain types (see docs/refactor/hexagonal-plan.md, Phase 6) cannot
// change what clients receive without failing here.
//
// Regenerate after an *intended* response change with:
//
//	go test ./internal/adapters/driving/http/handlers/ -run TestGolden -update
var updateGolden = flag.Bool("update", false, "rewrite testdata/golden files")

// ── Fixtures ───────────────────────────────────────────────────────
// Every optional field is set somewhere and left nil somewhere else, so both
// the value and the null/omitted form are pinned.

var goldenTime = time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func goldenHospital() *domain.Hospital {
	return &domain.Hospital{
		ID:          "HID-1",
		HosName:     "General Hospital",
		HosSeq:      7,
		HosAddress:  ptr("1 Main Rd"),
		HosTel:      ptr("021234567"),
		UserContact: ptr("Contact Person"),
		UserTel:     nil,
		HosLatitude: ptr("13.7563"),
		HosPic:      nil,
		HosType:     domain.HosTypeHospital,
		CreatedAt:   goldenTime,
		UpdatedAt:   goldenTime,
		Wards: []domain.Ward{{
			ID: "WID-1", WardName: "Ward A", WardSeq: 1, Type: ptr(domain.WardTypeNew),
			HosID: "HID-1", CreatedAt: goldenTime, UpdatedAt: goldenTime,
		}},
	}
}

func goldenWard() *domain.Ward {
	h := goldenHospital()
	h.Wards = nil
	return &domain.Ward{
		ID: "WID-1", WardName: "Ward A", WardSeq: 1, Type: ptr(domain.WardTypeNew),
		HosID: "HID-1", CreatedAt: goldenTime, UpdatedAt: goldenTime,
		Hospital: h,
	}
}

func goldenUser() *domain.User {
	w := goldenWard()
	return &domain.User{
		ID:        "UID-1",
		WardID:    "WID-1",
		Username:  "jdoe",
		Password:  "$2a$10$this-hash-must-never-appear-in-a-response",
		Status:    true,
		Role:      domain.RoleUser,
		Display:   ptr("John Doe"),
		Pic:       nil,
		Comment:   ptr("night shift"),
		CreateBy:  nil,
		CreatedAt: goldenTime,
		UpdatedAt: goldenTime,
		Ward:      w,
	}
}

func goldenAuditLog() domain.AuditLog {
	return domain.AuditLog{
		ID: "AID-1", Entity: domain.AuditEntityUser, EntityID: "UID-1",
		Action: domain.AuditActionUpdate, ActorID: "UID-0", ActorName: "Admin",
		Changes: `{"display":"John Doe"}`, CreatedAt: goldenTime,
	}
}

var (
	goldenSuper = &domain.Actor{ID: "UID-0", Name: "Root", Role: "SUPER", HosID: "HID-1", WardID: "WID-1"}
	goldenGuest = &domain.Actor{ID: "UID-9", Name: "Guest", Role: "GUEST", HosID: "HID-1", WardID: "WID-1"}

	errDomain   = errors.New("username already exists")
	errInternal = fmt.Errorf("database error: %w", errors.New(`pq: relation "Users" does not exist`))
)

// ── Cases ──────────────────────────────────────────────────────────

type goldenCase struct {
	name   string
	method string
	route  string // pattern registered on the app
	target string // request path
	body   string
	caller *domain.Actor // nil = no authenticated caller
	h      func() fiber.Handler
}

func goldenCases() []goldenCase {
	user := goldenUser()
	userList := []domain.User{*goldenUser()}
	hospital := goldenHospital()
	ward := goldenWard()

	okAuth := &mockAuthService{
		RegisterFn: func(_ context.Context, _ *domain.User, _ *multipart.FileHeader) (*domain.User, error) {
			u := goldenUser()
			u.Role = domain.RoleGuest
			u.Ward = nil
			return u, nil
		},
		ValidateUserFn: func(_ context.Context, _, _ string) (*domain.User, error) { return goldenUser(), nil },
		LoginFn: func(_ context.Context, _ *domain.User) (*ports.LoginResult, error) {
			return &ports.LoginResult{
				Token: "access.jwt", RefreshToken: "refresh.jwt", ID: "UID-1", Name: ptr("John Doe"),
				HosID: "HID-1", WardID: "WID-1", Role: "USER", Pic: nil,
			}, nil
		},
		RefreshTokensFn: func(_ context.Context, _ string) (*ports.RefreshResult, error) {
			return &ports.RefreshResult{Token: "access2.jwt", RefreshToken: "refresh2.jwt"}, nil
		},
		ResetPasswordFn: func(_ context.Context, _, _, _ string, _ *domain.Actor) (string, error) {
			return "Password reset successfully", nil
		},
	}
	failAuth := &mockAuthService{
		RegisterFn: func(_ context.Context, _ *domain.User, _ *multipart.FileHeader) (*domain.User, error) {
			return nil, errDomain
		},
		ValidateUserFn: func(_ context.Context, _, _ string) (*domain.User, error) { return nil, nil },
		RefreshTokensFn: func(_ context.Context, _ string) (*ports.RefreshResult, error) {
			return nil, errors.New("user is no longer active")
		},
	}
	brokenAuth := &mockAuthService{
		RegisterFn: func(_ context.Context, _ *domain.User, _ *multipart.FileHeader) (*domain.User, error) {
			return nil, errInternal
		},
	}

	okUsers := &mockUserService{
		CreateFn:  func(_ context.Context, _ *domain.User, _ *domain.Actor) (*domain.User, error) { return user, nil },
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.User, error) { return userList, nil },
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.User, error) { return user, nil },
		UpdateFn: func(_ context.Context, _ string, _ map[string]any, _ *multipart.FileHeader, _ *domain.Actor) (*domain.User, error) {
			return user, nil
		},
		RemoveFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.User, error) { return user, nil },
	}
	emptyUsers := &mockUserService{
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.User, error) { return []domain.User{}, nil },
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.User, error) {
			return nil, errors.New("user not found")
		},
	}

	okHospitals := &mockHospitalService{
		CreateFn: func(_ context.Context, _ *domain.Hospital, _ *multipart.FileHeader, _ *domain.Actor) (*domain.Hospital, error) {
			return hospital, nil
		},
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.Hospital, error) {
			return []domain.Hospital{*goldenHospital()}, nil
		},
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Hospital, error) { return hospital, nil },
		UpdateFn: func(_ context.Context, _ string, _ map[string]any, _ *multipart.FileHeader, _ *domain.Actor) (*domain.Hospital, error) {
			return hospital, nil
		},
		RemoveFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Hospital, error) { return hospital, nil },
	}

	okWards := &mockWardService{
		CreateFn: func(_ context.Context, _ *domain.Ward, _ *domain.Actor) (*domain.Ward, error) { return ward, nil },
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.Ward, error) {
			return []domain.Ward{*goldenWard()}, nil
		},
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Ward, error) { return ward, nil },
		UpdateFn: func(_ context.Context, _ string, _ map[string]any, _ *domain.Actor) (*domain.Ward, error) {
			return ward, nil
		},
		RemoveFn: func(_ context.Context, _ string, _ *domain.Actor) (string, error) {
			return "Ward deleted successfully", nil
		},
	}

	okAudit := &mockAuditService{
		FindAllFn: func(_ context.Context, _ ports.AuditFilter) ([]domain.AuditLog, error) {
			return []domain.AuditLog{goldenAuditLog()}, nil
		},
	}

	a := func(svc *mockAuthService, pick func(*AuthHandler) fiber.Handler) func() fiber.Handler {
		return func() fiber.Handler { return pick(NewAuthHandler(svc)) }
	}
	u := func(svc *mockUserService, pick func(*UserHandler) fiber.Handler) func() fiber.Handler {
		return func() fiber.Handler { return pick(NewUserHandler(svc)) }
	}
	ho := func(pick func(*HospitalHandler) fiber.Handler) func() fiber.Handler {
		return func() fiber.Handler { return pick(NewHospitalHandler(okHospitals)) }
	}
	w := func(pick func(*WardHandler) fiber.Handler) func() fiber.Handler {
		return func() fiber.Handler { return pick(NewWardHandler(okWards)) }
	}

	register := `{"wardId":"WID-1","username":"jdoe","password":"secret12"}`
	newUser := `{"wardId":"WID-1","username":"jdoe","password":"secret12","role":"USER"}`

	return []goldenCase{
		{name: "health", method: http.MethodGet, route: "/health", target: "/health",
			h: func() fiber.Handler { return NewHealthHandler().Check }},

		// auth
		{name: "auth_register_ok", method: http.MethodPost, route: "/register", target: "/register", body: register,
			h: a(okAuth, func(h *AuthHandler) fiber.Handler { return h.Register })},
		{name: "auth_register_validation", method: http.MethodPost, route: "/register", target: "/register", body: `{"username":"x"}`,
			h: a(okAuth, func(h *AuthHandler) fiber.Handler { return h.Register })},
		{name: "auth_register_malformed_json", method: http.MethodPost, route: "/register", target: "/register", body: `{`,
			h: a(okAuth, func(h *AuthHandler) fiber.Handler { return h.Register })},
		{name: "auth_register_domain_error", method: http.MethodPost, route: "/register", target: "/register", body: register,
			h: a(failAuth, func(h *AuthHandler) fiber.Handler { return h.Register })},
		{name: "auth_register_internal_error", method: http.MethodPost, route: "/register", target: "/register", body: register,
			h: a(brokenAuth, func(h *AuthHandler) fiber.Handler { return h.Register })},
		{name: "auth_login_ok", method: http.MethodPost, route: "/login", target: "/login", body: `{"username":"jdoe","password":"secret12"}`,
			h: a(okAuth, func(h *AuthHandler) fiber.Handler { return h.Login })},
		{name: "auth_login_bad_credentials", method: http.MethodPost, route: "/login", target: "/login", body: `{"username":"jdoe","password":"wrong123"}`,
			h: a(failAuth, func(h *AuthHandler) fiber.Handler { return h.Login })},
		{name: "auth_refresh_ok", method: http.MethodPost, route: "/refresh", target: "/refresh", body: `{"token":"refresh.jwt"}`,
			h: a(okAuth, func(h *AuthHandler) fiber.Handler { return h.RefreshToken })},
		{name: "auth_refresh_missing_token", method: http.MethodPost, route: "/refresh", target: "/refresh", body: `{}`,
			h: a(okAuth, func(h *AuthHandler) fiber.Handler { return h.RefreshToken })},
		{name: "auth_refresh_rejected", method: http.MethodPost, route: "/refresh", target: "/refresh", body: `{"token":"refresh.jwt"}`,
			h: a(failAuth, func(h *AuthHandler) fiber.Handler { return h.RefreshToken })},
		{name: "auth_reset_ok", method: http.MethodPatch, route: "/reset/:username", target: "/reset/jdoe", body: `{"password":"newpass1","oldPassword":"secret12"}`,
			caller: goldenSuper, h: a(okAuth, func(h *AuthHandler) fiber.Handler { return h.ResetPassword })},
		{name: "auth_reset_no_caller", method: http.MethodPatch, route: "/reset/:username", target: "/reset/jdoe", body: `{"password":"newpass1"}`,
			h: a(okAuth, func(h *AuthHandler) fiber.Handler { return h.ResetPassword })},

		// users
		{name: "user_create_ok", method: http.MethodPost, route: "/user", target: "/user", body: newUser,
			caller: goldenSuper, h: u(okUsers, func(h *UserHandler) fiber.Handler { return h.Create })},
		{name: "user_findall_ok", method: http.MethodGet, route: "/user", target: "/user",
			caller: goldenSuper, h: u(okUsers, func(h *UserHandler) fiber.Handler { return h.FindAll })},
		{name: "user_findall_empty", method: http.MethodGet, route: "/user", target: "/user",
			caller: goldenSuper, h: u(emptyUsers, func(h *UserHandler) fiber.Handler { return h.FindAll })},
		{name: "user_findone_ok", method: http.MethodGet, route: "/user/:id", target: "/user/UID-1",
			caller: goldenSuper, h: u(okUsers, func(h *UserHandler) fiber.Handler { return h.FindOne })},
		{name: "user_findone_not_found", method: http.MethodGet, route: "/user/:id", target: "/user/UID-404",
			caller: goldenSuper, h: u(emptyUsers, func(h *UserHandler) fiber.Handler { return h.FindOne })},
		{name: "user_findone_forbidden", method: http.MethodGet, route: "/user/:id", target: "/user/UID-1",
			caller: goldenGuest, h: u(okUsers, func(h *UserHandler) fiber.Handler { return h.FindOne })},
		{name: "user_findone_no_caller", method: http.MethodGet, route: "/user/:id", target: "/user/UID-1",
			h: u(okUsers, func(h *UserHandler) fiber.Handler { return h.FindOne })},
		{name: "user_update_ok", method: http.MethodPut, route: "/user/:id", target: "/user/UID-1", body: `{"display":"John Doe"}`,
			caller: goldenSuper, h: u(okUsers, func(h *UserHandler) fiber.Handler { return h.Update })},
		{name: "user_remove_ok", method: http.MethodDelete, route: "/user/:id", target: "/user/UID-1",
			caller: goldenSuper, h: u(okUsers, func(h *UserHandler) fiber.Handler { return h.Remove })},

		// hospitals
		{name: "hospital_create_ok", method: http.MethodPost, route: "/hospital", target: "/hospital", body: `{"hosName":"General Hospital"}`,
			caller: goldenSuper, h: ho(func(h *HospitalHandler) fiber.Handler { return h.Create })},
		{name: "hospital_findall_ok", method: http.MethodGet, route: "/hospital", target: "/hospital",
			caller: goldenSuper, h: ho(func(h *HospitalHandler) fiber.Handler { return h.FindAll })},
		{name: "hospital_findone_ok", method: http.MethodGet, route: "/hospital/:id", target: "/hospital/HID-1",
			caller: goldenSuper, h: ho(func(h *HospitalHandler) fiber.Handler { return h.FindOne })},
		{name: "hospital_update_ok", method: http.MethodPut, route: "/hospital/:id", target: "/hospital/HID-1", body: `{"hosName":"General Hospital"}`,
			caller: goldenSuper, h: ho(func(h *HospitalHandler) fiber.Handler { return h.Update })},
		{name: "hospital_remove_ok", method: http.MethodDelete, route: "/hospital/:id", target: "/hospital/HID-1",
			caller: goldenSuper, h: ho(func(h *HospitalHandler) fiber.Handler { return h.Remove })},

		// wards
		{name: "ward_create_ok", method: http.MethodPost, route: "/ward", target: "/ward", body: `{"wardName":"Ward A","hosId":"HID-1"}`,
			caller: goldenSuper, h: w(func(h *WardHandler) fiber.Handler { return h.Create })},
		{name: "ward_findall_ok", method: http.MethodGet, route: "/ward", target: "/ward",
			caller: goldenSuper, h: w(func(h *WardHandler) fiber.Handler { return h.FindAll })},
		{name: "ward_findone_ok", method: http.MethodGet, route: "/ward/:id", target: "/ward/WID-1",
			caller: goldenSuper, h: w(func(h *WardHandler) fiber.Handler { return h.FindOne })},
		{name: "ward_update_ok", method: http.MethodPut, route: "/ward/:id", target: "/ward/WID-1", body: `{"wardName":"Ward A"}`,
			caller: goldenSuper, h: w(func(h *WardHandler) fiber.Handler { return h.Update })},
		{name: "ward_remove_ok", method: http.MethodDelete, route: "/ward/:id", target: "/ward/WID-1",
			caller: goldenSuper, h: w(func(h *WardHandler) fiber.Handler { return h.Remove })},

		// audit
		{name: "audit_findall_ok", method: http.MethodGet, route: "/audit", target: "/audit?entity=USER&limit=10",
			caller: goldenSuper, h: func() fiber.Handler { return NewAuditHandler(okAudit).FindAll }},
	}
}

// ── Runner ─────────────────────────────────────────────────────────

type goldenRecord struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

func TestGolden(t *testing.T) {
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			// Same validator and error handler wiring as main.go.
			app := fiber.New(fiber.Config{StructValidator: validator.Validator{}})
			if tc.caller != nil {
				app.Add([]string{tc.method}, tc.route, injectCaller(tc.caller), tc.h())
			} else {
				app.Add([]string{tc.method}, tc.route, tc.h())
			}

			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.target, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.NotContains(t, string(raw), "this-hash-must-never-appear", "password hash leaked into a response")

			var indented bytes.Buffer
			require.NoError(t, json.Indent(&indented, raw, "", "  "), "response is not JSON: %s", raw)
			got, err := json.MarshalIndent(goldenRecord{Status: resp.StatusCode, Body: indented.Bytes()}, "", "  ")
			require.NoError(t, err)
			got = append(got, '\n')

			path := filepath.Join("testdata", "golden", tc.name+".json")
			if *updateGolden {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, got, 0o644))
				return
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err, "missing golden file; run with -update to create it")
			require.Equal(t, string(want), string(got),
				"response for %s changed; if intended, rerun with -update and review the diff", tc.name)
		})
	}
}
