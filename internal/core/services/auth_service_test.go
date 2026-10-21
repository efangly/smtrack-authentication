package services

import (
	"context"
	"errors"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAuthService(userSvc *mockUserService) (ports.AuthService, *mockCache, *mockUpload) {
	svc, cache, upload, _, _ := newTestAuthServiceFull(userSvc)
	return svc, cache, upload
}

func newTestAuthServiceFull(userSvc *mockUserService) (ports.AuthService, *mockCache, *mockUpload, *fakeTokens, *fakeHasher) {
	cache := newMockCache()
	upload := newMockUpload()
	tokens := &fakeTokens{}
	hasher := &fakeHasher{}
	svc := NewAuthService(userSvc, cache, upload, tokens, hasher)
	return svc, cache, upload, tokens, hasher
}

// --- Register ---

func TestAuthService_Register_Success(t *testing.T) {
	var passwordSeenByCreate string
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil // no existing user
		},
		CreateFn: func(_ context.Context, user *domain.User, _ *domain.Actor) (*domain.User, error) {
			passwordSeenByCreate = user.Password
			return user, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	user, err := svc.Register(context.Background(), &domain.User{
		Username: "TestUser",
		Password: "pass1234",
		WardID:   "W1",
	}, nil)

	require.NoError(t, err)
	assert.NotNil(t, user)
	// Register itself no longer hashes — userService.Create is the single
	// place that does, so the plain password must reach Create unchanged.
	assert.Equal(t, "pass1234", passwordSeenByCreate)
}

func TestAuthService_Register_DuplicateUsername(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "existing"}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.Register(context.Background(), &domain.User{
		Username: "existinguser",
		Password: "pass1234",
	}, nil)

	assert.EqualError(t, err, "username already exists")
}

func TestAuthService_Register_WithFile(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
		CreateFn: func(_ context.Context, user *domain.User, _ *domain.Actor) (*domain.User, error) {
			return user, nil
		},
	}
	svc, _, upload := newTestAuthService(userSvc)
	upload.UploadFn = func(_ *domain.File, _ string) (string, error) {
		return "http://cdn.example.com/user/pic.jpg", nil
	}

	user, err := svc.Register(context.Background(), &domain.User{
		Username: "newuser",
		Password: "pass1234",
	}, &domain.File{Name: "pic.jpg"})

	require.NoError(t, err)
	assert.NotNil(t, user.Pic)
	assert.Equal(t, "http://cdn.example.com/user/pic.jpg", *user.Pic)
}

// --- ValidateUser ---

func TestAuthService_ValidateUser_Success(t *testing.T) {
	hashed := fakeHash("correct")
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: hashed, Status: true}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	user, err := svc.ValidateUser(context.Background(), "admin", "correct")
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "U1", user.ID)
}

func TestAuthService_ValidateUser_WrongPassword(t *testing.T) {
	hashed := fakeHash("correct")
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: hashed}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	user, err := svc.ValidateUser(context.Background(), "admin", "wrong")
	require.NoError(t, err)
	assert.Nil(t, user)
}

func TestAuthService_ValidateUser_UserNotFound(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
	}
	svc, _, _, _, hasher := newTestAuthServiceFull(userSvc)

	user, err := svc.ValidateUser(context.Background(), "nobody", "pass")
	require.NoError(t, err)
	assert.Nil(t, user)
	// An unknown username must still cost a hash comparison.
	assert.Equal(t, 1, hasher.dummyCalls)
}

func TestAuthService_ValidateUser_DisabledAccount(t *testing.T) {
	hashed := fakeHash("correct")
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: hashed, Status: false}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	// Same outcome as a wrong password: the handler turns nil, nil into
	// "Invalid username or password".
	user, err := svc.ValidateUser(context.Background(), "admin", "correct")
	require.NoError(t, err)
	assert.Nil(t, user)
}

// --- Login ---

func TestAuthService_Login_Success(t *testing.T) {
	svc, _, _, tokens, _ := newTestAuthServiceFull(&mockUserService{})

	display := "Admin"
	user := &domain.User{
		ID:      "U1",
		Role:    domain.RoleAdmin,
		WardID:  "W1",
		Ward:    &domain.Ward{HosID: "H1"},
		Display: &display,
	}

	result, err := svc.Login(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, "access:U1:ADMIN", result.Token)
	assert.Equal(t, "refresh:U1", result.RefreshToken)
	assert.Equal(t, "U1", result.ID)
	assert.Equal(t, domain.RoleAdmin, result.Role)
	assert.Equal(t, "H1", result.HosID)
	assert.Equal(t, "W1", result.WardID)

	// The tokens carry the user's identity, with the hospital taken from the ward.
	require.Len(t, tokens.issued, 1)
	assert.Equal(t, domain.Actor{ID: "U1", Name: "Admin", Role: domain.RoleAdmin, HosID: "H1", WardID: "W1"}, tokens.issued[0])
}

// --- RefreshTokens ---

func TestAuthService_RefreshTokens_Success(t *testing.T) {
	userSvc := &mockUserService{
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{
				ID:     id,
				Role:   domain.RoleAdmin,
				WardID: "W1",
				Status: true,
				Ward:   &domain.Ward{HosID: "H1"},
			}, nil
		},
	}
	svc, _, _, tokens, _ := newTestAuthServiceFull(userSvc)
	tokens.refreshUserID = "U1"

	result, err := svc.RefreshTokens(context.Background(), "refresh-token")
	require.NoError(t, err)
	assert.Equal(t, "access:U1:ADMIN", result.Token)
	assert.Equal(t, "refresh:U1", result.RefreshToken)
}

func TestAuthService_RefreshTokens_InvalidToken(t *testing.T) {
	svc, _, _, tokens, _ := newTestAuthServiceFull(&mockUserService{})
	tokens.refreshErr = errors.New("invalid or expired refresh token")

	_, err := svc.RefreshTokens(context.Background(), "invalid-token")
	// Passed through unwrapped, so the HTTP layer reports it as a 401 rather
	// than hiding it behind a generic 500.
	assert.EqualError(t, err, "invalid or expired refresh token")
	assert.Nil(t, errors.Unwrap(err))
}

// --- ResetPassword ---

func TestAuthService_ResetPassword_SuperRole(t *testing.T) {
	hashed := fakeHash("oldpass")
	updated := false
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Username: "user1", Password: hashed}, nil
		},
		UpdateFn: func(_ context.Context, _ string, _ domain.UserPatch, _ *domain.File, _ *domain.Actor) (*domain.User, error) {
			updated = true
			return &domain.User{ID: "U1"}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	msg, err := svc.ResetPassword(context.Background(), "user1", "newpass", "", &domain.Actor{Role: "SUPER"})
	require.NoError(t, err)
	assert.Equal(t, "Password reset successfully", msg)
	assert.True(t, updated)
}

func TestAuthService_ResetPassword_NonSuperRequiresOldPassword(t *testing.T) {
	hashed := fakeHash("oldpass")
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: hashed}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.ResetPassword(context.Background(), "user1", "newpass", "", &domain.Actor{Role: "ADMIN"})
	assert.EqualError(t, err, "old password is required")
}

func TestAuthService_ResetPassword_WrongOldPassword(t *testing.T) {
	hashed := fakeHash("oldpass")
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: hashed}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.ResetPassword(context.Background(), "user1", "newpass", "wrongold", &domain.Actor{Role: "ADMIN"})
	assert.EqualError(t, err, "old password is incorrect")
}

func TestAuthService_ResetPassword_UserNotFound(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.ResetPassword(context.Background(), "nobody", "newpass", "", &domain.Actor{Role: "SUPER"})
	assert.EqualError(t, err, "user not found")
}

// --- Register error from FindByUsername ---

func TestAuthService_Register_FindByUsernameError(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.Register(context.Background(), &domain.User{Username: "u1", Password: "pass"}, nil)
	assert.EqualError(t, err, "db error")
}

func TestAuthService_RefreshTokens_ReflectsCurrentRole(t *testing.T) {
	// Token was issued while the user was SUPER; the record now says GUEST.
	userSvc := &mockUserService{
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleGuest, Status: true}, nil
		},
	}
	svc, _, _, tokens, _ := newTestAuthServiceFull(userSvc)
	tokens.refreshUserID = "U1"

	_, err := svc.RefreshTokens(context.Background(), "refresh-token")
	require.NoError(t, err)
	require.Len(t, tokens.issued, 1)
	assert.Equal(t, domain.RoleGuest, tokens.issued[0].Role)
}

func TestAuthService_RefreshTokens_InactiveUser(t *testing.T) {
	userSvc := &mockUserService{
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Role: domain.RoleGuest, Status: false}, nil
		},
	}
	svc, _, _, tokens, _ := newTestAuthServiceFull(userSvc)
	tokens.refreshUserID = "U1"

	_, err := svc.RefreshTokens(context.Background(), "refresh-token")
	assert.EqualError(t, err, "user is no longer active")
	assert.Empty(t, tokens.issued)
}

func TestAuthService_ResetPassword_RejectsOtherUsersAccount(t *testing.T) {
	hashed := fakeHash("oldpass")
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "VICTIM", Password: hashed}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	_, err := svc.ResetPassword(context.Background(), "victim", "newpass", "oldpass",
		&domain.Actor{ID: "ATTACKER", Role: "GUEST"})
	assert.EqualError(t, err, "you can only reset your own password")
}

func TestAuthService_ResetPassword_OwnAccountAllowed(t *testing.T) {
	hashed := fakeHash("oldpass")
	userSvc := &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Password: hashed}, nil
		},
		UpdateFn: func(_ context.Context, _ string, data domain.UserPatch, _ *domain.File, _ *domain.Actor) (*domain.User, error) {
			return &domain.User{ID: "U1"}, nil
		},
	}
	svc, _, _ := newTestAuthService(userSvc)

	msg, err := svc.ResetPassword(context.Background(), "user1", "newpass", "oldpass",
		&domain.Actor{ID: "U1", Role: "GUEST"})
	require.NoError(t, err)
	assert.Equal(t, "Password reset successfully", msg)
}
