package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
// field names, nulls and omitted fields — so a change to a response DTO cannot
// reach clients without failing here. The domain types carry no tags, so what
// is pinned is dto/mapping.go.
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

func goldenTenant() *domain.Tenant {
	return &domain.Tenant{
		ID:          "TID-1",
		Seq:         7,
		Name:        "Acme Plant",
		Kind:        domain.TenantKindFactory,
		Address:     ptr("1 Main Rd"),
		Tel:         ptr("021234567"),
		ContactName: ptr("Contact Person"),
		ContactTel:  nil,
		Latitude:    ptr("13.7563"),
		Pic:         nil,
		// Only the department label is the tenant's own; the tenant label
		// falls back to the default for its kind.
		Labels:    domain.Labels{"department": {"th": "สายการผลิต", "en": "Line"}},
		CreatedAt: goldenTime,
		UpdatedAt: goldenTime,
		Departments: []domain.Department{{
			ID: "DID-1", Seq: 1, TenantID: "TID-1", Name: "Line A",
			CreatedAt: goldenTime, UpdatedAt: goldenTime,
		}},
	}
}

func goldenDepartment() *domain.Department {
	t := goldenTenant()
	t.Departments = nil
	return &domain.Department{
		ID: "DID-1", Seq: 1, TenantID: "TID-1", Name: "Line A", Legacy: true,
		CreatedAt: goldenTime, UpdatedAt: goldenTime,
		Tenant: t,
	}
}

func goldenUser() *domain.User {
	t := goldenTenant()
	t.Departments = nil
	return &domain.User{
		ID:          "UID-1",
		TenantID:    "TID-1",
		Username:    "jdoe",
		Password:    "$2a$10$this-hash-must-never-appear-in-a-response",
		Status:      true,
		Role:        domain.RoleUser,
		Display:     ptr("John Doe"),
		Pic:         nil,
		Comment:     ptr("night shift"),
		CreateBy:    nil,
		LastLoginAt: &goldenTime,
		CreatedAt:   goldenTime,
		UpdatedAt:   goldenTime,
		Tenant:      t,
		Departments: []domain.Department{{
			ID: "DID-1", Seq: 1, TenantID: "TID-1", Name: "Line A",
			CreatedAt: goldenTime, UpdatedAt: goldenTime,
		}},
	}
}

func goldenAuditLog() domain.AuditLog {
	return domain.AuditLog{
		ID: "AID-1", Entity: domain.AuditEntityUser, EntityID: "UID-1",
		Action: domain.AuditActionUpdate, ActorID: "UID-0", ActorName: "Admin",
		Changes: `{"display":"John Doe"}`, TenantID: "TID-1", CreatedAt: goldenTime,
	}
}

var (
	goldenSuper = &domain.Actor{ID: "UID-0", Name: "Root", Role: "SUPER", TenantID: "TID-1", DepartmentIDs: []string{"DID-1"}}
	goldenGuest = &domain.Actor{ID: "UID-9", Name: "Guest", Role: "GUEST", TenantID: "TID-1", DepartmentIDs: []string{"DID-1"}}

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
	tenant := goldenTenant()
	department := goldenDepartment()

	okAuth := &mockAuthService{
		RegisterFn: func(_ context.Context, _ *domain.User, _ *domain.File) (*domain.User, error) {
			u := goldenUser()
			u.Role = domain.RoleGuest
			u.Tenant, u.Departments = nil, nil
			return u, nil
		},
		ValidateUserFn: func(_ context.Context, _, _ string) (*domain.User, error) { return goldenUser(), nil },
		LoginFn: func(_ context.Context, _ *domain.User) (*ports.LoginResult, error) {
			return &ports.LoginResult{
				Token: "access.jwt", RefreshToken: "refresh.jwt", ID: "UID-1", Name: ptr("John Doe"),
				TenantID: "TID-1", DepartmentIDs: []string{"DID-1"}, Role: "USER", Pic: nil,
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
		RegisterFn: func(_ context.Context, _ *domain.User, _ *domain.File) (*domain.User, error) {
			return nil, errDomain
		},
		ValidateUserFn: func(_ context.Context, _, _ string) (*domain.User, error) { return nil, nil },
		RefreshTokensFn: func(_ context.Context, _ string) (*ports.RefreshResult, error) {
			return nil, errors.New("user is no longer active")
		},
	}
	brokenAuth := &mockAuthService{
		RegisterFn: func(_ context.Context, _ *domain.User, _ *domain.File) (*domain.User, error) {
			return nil, errInternal
		},
	}

	okUsers := &mockUserService{
		CreateFn:  func(_ context.Context, _ *domain.User, _ *domain.Actor) (*domain.User, error) { return user, nil },
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.User, error) { return userList, nil },
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.User, error) { return user, nil },
		UpdateFn: func(_ context.Context, _ string, _ domain.UserPatch, _ *domain.File, _ *domain.Actor) (*domain.User, error) {
			return user, nil
		},
		RemoveFn:  func(_ context.Context, _ string, _ *domain.Actor) (*domain.User, error) { return user, nil },
		RestoreFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.User, error) { return user, nil },
	}
	emptyUsers := &mockUserService{
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.User, error) { return []domain.User{}, nil },
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.User, error) {
			return nil, errors.New("user not found")
		},
	}

	okTenants := &mockTenantService{
		CreateFn: func(_ context.Context, _ *domain.Tenant, _ *domain.File, _ *domain.Actor) (*domain.Tenant, error) {
			return tenant, nil
		},
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.Tenant, error) {
			return []domain.Tenant{*goldenTenant()}, nil
		},
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Tenant, error) { return tenant, nil },
		UpdateFn: func(_ context.Context, _ string, _ domain.TenantPatch, _ *domain.File, _ *domain.Actor) (*domain.Tenant, error) {
			return tenant, nil
		},
		UpdateLabelsFn: func(_ context.Context, _ string, _ domain.Labels, _ *domain.Actor) (*domain.Tenant, error) {
			return tenant, nil
		},
		RemoveFn:  func(_ context.Context, _ string, _ *domain.Actor) (*domain.Tenant, error) { return tenant, nil },
		RestoreFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Tenant, error) { return tenant, nil },
	}

	okDepartments := &mockDepartmentService{
		CreateFn: func(_ context.Context, _ *domain.Department, _ *domain.Actor) (*domain.Department, error) {
			return department, nil
		},
		FindAllFn: func(_ context.Context, _ *domain.Actor) ([]domain.Department, error) {
			return []domain.Department{*goldenDepartment()}, nil
		},
		FindOneFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Department, error) { return department, nil },
		UpdateFn: func(_ context.Context, _ string, _ domain.DepartmentPatch, _ *domain.Actor) (*domain.Department, error) {
			return department, nil
		},
		RemoveFn:  func(_ context.Context, _ string, _ *domain.Actor) (*domain.Department, error) { return department, nil },
		RestoreFn: func(_ context.Context, _ string, _ *domain.Actor) (*domain.Department, error) { return department, nil },
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
	te := func(pick func(*TenantHandler) fiber.Handler) func() fiber.Handler {
		return func() fiber.Handler { return pick(NewTenantHandler(okTenants)) }
	}
	de := func(pick func(*DepartmentHandler) fiber.Handler) func() fiber.Handler {
		return func() fiber.Handler { return pick(NewDepartmentHandler(okDepartments)) }
	}

	register := `{"tenantId":"TID-1","username":"jdoe","password":"secret12"}`
	newUser := `{"tenantId":"TID-1","departmentIds":["DID-1"],"username":"jdoe","password":"secret12","role":"USER"}`

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
		{name: "user_restore_ok", method: http.MethodPost, route: "/user/:id/restore", target: "/user/UID-1/restore",
			caller: goldenSuper, h: u(okUsers, func(h *UserHandler) fiber.Handler { return h.Restore })},

		// tenants
		{name: "tenant_create_ok", method: http.MethodPost, route: "/tenant", target: "/tenant", body: `{"name":"Acme Plant","kind":"FACTORY"}`,
			caller: goldenSuper, h: te(func(h *TenantHandler) fiber.Handler { return h.Create })},
		{name: "tenant_create_validation", method: http.MethodPost, route: "/tenant", target: "/tenant", body: `{"kind":"FACTORY"}`,
			caller: goldenSuper, h: te(func(h *TenantHandler) fiber.Handler { return h.Create })},
		{name: "tenant_findall_ok", method: http.MethodGet, route: "/tenant", target: "/tenant",
			caller: goldenSuper, h: te(func(h *TenantHandler) fiber.Handler { return h.FindAll })},
		{name: "tenant_findone_ok", method: http.MethodGet, route: "/tenant/:id", target: "/tenant/TID-1",
			caller: goldenSuper, h: te(func(h *TenantHandler) fiber.Handler { return h.FindOne })},
		{name: "tenant_update_ok", method: http.MethodPut, route: "/tenant/:id", target: "/tenant/TID-1", body: `{"name":"Acme Plant"}`,
			caller: goldenSuper, h: te(func(h *TenantHandler) fiber.Handler { return h.Update })},
		{name: "tenant_labels_ok", method: http.MethodPut, route: "/tenant/:id/labels", target: "/tenant/TID-1/labels",
			body:   `{"labels":{"department":{"th":"สายการผลิต","en":"Line"}}}`,
			caller: goldenSuper, h: te(func(h *TenantHandler) fiber.Handler { return h.UpdateLabels })},
		{name: "tenant_labels_missing", method: http.MethodPut, route: "/tenant/:id/labels", target: "/tenant/TID-1/labels", body: `{}`,
			caller: goldenSuper, h: te(func(h *TenantHandler) fiber.Handler { return h.UpdateLabels })},
		{name: "tenant_remove_ok", method: http.MethodDelete, route: "/tenant/:id", target: "/tenant/TID-1",
			caller: goldenSuper, h: te(func(h *TenantHandler) fiber.Handler { return h.Remove })},
		{name: "tenant_restore_ok", method: http.MethodPost, route: "/tenant/:id/restore", target: "/tenant/TID-1/restore",
			caller: goldenSuper, h: te(func(h *TenantHandler) fiber.Handler { return h.Restore })},

		// departments
		{name: "department_create_ok", method: http.MethodPost, route: "/department", target: "/department", body: `{"name":"Line A","tenantId":"TID-1"}`,
			caller: goldenSuper, h: de(func(h *DepartmentHandler) fiber.Handler { return h.Create })},
		{name: "department_findall_ok", method: http.MethodGet, route: "/department", target: "/department",
			caller: goldenSuper, h: de(func(h *DepartmentHandler) fiber.Handler { return h.FindAll })},
		{name: "department_findone_ok", method: http.MethodGet, route: "/department/:id", target: "/department/DID-1",
			caller: goldenSuper, h: de(func(h *DepartmentHandler) fiber.Handler { return h.FindOne })},
		{name: "department_update_ok", method: http.MethodPut, route: "/department/:id", target: "/department/DID-1", body: `{"name":"Line A"}`,
			caller: goldenSuper, h: de(func(h *DepartmentHandler) fiber.Handler { return h.Update })},
		{name: "department_remove_ok", method: http.MethodDelete, route: "/department/:id", target: "/department/DID-1",
			caller: goldenSuper, h: de(func(h *DepartmentHandler) fiber.Handler { return h.Remove })},
		{name: "department_restore_ok", method: http.MethodPost, route: "/department/:id/restore", target: "/department/DID-1/restore",
			caller: goldenSuper, h: de(func(h *DepartmentHandler) fiber.Handler { return h.Restore })},

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
