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

func userWithPassword(plain string) *mockUserService {
	return &mockUserService{
		FindByUsernameFn: func(_ context.Context, _ string) (*domain.User, error) {
			return &domain.User{ID: "U1", Username: "admin", Password: fakeHash(plain), Status: true, Role: domain.RoleUser, TenantID: "T1"}, nil
		},
	}
}

func TestAuthService_Lockout_AfterFiveFailures(t *testing.T) {
	svc, _, _, _, hasher, sessions, audit := newTestAuthServiceAll(userWithPassword("correct-password"))
	ctx := context.Background()

	for i := 0; i < maxLoginFailures; i++ {
		user, err := svc.ValidateUser(ctx, "Admin", "wrong")
		require.NoError(t, err)
		assert.Nil(t, user)
	}
	assert.Equal(t, maxLoginFailures, sessions.failures["admin"], "key is case-insensitive")

	// Even the right password is refused now, exactly like a wrong one.
	user, err := svc.ValidateUser(ctx, "admin", "correct-password")
	require.NoError(t, err)
	assert.Nil(t, user)
	assert.Equal(t, 1, hasher.dummyCalls, "a locked login still costs a hash")
	assert.Len(t, audit.actions, maxLoginFailures+1)
	for _, a := range audit.actions {
		assert.Equal(t, domain.AuditActionLoginFailure, a)
	}
}

func TestAuthService_Lockout_SuccessClearsCounter(t *testing.T) {
	svc, _, _, _, _, sessions, _ := newTestAuthServiceAll(userWithPassword("correct-password"))
	ctx := context.Background()

	_, _ = svc.ValidateUser(ctx, "admin", "wrong")
	require.Equal(t, 1, sessions.failures["admin"])
	user, err := svc.ValidateUser(ctx, "admin", "correct-password")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Zero(t, sessions.failures["admin"])
}

func TestAuthService_Lockout_UnknownUserCountsToo(t *testing.T) {
	userSvc := &mockUserService{FindByUsernameFn: func(context.Context, string) (*domain.User, error) { return nil, nil }}
	svc, _, _, _, _, sessions, _ := newTestAuthServiceAll(userSvc)

	_, _ = svc.ValidateUser(context.Background(), "ghost", "x")
	assert.Equal(t, 1, sessions.failures["ghost"], "a locked unknown name must look like a locked real one")
}

func TestAuthService_Lockout_StoreOutageDoesNotBlockLogin(t *testing.T) {
	svc, _, _, _, _, sessions, _ := newTestAuthServiceAll(userWithPassword("correct-password"))
	sessions.err = errors.New("redis down")

	user, err := svc.ValidateUser(context.Background(), "admin", "correct-password")
	require.NoError(t, err)
	assert.NotNil(t, user)
}

func TestAuthService_Login_RecordsLoginAndAudit(t *testing.T) {
	var stamped string
	userSvc := &mockUserService{RecordLoginFn: func(_ context.Context, id string) error { stamped = id; return nil }}
	svc, _, _, _, _, _, audit := newTestAuthServiceAll(userSvc)

	_, err := svc.Login(context.Background(), &domain.User{ID: "U1", Role: domain.RoleUser, TenantID: "T1"})
	require.NoError(t, err)
	assert.Equal(t, "U1", stamped)
	assert.Equal(t, []domain.AuditAction{domain.AuditActionLoginSuccess}, audit.actions)
}

func TestAuthService_Login_StampFailureDoesNotFailLogin(t *testing.T) {
	userSvc := &mockUserService{RecordLoginFn: func(context.Context, string) error { return errors.New("db") }}
	svc, _, _, _, _, _, _ := newTestAuthServiceAll(userSvc)

	res, err := svc.Login(context.Background(), &domain.User{ID: "U1", Role: domain.RoleUser})
	require.NoError(t, err)
	assert.NotEmpty(t, res.Token)
}

func activeUser() *mockUserService {
	return &mockUserService{FindByIDFn: func(context.Context, string) (*domain.User, error) {
		return &domain.User{ID: "U1", Status: true, Role: domain.RoleUser, TenantID: "T1"}, nil
	}}
}

func TestAuthService_Logout_RevokesOwnRefreshToken(t *testing.T) {
	svc, _, _, tokens, _, sessions, audit := newTestAuthServiceAll(activeUser())
	tokens.refreshUserID = "U1"

	require.NoError(t, svc.Logout(context.Background(), "tok", &domain.Actor{ID: "U1"}))
	assert.True(t, sessions.revoked["jti-1"])
	assert.Equal(t, []domain.AuditAction{domain.AuditActionLogout}, audit.actions)

	// The same token can no longer refresh.
	_, err := svc.RefreshTokens(context.Background(), "tok")
	assert.EqualError(t, err, "invalid or expired refresh token")
}

func TestAuthService_Logout_RejectsSomeoneElsesToken(t *testing.T) {
	svc, _, _, tokens, _, sessions, _ := newTestAuthServiceAll(activeUser())
	tokens.refreshUserID = "U2"

	err := svc.Logout(context.Background(), "tok", &domain.Actor{ID: "U1"})
	assert.EqualError(t, err, "refresh token does not belong to you")
	assert.Empty(t, sessions.revoked)
}

func TestAuthService_Logout_InvalidToken(t *testing.T) {
	svc, _, _, tokens, _, _, _ := newTestAuthServiceAll(activeUser())
	tokens.refreshErr = errors.New("invalid or expired refresh token")

	assert.Error(t, svc.Logout(context.Background(), "bad", &domain.Actor{ID: "U1"}))
}

func TestAuthService_Refresh_FailsClosedWhenStoreIsDown(t *testing.T) {
	svc, _, _, tokens, _, sessions, _ := newTestAuthServiceAll(activeUser())
	tokens.refreshUserID = "U1"
	sessions.err = errors.New("redis down")

	_, err := svc.RefreshTokens(context.Background(), "tok")
	assert.Error(t, err)
}

func TestAuthService_Refresh_RejectsTokenIssuedBeforePasswordChange(t *testing.T) {
	svc, _, _, tokens, _, sessions, audit := newTestAuthServiceAll(activeUser())
	issued := time.Now().Add(-time.Hour)
	tokens.refreshClaims = &ports.RefreshClaims{UserID: "U1", TokenID: "old", IssuedAt: issued, ExpiresAt: issued.Add(7 * 24 * time.Hour)}
	sessions.validAfter["U1"] = time.Now().Add(-time.Minute)

	_, err := svc.RefreshTokens(context.Background(), "tok")
	assert.EqualError(t, err, "invalid or expired refresh token")

	// A token issued after the change is fine, and the refresh is audited.
	tokens.refreshClaims = &ports.RefreshClaims{UserID: "U1", TokenID: "new", IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	_, err = svc.RefreshTokens(context.Background(), "tok")
	require.NoError(t, err)
	assert.Equal(t, []domain.AuditAction{domain.AuditActionTokenRefresh}, audit.actions)
}

func TestAuthService_ResetPassword_InvalidatesEarlierSessionsAndAudits(t *testing.T) {
	userSvc := &mockUserService{
		FindByUsernameFn: func(context.Context, string) (*domain.User, error) {
			return &domain.User{ID: "U1", Username: "admin", Password: fakeHash("old-password-123")}, nil
		},
		UpdateFn: func(_ context.Context, _ string, _ domain.UserPatch, _ *domain.File, _ *domain.Actor) (*domain.User, error) {
			return &domain.User{ID: "U1"}, nil
		},
	}
	svc, _, _, _, _, sessions, audit := newTestAuthServiceAll(userSvc)
	caller := &domain.Actor{ID: "U1", Role: domain.RoleUser}

	_, err := svc.ResetPassword(context.Background(), "admin", "new-password-123", "old-password-123", caller)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), sessions.validAfter["U1"], time.Second)
	assert.Equal(t, []domain.AuditAction{domain.AuditActionPasswordReset}, audit.actions)
}
