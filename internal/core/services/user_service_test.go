package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tng-coop/auth-service/internal/core/domain"
	"github.com/tng-coop/auth-service/internal/core/ports"
)

func newTestUserService(repo *mockUserRepo) (ports.UserService, *mockCache, *mockUpload) {
	cache := newMockCache()
	upload := newMockUpload()
	svc := NewUserService(repo, cache, upload)
	return svc, cache, upload
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
	})
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
	})
	require.NoError(t, err)
	assert.Equal(t, "CUSTOM-ID", user.ID)
}

func TestUserService_Create_RepoError(t *testing.T) {
	repo := &mockUserRepo{
		CreateFn: func(_ context.Context, _ *domain.User) (*domain.User, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Create(context.Background(), &domain.User{Username: "u1"})
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
			return &domain.User{ID: id, Username: "user1"}, nil
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
		UpdateFn: func(_ context.Context, _ string, _ map[string]any) (*domain.User, error) {
			return &domain.User{ID: "U1", Username: "updated"}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.Update(context.Background(), "U1", map[string]any{"username": "updated"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "updated", user.Username)
}

func TestUserService_Update_RepoError(t *testing.T) {
	repo := &mockUserRepo{
		UpdateFn: func(_ context.Context, _ string, _ map[string]any) (*domain.User, error) {
			return nil, errors.New("update failed")
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "U1", map[string]any{}, nil)
	assert.EqualError(t, err, "update failed")
}

// --- Remove ---

func TestUserService_Remove_Success(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "user1"}, nil
		},
		DeleteFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "user1"}, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	user, err := svc.Remove(context.Background(), "U1")
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

	_, err := svc.Remove(context.Background(), "nonexistent")
	assert.EqualError(t, err, "user not found")
}

func TestUserService_Remove_WithPic(t *testing.T) {
	pic := "http://example.com/user/photo.jpg"
	deleteCalledWith := ""
	repo := &mockUserRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "user1", Pic: &pic}, nil
		},
		DeleteFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "user1", Pic: &pic}, nil
		},
	}
	svc, _, upload := newTestUserService(repo)
	upload.DeleteFn = func(path, filename string) error {
		deleteCalledWith = filename
		return nil
	}

	_, err := svc.Remove(context.Background(), "U1")
	require.NoError(t, err)
	assert.Equal(t, "photo.jpg", deleteCalledWith)
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
