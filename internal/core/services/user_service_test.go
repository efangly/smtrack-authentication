package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fixture world: tenants T1 and T2 exist, HIDDEN is a hidden tenant,
// departments D1 and D2 belong to T1 and D3 to T2.

// testPassword is an ordinary valid password.
const testPassword = "correct-horse-battery"

func tenantsExist(_ context.Context, id string) (*domain.Tenant, error) {
	switch id {
	case "T1", "T2", "HIDDEN":
		return &domain.Tenant{ID: id, Hidden: id == "HIDDEN"}, nil
	}
	return nil, nil
}

func departmentsExist(_ context.Context, ids []string) ([]domain.Department, error) {
	owner := map[string]string{"D1": "T1", "D2": "T1", "D3": "T2"}
	var out []domain.Department
	for _, id := range ids {
		if tenant, ok := owner[id]; ok {
			out = append(out, domain.Department{ID: id, TenantID: tenant})
		}
	}
	return out, nil
}

type userFixture struct {
	svc     ports.UserService
	cache   *mockCache
	upload  *mockUpload
	tenants *mockTenantRepo
	depts   *mockDepartmentRepo
	audit   *mockAuditService
}

func newUserFixture(repo *mockUserRepo) *userFixture {
	f := &userFixture{
		cache:   newMockCache(),
		upload:  newMockUpload(),
		tenants: &mockTenantRepo{FindByIDFn: tenantsExist},
		depts:   &mockDepartmentRepo{FindByIDsFn: departmentsExist},
		audit:   newMockAuditService(),
	}
	f.svc = NewUserService(repo, f.tenants, f.depts, f.cache, f.upload, &fakeHasher{}, f.audit)
	return f
}

func newTestUserService(repo *mockUserRepo) (ports.UserService, *mockCache, *mockUpload) {
	f := newUserFixture(repo)
	return f.svc, f.cache, f.upload
}

// superCaller is an actor that passes every role-ceiling check, for tests
// that are about something else.
var superCaller = &domain.Actor{ID: "SUPER-1", Role: domain.RoleSuper}

// findUser returns a user with role in tenant T1.
func findUser(role domain.Role) func(context.Context, string) (*domain.User, error) {
	return findUserIn(role, "T1")
}

func findUserIn(role domain.Role, tenantID string) func(context.Context, string) (*domain.User, error) {
	return func(_ context.Context, id string) (*domain.User, error) {
		return &domain.User{
			ID: id, Username: "user1", Role: role, TenantID: tenantID,
			Departments: []domain.Department{{ID: "D1", TenantID: tenantID}},
		}, nil
	}
}

func echoUser(_ context.Context, user *domain.User) (*domain.User, error) { return user, nil }

// --- Create ---

func TestUserService_Create_Success(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{CreateFn: echoUser})

	user, err := svc.Create(context.Background(), &domain.User{Password: testPassword, Username: "TestUser", TenantID: "T1"}, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)                // uuid generated
	assert.Equal(t, "testuser", user.Username) // lowercased
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
}

func TestUserService_Create_PreservesID(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{CreateFn: echoUser})

	user, err := svc.Create(context.Background(), &domain.User{Password: testPassword, ID: "CUSTOM-ID", Username: "User1", TenantID: "T1"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "CUSTOM-ID", user.ID)
}

func TestUserService_Create_HashesPassword(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{CreateFn: echoUser})

	user, err := svc.Create(context.Background(), &domain.User{Username: "TestUser", Password: "pass1234-long-enough", TenantID: "T1"}, nil)
	require.NoError(t, err)
	// Create is the single place that hashes on this path (Register hashes
	// nothing itself — it relies on this), so every caller must see a hash,
	// never the plain value it sent in.
	assert.Equal(t, fakeHash("pass1234-long-enough"), user.Password)
}

func TestUserService_Create_RepoError(t *testing.T) {
	repo := &mockUserRepo{CreateFn: func(context.Context, *domain.User) (*domain.User, error) {
		return nil, errors.New("db error")
	}}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Create(context.Background(), &domain.User{Password: testPassword, Username: "u1", TenantID: "T1"}, nil)
	assert.EqualError(t, err, "db error")
}

func TestUserService_Create_UsernameTaken(t *testing.T) {
	repo := &mockUserRepo{
		CreateFn:        func(context.Context, *domain.User) (*domain.User, error) { t.Fatal("must not create"); return nil, nil },
		UsernameTakenFn: func(_ context.Context, username string) (bool, error) { return username == "taken", nil },
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Create(context.Background(), &domain.User{Password: testPassword, Username: "TAKEN", TenantID: "T1"}, nil)
	assert.EqualError(t, err, "username already exists")
}

// --- Create: tenant and departments ---

func TestUserService_Create_TenantMustBeInScope(t *testing.T) {
	tests := []struct {
		name     string
		caller   *domain.Actor
		tenantID string
		wantErr  string
	}{
		{"public into a normal tenant", nil, "T1", ""},
		{"public into a hidden tenant", nil, "HIDDEN", "tenant not found"},
		{"public into a missing tenant", nil, "T9", "tenant not found"},
		{"ADMIN into own tenant", actor(domain.RoleAdmin, "T1"), "T1", ""},
		{"ADMIN into other tenant", actor(domain.RoleAdmin, "T1"), "T2", "tenant not found"},
		{"ADMIN into missing tenant", actor(domain.RoleAdmin, "T1"), "T9", "tenant not found"},
		{"SERVICE into any tenant", actor(domain.RoleService, "T1"), "T2", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newTestUserService(&mockUserRepo{CreateFn: echoUser})

			_, err := svc.Create(context.Background(), &domain.User{Password: testPassword, Username: "u1", Role: domain.RoleGuest, TenantID: tt.tenantID}, tt.caller)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestUserService_Create_DepartmentsMustBelongToTenant(t *testing.T) {
	tests := []struct {
		name    string
		ids     []string
		wantErr string
	}{
		{"none", nil, ""},
		{"own tenant's", []string{"D1", "D2"}, ""},
		{"another tenant's", []string{"D1", "D3"}, "department not found"},
		{"missing", []string{"D9"}, "department not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newTestUserService(&mockUserRepo{CreateFn: echoUser})

			_, err := svc.Create(context.Background(),
				&domain.User{Username: "u1", Password: testPassword, Role: domain.RoleUser, TenantID: "T1", DepartmentIDs: tt.ids}, actor(domain.RoleAdmin, "T1"))
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func TestUserService_Create_PublicRegistrationGetsNoDepartments(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{CreateFn: echoUser})

	_, err := svc.Create(context.Background(), &domain.User{Password: testPassword, Username: "u1", TenantID: "T1", DepartmentIDs: []string{"D1"}}, nil)
	assert.EqualError(t, err, "you cannot assign departments")
}

// --- FindAll / FindOne / FindByUsername ---

func TestUserService_FindAll_FromRepo(t *testing.T) {
	repo := &mockUserRepo{FindAllFn: func(context.Context, ports.UserFilter) ([]domain.User, error) {
		return []domain.User{{ID: "U1"}, {ID: "U2"}}, nil
	}}
	svc, _, _ := newTestUserService(repo)

	users, err := svc.FindAll(context.Background(), superCaller)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserService_FindAll_RepoError(t *testing.T) {
	repo := &mockUserRepo{FindAllFn: func(context.Context, ports.UserFilter) ([]domain.User, error) {
		return nil, errors.New("db error")
	}}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.FindAll(context.Background(), superCaller)
	assert.EqualError(t, err, "db error")
}

func TestUserService_FindAll_TenantScopedRoleWithoutTenantSeesNothing(t *testing.T) {
	repo := &mockUserRepo{FindAllFn: func(context.Context, ports.UserFilter) ([]domain.User, error) {
		t.Fatal("an empty scope must not reach the repository")
		return nil, nil
	}}
	svc, _, _ := newTestUserService(repo)

	for _, role := range []domain.Role{domain.RoleAdmin, domain.RoleLegacyAdmin, domain.RoleUser, domain.RoleGuest} {
		users, err := svc.FindAll(context.Background(), actor(role, ""))
		require.NoError(t, err, role)
		assert.Empty(t, users, role)
	}
}

func TestUserService_FindOne_Success(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{FindByIDFn: findUser(domain.RoleUser)})

	user, err := svc.FindOne(context.Background(), "U1", superCaller)
	require.NoError(t, err)
	assert.Equal(t, "U1", user.ID)
}

func TestUserService_FindOne_NotFound(t *testing.T) {
	repo := &mockUserRepo{FindByIDFn: func(context.Context, string) (*domain.User, error) { return nil, nil }}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.FindOne(context.Background(), "nonexistent", superCaller)
	assert.EqualError(t, err, "user not found")
}

func TestUserService_FindByUsername(t *testing.T) {
	repo := &mockUserRepo{FindByUsernameFn: func(_ context.Context, username string) (*domain.User, error) {
		if username == "nobody" {
			return nil, nil
		}
		return &domain.User{ID: "U1", Username: username}, nil
	}}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.FindByUsername(context.Background(), "admin")
	require.NoError(t, err)
	assert.Equal(t, "admin", user.Username)

	user, err = svc.FindByUsername(context.Background(), "nobody")
	require.NoError(t, err)
	assert.Nil(t, user)
}

// --- Update ---

func TestUserService_Update_Success(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser),
		UpdateFn: func(context.Context, string, domain.UserPatch) (*domain.User, error) {
			return &domain.User{ID: "U1", Username: "updated"}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.Update(context.Background(), "U1", domain.UserPatch{Username: ptr("updated")}, nil, superCaller)
	require.NoError(t, err)
	assert.Equal(t, "updated", user.Username)
}

func TestUserService_Update_RepoError(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser),
		UpdateFn: func(context.Context, string, domain.UserPatch) (*domain.User, error) {
			return nil, errors.New("update failed")
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "U1", domain.UserPatch{}, nil, superCaller)
	assert.EqualError(t, err, "update failed")
}

func TestUserService_Update_UsernameTaken(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn:      findUser(domain.RoleUser),
		UsernameTakenFn: func(context.Context, string) (bool, error) { return true, nil },
		UpdateFn: func(context.Context, string, domain.UserPatch) (*domain.User, error) {
			t.Fatal("repo.Update must not be reached")
			return nil, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "U1", domain.UserPatch{Username: ptr("other")}, nil, superCaller)
	assert.EqualError(t, err, "username already exists")

	// Resubmitting its own username is not a conflict.
	repo.UpdateFn = func(_ context.Context, id string, _ domain.UserPatch) (*domain.User, error) {
		return &domain.User{ID: id}, nil
	}
	_, err = svc.Update(context.Background(), "U1", domain.UserPatch{Username: ptr("user1")}, nil, superCaller)
	assert.NoError(t, err)
}

func TestUserService_Update_TenantMoveNeedsDepartments(t *testing.T) {
	var patched domain.UserPatch
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser), // in T1
		UpdateFn: func(_ context.Context, id string, p domain.UserPatch) (*domain.User, error) {
			patched = p
			return &domain.User{ID: id}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "U2", domain.UserPatch{TenantID: ptr("T2")}, nil, superCaller)
	assert.EqualError(t, err, "departmentIds is required when changing tenant")

	// The new departments must belong to the new tenant.
	_, err = svc.Update(context.Background(), "U2", domain.UserPatch{TenantID: ptr("T2"), DepartmentIDs: &[]string{"D1"}}, nil, superCaller)
	assert.EqualError(t, err, "department not found")

	_, err = svc.Update(context.Background(), "U2", domain.UserPatch{TenantID: ptr("T2"), DepartmentIDs: &[]string{"D3"}}, nil, superCaller)
	require.NoError(t, err)
	assert.Equal(t, "T2", *patched.TenantID)

	// Clearing every department is allowed.
	_, err = svc.Update(context.Background(), "U2", domain.UserPatch{DepartmentIDs: &[]string{}}, nil, superCaller)
	assert.NoError(t, err)
}

func TestUserService_Update_AdminCannotMoveUserToAnotherTenant(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser),
		UpdateFn: func(context.Context, string, domain.UserPatch) (*domain.User, error) {
			t.Fatal("repo.Update must not be reached")
			return nil, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "U2",
		domain.UserPatch{TenantID: ptr("T2"), DepartmentIDs: &[]string{"D3"}}, nil, actor(domain.RoleAdmin, "T1"))
	assert.EqualError(t, err, "tenant not found")
}

// --- Remove (soft delete) ---

func TestUserService_Remove_Success(t *testing.T) {
	var deletedBy string
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser),
		DeleteFn: func(_ context.Context, id, by string) (*domain.User, error) {
			deletedBy = by
			return &domain.User{ID: id, Username: "user1", Role: domain.RoleUser}, nil
		},
	}
	svc, cache, _ := newTestUserService(repo)

	user, err := svc.Remove(context.Background(), "U1", superCaller)
	require.NoError(t, err)
	assert.Equal(t, "U1", user.ID)
	assert.Equal(t, "SUPER-1", deletedBy, "the caller is recorded as the deleter")
	assert.Contains(t, cache.deleted, "user")
}

func TestUserService_Remove_NotFound(t *testing.T) {
	repo := &mockUserRepo{FindByIDFn: func(context.Context, string) (*domain.User, error) { return nil, nil }}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Remove(context.Background(), "nonexistent", superCaller)
	assert.EqualError(t, err, "user not found")
}

// A soft-deleted user keeps its picture, so restoring brings back the whole record.
func TestUserService_Remove_KeepsPicture(t *testing.T) {
	pic := "http://example.com/user/photo.jpg"
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser),
		DeleteFn: func(_ context.Context, id, _ string) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleUser, Pic: &pic}, nil
		},
	}
	svc, _, upload := newTestUserService(repo)
	upload.DeleteFn = func(string, string) error {
		t.Fatal("the image must not be deleted by a soft delete")
		return nil
	}

	_, err := svc.Remove(context.Background(), "U1", superCaller)
	require.NoError(t, err)
}

// --- Restore ---

func deletedUser(role domain.Role, tenantID string) func(context.Context, string) (*domain.User, error) {
	return func(_ context.Context, id string) (*domain.User, error) {
		now := time.Now()
		return &domain.User{ID: id, Role: role, TenantID: tenantID, DeletedAt: &now}, nil
	}
}

func TestUserService_Restore(t *testing.T) {
	tests := []struct {
		name    string
		caller  *domain.Actor
		target  func(context.Context, string) (*domain.User, error)
		wantErr string
	}{
		{"SUPER restores", superCaller, deletedUser(domain.RoleUser, "T1"), ""},
		{"ADMIN restores in own tenant", actor(domain.RoleAdmin, "T1"), deletedUser(domain.RoleUser, "T1"), ""},
		{"ADMIN cannot restore in another tenant", actor(domain.RoleAdmin, "T1"), deletedUser(domain.RoleUser, "T2"), "user not found"},
		{"ADMIN cannot restore above its ceiling", actor(domain.RoleAdmin, "T1"), deletedUser(domain.RoleAdmin, "T1"), "you do not have permission to modify this user"},
		{"a live user is not restorable", superCaller, findUser(domain.RoleUser), "user not found"},
		{"a missing user", superCaller, func(context.Context, string) (*domain.User, error) { return nil, nil }, "user not found"},
		{"a deleted tenant blocks it", superCaller, deletedUser(domain.RoleUser, "T9"), "tenant not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restored := false
			repo := &mockUserRepo{
				FindAnyByIDFn: tt.target,
				RestoreFn: func(_ context.Context, id string) (*domain.User, error) {
					restored = true
					return &domain.User{ID: id}, nil
				},
			}
			svc, _, _ := newTestUserService(repo)

			_, err := svc.Restore(context.Background(), "U2", tt.caller)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				assert.True(t, restored)
			} else {
				assert.EqualError(t, err, tt.wantErr)
				assert.False(t, restored)
			}
		})
	}
}

func TestUserService_Restore_RequiresCaller(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{})
	_, err := svc.Restore(context.Background(), "U1", nil)
	assert.EqualError(t, err, "unauthorized")
}

// --- Role ceiling ---

var allRoles = []domain.Role{
	domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin,
	domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest,
}

// assignable is the role ceiling: which target roles each caller role may
// assign, modify or delete.
var assignable = map[domain.Role][]domain.Role{
	domain.RoleSuper:       allRoles,
	domain.RoleService:     {domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin, domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest},
	domain.RoleAdmin:       {domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest},
	domain.RoleLegacyAdmin: {domain.RoleUser, domain.RoleLegacyUser, domain.RoleGuest},
}

// deniedChange is the error for a caller acting on another user in its own
// tenant: roles outside the admin tier have no tenant scope at all, so the
// user reads as not found; admin-tier callers hit the role ceiling instead.
func deniedChange(caller domain.Role) string {
	if (&domain.Actor{Role: caller}).IsAdminTier() {
		return "you do not have permission to modify this user"
	}
	return "user not found"
}

func canAssign(caller, target domain.Role) bool {
	for _, r := range assignable[caller] {
		if r == target {
			return true
		}
	}
	return false
}

func TestUserService_Create_RoleCeiling(t *testing.T) {
	for _, callerRole := range allRoles {
		for _, target := range allRoles {
			t.Run(string(callerRole)+"->"+string(target), func(t *testing.T) {
				created := false
				svc, _, _ := newTestUserService(&mockUserRepo{CreateFn: func(_ context.Context, user *domain.User) (*domain.User, error) {
					created = true
					return user, nil
				}})

				_, err := svc.Create(context.Background(), &domain.User{Password: testPassword, Username: "u1", Role: target, TenantID: "T1"}, actor(callerRole, "T1"))
				if canAssign(callerRole, target) {
					assert.NoError(t, err)
					assert.True(t, created)
				} else {
					assert.EqualError(t, err, "you cannot assign role "+string(target))
					assert.False(t, created, "repo.Create must not be reached")
				}
			})
		}
	}
}

func TestUserService_Create_PublicRegistrationOnlyGuest(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{CreateFn: echoUser})

	_, err := svc.Create(context.Background(), &domain.User{Password: testPassword, Username: "u1", Role: domain.RoleGuest, TenantID: "T1"}, nil)
	assert.NoError(t, err)

	_, err = svc.Create(context.Background(), &domain.User{Password: testPassword, Username: "u2", Role: domain.RoleSuper, TenantID: "T1"}, nil)
	assert.EqualError(t, err, "you cannot assign role SUPER")
}

func TestUserService_Update_RoleCeilingOnTarget(t *testing.T) {
	for _, callerRole := range allRoles {
		for _, targetRole := range allRoles {
			t.Run(string(callerRole)+"->"+string(targetRole), func(t *testing.T) {
				updated := false
				repo := &mockUserRepo{
					FindByIDFn: findUser(targetRole),
					UpdateFn: func(_ context.Context, id string, _ domain.UserPatch) (*domain.User, error) {
						updated = true
						return &domain.User{ID: id}, nil
					},
				}
				svc, _, _ := newTestUserService(repo)

				_, err := svc.Update(context.Background(), "OTHER", domain.UserPatch{Display: ptr("x")}, nil, actor(callerRole, "T1"))
				if canAssign(callerRole, targetRole) {
					assert.NoError(t, err)
					assert.True(t, updated)
				} else {
					assert.EqualError(t, err, deniedChange(callerRole))
					assert.False(t, updated, "repo.Update must not be reached")
				}
			})
		}
	}
}

func TestUserService_Update_RoleCeilingOnNewRole(t *testing.T) {
	for _, callerRole := range allRoles {
		for _, newRole := range allRoles {
			t.Run(string(callerRole)+"->"+string(newRole), func(t *testing.T) {
				// The target currently holds GUEST, which every admin tier may
				// act on, so only the requested new role decides the outcome.
				repo := &mockUserRepo{
					FindByIDFn: findUser(domain.RoleGuest),
					UpdateFn: func(_ context.Context, id string, _ domain.UserPatch) (*domain.User, error) {
						return &domain.User{ID: id}, nil
					},
				}
				svc, _, _ := newTestUserService(repo)

				_, err := svc.Update(context.Background(), "OTHER", domain.UserPatch{Role: ptr(newRole)}, nil, actor(callerRole, "T1"))
				switch {
				case !canAssign(callerRole, domain.RoleGuest):
					assert.EqualError(t, err, deniedChange(callerRole))
				case newRole == domain.RoleGuest || canAssign(callerRole, newRole):
					assert.NoError(t, err)
				default:
					assert.EqualError(t, err, "you cannot assign role "+string(newRole))
				}
			})
		}
	}
}

func TestUserService_Update_CannotChangeOwnRole(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleAdmin),
		UpdateFn: func(context.Context, string, domain.UserPatch) (*domain.User, error) {
			t.Fatal("repo.Update must not be reached")
			return nil, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "A1", domain.UserPatch{Role: ptr(domain.RoleSuper)}, nil, &domain.Actor{ID: "A1", Role: domain.RoleAdmin})
	assert.EqualError(t, err, "you cannot change your own role")

	// Even SUPER cannot change its own role.
	repo.FindByIDFn = findUser(domain.RoleSuper)
	_, err = svc.Update(context.Background(), "S1", domain.UserPatch{Role: ptr(domain.RoleGuest)}, nil, &domain.Actor{ID: "S1", Role: domain.RoleSuper})
	assert.EqualError(t, err, "you cannot change your own role")
}

func TestUserService_Update_UnchangedOwnRoleIsAllowed(t *testing.T) {
	// A profile form that resubmits the caller's current role must not fail.
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleAdmin),
		UpdateFn: func(_ context.Context, id string, _ domain.UserPatch) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "A1", domain.UserPatch{Role: ptr(domain.RoleAdmin), Display: ptr("me")}, nil, &domain.Actor{ID: "A1", Role: domain.RoleAdmin})
	assert.NoError(t, err)
}

func TestUserService_Update_SelfEditAllowedForAnyRole(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleGuest),
		UpdateFn: func(_ context.Context, id string, _ domain.UserPatch) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "G1", domain.UserPatch{Display: ptr("me")}, nil, &domain.Actor{ID: "G1", Role: domain.RoleGuest})
	assert.NoError(t, err)
}

func TestUserService_UpdateAndRemove_RequireCaller(t *testing.T) {
	svc, _, _ := newTestUserService(&mockUserRepo{FindByIDFn: findUser(domain.RoleGuest)})

	_, err := svc.Update(context.Background(), "U1", domain.UserPatch{}, nil, nil)
	assert.EqualError(t, err, "unauthorized")
	_, err = svc.Remove(context.Background(), "U1", nil)
	assert.EqualError(t, err, "unauthorized")
}

func TestUserService_Remove_RoleCeilingOnTarget(t *testing.T) {
	for _, callerRole := range allRoles {
		for _, targetRole := range allRoles {
			t.Run(string(callerRole)+"->"+string(targetRole), func(t *testing.T) {
				deleted := false
				repo := &mockUserRepo{
					FindByIDFn: findUser(targetRole),
					DeleteFn: func(_ context.Context, id, _ string) (*domain.User, error) {
						deleted = true
						return &domain.User{ID: id}, nil
					},
				}
				svc, _, _ := newTestUserService(repo)

				_, err := svc.Remove(context.Background(), "OTHER", actor(callerRole, "T1"))
				if canAssign(callerRole, targetRole) {
					assert.NoError(t, err)
					assert.True(t, deleted)
				} else {
					assert.EqualError(t, err, deniedChange(callerRole))
					assert.False(t, deleted, "repo.Delete must not be reached")
				}
			})
		}
	}
}

// --- buildUserFilter ---

func TestBuildUserFilter(t *testing.T) {
	tests := []struct {
		role       domain.Role
		tenantID   string
		wantFilter ports.UserFilter
		wantKey    string
		wantOK     bool
	}{
		{domain.RoleSuper, "", ports.UserFilter{}, "user", true},
		{domain.RoleService, "", ports.UserFilter{ExcludeHiddenTenants: true}, "user:service", true},
		{domain.RoleAdmin, "T1", ports.UserFilter{TenantID: "T1", ExcludeHiddenTenants: true, ExcludeRoles: []domain.Role{domain.RoleService}}, "user:tenant:T1", true},
		{domain.RoleLegacyAdmin, "T2", ports.UserFilter{TenantID: "T2", ExcludeHiddenTenants: true}, "user:tenant:T2:legacy", true},
		{domain.RoleUser, "T1", ports.UserFilter{TenantID: "T1"}, "user:tenant:T1:member", true},
		{domain.RoleAdmin, "", ports.UserFilter{}, "", false},
		{domain.RoleGuest, "", ports.UserFilter{}, "", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.role)+"/"+tt.tenantID, func(t *testing.T) {
			filter, key, ok := buildUserFilter(&domain.Actor{Role: tt.role, TenantID: tt.tenantID})
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantFilter, filter)
			assert.Equal(t, tt.wantKey, key)
		})
	}
}
