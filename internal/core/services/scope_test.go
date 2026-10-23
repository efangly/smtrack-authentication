package services

import (
	"context"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func actor(role domain.Role, tenantID string) *domain.Actor {
	return &domain.Actor{ID: "C1", Role: role, TenantID: tenantID}
}

func actorIn(role domain.Role, tenantID string, departmentIDs ...string) *domain.Actor {
	return &domain.Actor{ID: "C1", Role: role, TenantID: tenantID, DepartmentIDs: departmentIDs}
}

// --- users ---

func TestUserService_FindOne_TenantScope(t *testing.T) {
	tests := []struct {
		name       string
		caller     *domain.Actor
		id         string
		userTenant string
		wantErr    string
	}{
		{"SUPER reads any tenant", actor(domain.RoleSuper, ""), "U2", "T2", ""},
		{"SERVICE reads any tenant", actor(domain.RoleService, ""), "U2", "T2", ""},
		{"ADMIN reads own tenant", actor(domain.RoleAdmin, "T1"), "U2", "T1", ""},
		{"ADMIN cannot read other tenant", actor(domain.RoleAdmin, "T1"), "U2", "T2", "user not found"},
		{"LEGACY_ADMIN cannot read other tenant", actor(domain.RoleLegacyAdmin, "T1"), "U2", "T2", "user not found"},
		{"ADMIN without tenant reads nobody", actor(domain.RoleAdmin, ""), "U2", "", "user not found"},
		{"USER cannot read another user", actor(domain.RoleUser, "T1"), "U2", "T1", "user not found"},
		{"USER reads self", actor(domain.RoleUser, "T1"), "C1", "T9", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newTestUserService(&mockUserRepo{FindByIDFn: findUserIn(domain.RoleUser, tt.userTenant)})

			_, err := svc.FindOne(context.Background(), tt.id, tt.caller)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestUserService_FindOne_RequiresCaller(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{FindByIDFn: findUser(domain.RoleUser)})
	_, err := svc.FindOne(context.Background(), "U1", nil)
	assert.EqualError(t, err, "unauthorized")
}

func TestUserService_UpdateAndRemove_TenantScope(t *testing.T) {
	tests := []struct {
		name       string
		caller     *domain.Actor
		userTenant string
		wantErr    string
	}{
		{"ADMIN own tenant", actor(domain.RoleAdmin, "T1"), "T1", ""},
		{"ADMIN other tenant", actor(domain.RoleAdmin, "T1"), "T2", "user not found"},
		{"LEGACY_ADMIN other tenant", actor(domain.RoleLegacyAdmin, "T1"), "T2", "user not found"},
		{"SERVICE other tenant", actor(domain.RoleService, "T1"), "T2", ""},
		{"SUPER other tenant", actor(domain.RoleSuper, ""), "T2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrote := false
			repo := &mockUserRepo{
				FindByIDFn: findUserIn(domain.RoleUser, tt.userTenant),
				UpdateFn: func(_ context.Context, id string, _ domain.UserPatch) (*domain.User, error) {
					wrote = true
					return &domain.User{ID: id}, nil
				},
				DeleteFn: func(_ context.Context, id, _ string) (*domain.User, error) {
					wrote = true
					return &domain.User{ID: id}, nil
				},
			}
			svc, _, _ := newTestUserService(repo)

			_, updErr := svc.Update(context.Background(), "U2", domain.UserPatch{Display: ptr("x")}, nil, tt.caller)
			_, rmErr := svc.Remove(context.Background(), "U2", tt.caller)
			if tt.wantErr == "" {
				assert.NoError(t, updErr)
				assert.NoError(t, rmErr)
			} else {
				assert.EqualError(t, updErr, tt.wantErr)
				assert.EqualError(t, rmErr, tt.wantErr)
				assert.False(t, wrote, "repository must not be written")
			}
		})
	}
}

// --- tenants ---

func TestTenantService_FindOne_Scope(t *testing.T) {
	tests := []struct {
		name     string
		caller   *domain.Actor
		tenantID string
		hidden   bool
		want     bool
	}{
		{"SUPER sees a hidden tenant", actor(domain.RoleSuper, ""), "T9", true, true},
		{"SERVICE sees other", actor(domain.RoleService, ""), "T2", false, true},
		{"SERVICE does not see a hidden tenant", actor(domain.RoleService, ""), "T9", true, false},
		{"ADMIN sees own", actor(domain.RoleAdmin, "T1"), "T1", false, true},
		{"ADMIN does not see other", actor(domain.RoleAdmin, "T1"), "T2", false, false},
		{"ADMIN does not see its own hidden tenant", actor(domain.RoleAdmin, "T9"), "T9", true, false},
		{"ADMIN without tenant sees nothing", actor(domain.RoleAdmin, ""), "", false, false},
		{"USER sees own tenant", actor(domain.RoleUser, "T1"), "T1", false, true},
		{"USER does not see other", actor(domain.RoleUser, "T1"), "T2", false, false},
		{"GUEST without tenant sees nothing", actor(domain.RoleGuest, ""), "T1", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockTenantRepo{FindByIDFn: func(_ context.Context, id string) (*domain.Tenant, error) {
				return &domain.Tenant{ID: id, Hidden: tt.hidden}, nil
			}}
			svc, _, _, _ := newTestTenantService(repo)

			_, err := svc.FindOne(context.Background(), tt.tenantID, tt.caller)
			if tt.want {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, "tenant not found")
			}
		})
	}
}

func TestTenantService_FindOne_RequiresCaller(t *testing.T) {
	svc, _, _, _ := newTestTenantService(&mockTenantRepo{})
	_, err := svc.FindOne(context.Background(), "T1", nil)
	assert.EqualError(t, err, "unauthorized")
}

// A user sees only the departments it is linked to; an admin sees all of its tenant's.
func TestTenantService_FindOne_NarrowsDepartments(t *testing.T) {
	repo := &mockTenantRepo{FindByIDFn: func(_ context.Context, id string) (*domain.Tenant, error) {
		return &domain.Tenant{ID: id, Departments: []domain.Department{{ID: "D1"}, {ID: "D2"}}}, nil
	}}
	svc, _, _, _ := newTestTenantService(repo)

	tenant, err := svc.FindOne(context.Background(), "T1", actorIn(domain.RoleUser, "T1", "D2"))
	require.NoError(t, err)
	require.Len(t, tenant.Departments, 1)
	assert.Equal(t, "D2", tenant.Departments[0].ID)

	tenant, err = svc.FindOne(context.Background(), "T1", actorIn(domain.RoleUser, "T1"))
	require.NoError(t, err)
	assert.Empty(t, tenant.Departments)

	tenant, err = svc.FindOne(context.Background(), "T1", actor(domain.RoleAdmin, "T1"))
	require.NoError(t, err)
	assert.Len(t, tenant.Departments, 2)
}

// --- departments ---

func TestDepartmentService_FindOne_Scope(t *testing.T) {
	tests := []struct {
		name   string
		caller *domain.Actor
		hidden bool
		tenant string
		want   bool
	}{
		{"SUPER sees a hidden tenant's", actor(domain.RoleSuper, ""), true, "T9", true},
		{"SERVICE does not see a hidden tenant's", actor(domain.RoleService, ""), true, "T9", false},
		{"SERVICE sees other", actor(domain.RoleService, ""), false, "T2", true},
		{"ADMIN sees every department of own tenant", actor(domain.RoleAdmin, "T1"), false, "T1", true},
		{"ADMIN does not see other tenant's", actor(domain.RoleAdmin, "T1"), false, "T2", false},
		{"LEGACY_ADMIN does not see other tenant's", actor(domain.RoleLegacyAdmin, "T1"), false, "T2", false},
		{"USER sees a linked department", actorIn(domain.RoleUser, "T1", "D9"), false, "T1", true},
		{"USER does not see an unlinked department", actorIn(domain.RoleUser, "T1", "D1"), false, "T1", false},
		{"USER does not see another tenant's department", actorIn(domain.RoleUser, "T1", "D9"), false, "T2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockDepartmentRepo{FindByIDFn: func(_ context.Context, id string) (*domain.Department, error) {
				return &domain.Department{ID: id, TenantID: tt.tenant, Tenant: &domain.Tenant{ID: tt.tenant, Hidden: tt.hidden}}, nil
			}}
			svc, _, _ := newTestDepartmentService(repo)

			_, err := svc.FindOne(context.Background(), "D9", tt.caller)
			if tt.want {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, "department not found")
			}
		})
	}
}

func TestDepartmentService_FindOne_RequiresCaller(t *testing.T) {
	svc, _, _ := newTestDepartmentService(&mockDepartmentRepo{})
	_, err := svc.FindOne(context.Background(), "D1", nil)
	assert.EqualError(t, err, "unauthorized")
}
