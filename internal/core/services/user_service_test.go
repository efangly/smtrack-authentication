package services

import (
	"context"
	"errors"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func newTestUserService(repo *mockUserRepo) (ports.UserService, *mockCache, *mockUpload) {
	cache := newMockCache()
	upload := newMockUpload()
	svc := NewUserService(repo, cache, upload, newMockAuditService())
	return svc, cache, upload
}

// superCaller is an actor that passes every role-ceiling check, for tests
// that are about something else.
var superCaller = &ports.JwtPayload{ID: "SUPER-1", Role: string(domain.RoleSuper)}

func findUser(role domain.Role) func(context.Context, string) (*domain.User, error) {
	return func(_ context.Context, id string) (*domain.User, error) {
		return &domain.User{ID: id, Username: "user1", Role: role}, nil
	}
}

// --- Create ---

func TestUserService_Create_Success(t *testing.T) {
	repo := &mockUserRepo{
		CreateFn: func(_ context.Context, user *domain.User) (*domain.User, error) {
			return user, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.Create(context.Background(), &domain.User{
		Username: "TestUser",
		WardID:   "W1",
	}, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)                // uuid generated
	assert.Equal(t, "testuser", user.Username) // lowercased
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
}

func TestUserService_Create_PreservesID(t *testing.T) {
	repo := &mockUserRepo{
		CreateFn: func(_ context.Context, user *domain.User) (*domain.User, error) {
			return user, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.Create(context.Background(), &domain.User{
		ID:       "CUSTOM-ID",
		Username: "User1",
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "CUSTOM-ID", user.ID)
}

func TestUserService_Create_HashesPassword(t *testing.T) {
	repo := &mockUserRepo{
		CreateFn: func(_ context.Context, user *domain.User) (*domain.User, error) {
			return user, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.Create(context.Background(), &domain.User{
		Username: "TestUser",
		Password: "pass1234",
		WardID:   "W1",
	}, nil)
	require.NoError(t, err)
	// Create is the single place that hashes on this path (Register hashes
	// nothing itself — it relies on this), so every caller must see a hash,
	// never the plain value it sent in.
	assert.NotEqual(t, "pass1234", user.Password)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("pass1234")))
}

func TestUserService_Create_RepoError(t *testing.T) {
	repo := &mockUserRepo{
		CreateFn: func(_ context.Context, _ *domain.User) (*domain.User, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Create(context.Background(), &domain.User{Username: "u1"}, nil)
	assert.EqualError(t, err, "db error")
}

// --- FindAll ---

func TestUserService_FindAll_FromRepo(t *testing.T) {
	repo := &mockUserRepo{
		FindAllFn: func(_ context.Context, _ map[string]any, _ []map[string]any) ([]domain.User, error) {
			return []domain.User{{ID: "U1"}, {ID: "U2"}}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)
	caller := &ports.JwtPayload{Role: "SUPER"}

	users, err := svc.FindAll(context.Background(), caller)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserService_FindAll_RepoError(t *testing.T) {
	repo := &mockUserRepo{
		FindAllFn: func(_ context.Context, _ map[string]any, _ []map[string]any) ([]domain.User, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.FindAll(context.Background(), &ports.JwtPayload{Role: "SUPER"})
	assert.EqualError(t, err, "db error")
}

// --- FindOne ---

func TestUserService_FindOne_Success(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "user1", Role: domain.RoleUser}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.FindOne(context.Background(), "U1")
	require.NoError(t, err)
	assert.Equal(t, "U1", user.ID)
}

func TestUserService_FindOne_NotFound(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.FindOne(context.Background(), "nonexistent")
	assert.EqualError(t, err, "user not found")
}

// --- FindByUsername ---

func TestUserService_FindByUsername_Found(t *testing.T) {
	repo := &mockUserRepo{
		FindByUsernameFn: func(_ context.Context, username string) (*domain.User, error) {
			return &domain.User{ID: "U1", Username: username}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.FindByUsername(context.Background(), "admin")
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "admin", user.Username)
}

func TestUserService_FindByUsername_NotFound(t *testing.T) {
	repo := &mockUserRepo{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.FindByUsername(context.Background(), "nobody")
	require.NoError(t, err)
	assert.Nil(t, user)
}

// --- Update ---

func TestUserService_Update_Success(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser),
		UpdateFn: func(_ context.Context, _ string, _ map[string]any) (*domain.User, error) {
			return &domain.User{ID: "U1", Username: "updated"}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.Update(context.Background(), "U1", map[string]any{"username": "updated"}, nil, superCaller)
	require.NoError(t, err)
	assert.Equal(t, "updated", user.Username)
}

func TestUserService_Update_RepoError(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser),
		UpdateFn: func(_ context.Context, _ string, _ map[string]any) (*domain.User, error) {
			return nil, errors.New("update failed")
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "U1", map[string]any{}, nil, superCaller)
	assert.EqualError(t, err, "update failed")
}

// --- Remove ---

func TestUserService_Remove_Success(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "user1", Role: domain.RoleUser}, nil
		},
		DeleteFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "user1", Role: domain.RoleUser}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.Remove(context.Background(), "U1", superCaller)
	require.NoError(t, err)
	assert.Equal(t, "U1", user.ID)
}

func TestUserService_Remove_NotFound(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Remove(context.Background(), "nonexistent", superCaller)
	assert.EqualError(t, err, "user not found")
}

func TestUserService_Remove_WithPic(t *testing.T) {
	pic := "http://example.com/user/photo.jpg"
	deleteCalledWith := ""
	repo := &mockUserRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "user1", Role: domain.RoleUser, Pic: &pic}, nil
		},
		DeleteFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "user1", Role: domain.RoleUser, Pic: &pic}, nil
		},
	}
	svc, _, upload := newTestUserService(repo)
	upload.DeleteFn = func(path, filename string) error {
		deleteCalledWith = filename
		return nil
	}

	_, err := svc.Remove(context.Background(), "U1", superCaller)
	require.NoError(t, err)
	assert.Equal(t, "photo.jpg", deleteCalledWith)
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
				repo := &mockUserRepo{
					CreateFn: func(_ context.Context, user *domain.User) (*domain.User, error) {
						created = true
						return user, nil
					},
				}
				svc, _, _ := newTestUserService(repo)
				caller := &ports.JwtPayload{ID: "C1", Role: string(callerRole)}

				_, err := svc.Create(context.Background(), &domain.User{Username: "u1", Role: target}, caller)
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
	repo := &mockUserRepo{
		CreateFn: func(_ context.Context, user *domain.User) (*domain.User, error) {
			return user, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Create(context.Background(), &domain.User{Username: "u1", Role: domain.RoleGuest}, nil)
	assert.NoError(t, err)

	_, err = svc.Create(context.Background(), &domain.User{Username: "u2", Role: domain.RoleSuper}, nil)
	assert.EqualError(t, err, "you cannot assign role SUPER")
}

func TestUserService_Update_RoleCeilingOnTarget(t *testing.T) {
	for _, callerRole := range allRoles {
		for _, targetRole := range allRoles {
			t.Run(string(callerRole)+"->"+string(targetRole), func(t *testing.T) {
				updated := false
				repo := &mockUserRepo{
					FindByIDFn: findUser(targetRole),
					UpdateFn: func(_ context.Context, id string, _ map[string]any) (*domain.User, error) {
						updated = true
						return &domain.User{ID: id}, nil
					},
				}
				svc, _, _ := newTestUserService(repo)
				caller := &ports.JwtPayload{ID: "C1", Role: string(callerRole)}

				_, err := svc.Update(context.Background(), "OTHER", map[string]any{"display": "x"}, nil, caller)
				if canAssign(callerRole, targetRole) {
					assert.NoError(t, err)
					assert.True(t, updated)
				} else {
					assert.EqualError(t, err, "you do not have permission to modify this user")
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
					UpdateFn: func(_ context.Context, id string, _ map[string]any) (*domain.User, error) {
						return &domain.User{ID: id}, nil
					},
				}
				svc, _, _ := newTestUserService(repo)
				caller := &ports.JwtPayload{ID: "C1", Role: string(callerRole)}

				_, err := svc.Update(context.Background(), "OTHER", map[string]any{"role": string(newRole)}, nil, caller)
				switch {
				case !canAssign(callerRole, domain.RoleGuest):
					assert.EqualError(t, err, "you do not have permission to modify this user")
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
		UpdateFn: func(_ context.Context, id string, _ map[string]any) (*domain.User, error) {
			t.Fatal("repo.Update must not be reached")
			return nil, nil
		},
	}
	svc, _, _ := newTestUserService(repo)
	self := &ports.JwtPayload{ID: "A1", Role: string(domain.RoleAdmin)}

	_, err := svc.Update(context.Background(), "A1", map[string]any{"role": "SUPER"}, nil, self)
	assert.EqualError(t, err, "you cannot change your own role")

	// Even SUPER cannot change its own role.
	repo.FindByIDFn = findUser(domain.RoleSuper)
	super := &ports.JwtPayload{ID: "S1", Role: string(domain.RoleSuper)}
	_, err = svc.Update(context.Background(), "S1", map[string]any{"role": "GUEST"}, nil, super)
	assert.EqualError(t, err, "you cannot change your own role")
}

func TestUserService_Update_UnchangedOwnRoleIsAllowed(t *testing.T) {
	// A profile form that resubmits the caller's current role must not fail.
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleAdmin),
		UpdateFn: func(_ context.Context, id string, _ map[string]any) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)
	self := &ports.JwtPayload{ID: "A1", Role: string(domain.RoleAdmin)}

	_, err := svc.Update(context.Background(), "A1", map[string]any{"role": "ADMIN", "display": "me"}, nil, self)
	assert.NoError(t, err)
}

func TestUserService_Update_SelfEditAllowedForAnyRole(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleGuest),
		UpdateFn: func(_ context.Context, id string, _ map[string]any) (*domain.User, error) {
			return &domain.User{ID: id}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)
	self := &ports.JwtPayload{ID: "G1", Role: string(domain.RoleGuest)}

	_, err := svc.Update(context.Background(), "G1", map[string]any{"display": "me"}, nil, self)
	assert.NoError(t, err)
}

func TestUserService_UpdateAndRemove_RequireCaller(t *testing.T) {
	repo := &mockUserRepo{FindByIDFn: findUser(domain.RoleGuest)}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "U1", map[string]any{}, nil, nil)
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
					DeleteFn: func(_ context.Context, id string) (*domain.User, error) {
						deleted = true
						return &domain.User{ID: id}, nil
					},
				}
				svc, _, _ := newTestUserService(repo)
				caller := &ports.JwtPayload{ID: "C1", Role: string(callerRole)}

				_, err := svc.Remove(context.Background(), "OTHER", caller)
				if canAssign(callerRole, targetRole) {
					assert.NoError(t, err)
					assert.True(t, deleted)
				} else {
					assert.EqualError(t, err, "you do not have permission to modify this user")
					assert.False(t, deleted, "repo.Delete must not be reached")
				}
			})
		}
	}
}

// --- buildUserConditions ---

func TestBuildUserConditions(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		hosID          string
		expectConds    bool
		expectNotConds int
		expectKey      string
	}{
		{"SUPER", "SUPER", "", false, 0, "user"},
		{"ADMIN", "ADMIN", "H1", true, 2, "user:H1"},
		{"LEGACY_ADMIN", "LEGACY_ADMIN", "H2", true, 1, "user:H2"},
		{"SERVICE", "SERVICE", "", false, 1, "user:HID-DEVELOPMENT"},
		{"USER default", "USER", "", false, 0, "user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &ports.JwtPayload{Role: tt.role, HosID: tt.hosID}
			conds, notConds, key := buildUserConditions(caller)
			if tt.expectConds {
				assert.NotNil(t, conds)
			} else {
				assert.Nil(t, conds)
			}
			assert.Len(t, notConds, tt.expectNotConds)
			assert.Equal(t, tt.expectKey, key)
		})
	}
}
