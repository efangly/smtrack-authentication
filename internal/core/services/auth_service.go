package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
)

const (
	// Five wrong passwords for one username lock it for the rest of a 15-minute window.
	maxLoginFailures = 5
	lockoutWindow    = 15 * time.Minute
	// maxUsernameKey bounds the key a caller can make us store.
	maxUsernameKey = 155
)

type authService struct {
	userService ports.UserService
	cache       ports.CachePort
	sessions    ports.SessionStore
	upload      ports.FileUploadPort
	tokens      ports.TokenIssuer
	hasher      ports.PasswordHasher
	audit       ports.AuditService
}

func NewAuthService(
	userService ports.UserService,
	cache ports.CachePort,
	sessions ports.SessionStore,
	upload ports.FileUploadPort,
	tokens ports.TokenIssuer,
	hasher ports.PasswordHasher,
	audit ports.AuditService,
) ports.AuthService {
	return &authService{
		userService: userService,
		cache:       cache,
		sessions:    sessions,
		upload:      upload,
		tokens:      tokens,
		hasher:      hasher,
		audit:       audit,
	}
}

func (s *authService) Register(ctx context.Context, data *domain.User, file *domain.File) (*domain.User, error) {
	// Counts soft-deleted users too: a username stays reserved.
	taken, err := s.userService.UsernameTaken(ctx, strings.ToLower(data.Username))
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, errors.New("username already exists")
	}

	if file != nil {
		picURL, err := s.upload.Upload(ctx, file, "user")
		if err != nil {
			logger.Error("Failed to upload user image", "error", err)
			return nil, fmt.Errorf("image upload failed: %w", err)
		}
		data.Pic = &picURL
	}

	// userService.Create hashes the password before it reaches the database.
	return s.userService.Create(ctx, data, nil)
}

func (s *authService) ValidateUser(ctx context.Context, username string, password string) (*domain.User, error) {
	key := lockoutKey(username)

	// A locked username answers exactly like a wrong password, after the same hashing work,
	// so a caller cannot tell a lock from a bad guess or learn that the account exists.
	if s.locked(ctx, key) {
		s.hasher.CompareDummy(password)
		s.recordFailure(ctx, username, "locked")
		return nil, nil
	}

	user, err := s.userService.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		// Hash anyway so an unknown username costs the same as a wrong
		// password — otherwise response time reveals which accounts exist.
		s.hasher.CompareDummy(password)
		s.failed(ctx, key, username)
		return nil, nil
	}

	if !s.hasher.Compare(user.Password, password) {
		s.failed(ctx, key, username)
		return nil, nil
	}

	// A deactivated account fails exactly like a wrong password, and only
	// after the hash comparison, so neither the message nor the timing tells
	// a caller that the account exists but is disabled.
	if !user.Status {
		s.failed(ctx, key, username)
		return nil, nil
	}

	if err := s.sessions.ClearLoginFailures(ctx, key); err != nil {
		logger.Warn("Failed to clear login failures", "error", err)
	}
	return user, nil
}

// locked reports whether the username has used up its failures. A store outage lets the
// login proceed: refusing everyone would turn a Redis fault into a full lockout.
func (s *authService) locked(ctx context.Context, key string) bool {
	n, err := s.sessions.LoginFailures(ctx, key)
	if err != nil {
		logger.Warn("Failed to read login failures", "error", err)
		return false
	}
	return n >= maxLoginFailures
}

func (s *authService) failed(ctx context.Context, key, username string) {
	if _, err := s.sessions.RecordLoginFailure(ctx, key, lockoutWindow); err != nil {
		logger.Warn("Failed to record login failure", "error", err)
	}
	s.recordFailure(ctx, username, "invalid credentials")
}

func (s *authService) recordFailure(ctx context.Context, username, reason string) {
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionLoginFailure, truncate(username, 100), nil,
		map[string]any{"reason": reason})
}

func lockoutKey(username string) string { return truncate(strings.ToLower(username), maxUsernameKey) }

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func (s *authService) Login(ctx context.Context, user *domain.User) (*ports.LoginResult, error) {
	actor := actorFor(user)
	pair, err := s.tokens.IssuePair(actor)
	if err != nil {
		return nil, err
	}

	if err := s.userService.RecordLogin(ctx, user.ID); err != nil {
		// Not worth refusing a valid login over.
		logger.Warn("Failed to record last login", "error", err, "userId", user.ID)
	}
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionLoginSuccess, user.ID, &actor, nil)

	return &ports.LoginResult{
		Token:         pair.Access,
		RefreshToken:  pair.Refresh,
		ID:            user.ID,
		Name:          user.Display,
		TenantID:      actor.TenantID,
		DepartmentIDs: actor.DepartmentIDs,
		Role:          user.Role,
		Pic:           user.Pic,
	}, nil
}

func (s *authService) RefreshTokens(ctx context.Context, token string) (*ports.RefreshResult, error) {
	claims, err := s.tokens.VerifyRefresh(token)
	if err != nil {
		return nil, err
	}
	userID := claims.UserID

	// Fail closed: if revocation cannot be checked, the token is not honoured.
	if err := s.checkNotRevoked(ctx, claims); err != nil {
		return nil, err
	}

	// Rebuild the claims from the current record rather than copying the old
	// ones: a user who was demoted or deleted must not keep their old rights
	// for the remaining lifetime of the refresh token. FindByID bypasses the
	// cache, so a deactivation takes effect at the very next refresh.
	user, err := s.userService.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.Status {
		return nil, errors.New("user is no longer active")
	}

	actor := actorFor(user)
	pair, err := s.tokens.IssuePair(actor)
	if err != nil {
		return nil, err
	}
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionTokenRefresh, user.ID, &actor, nil)
	return &ports.RefreshResult{Token: pair.Access, RefreshToken: pair.Refresh}, nil
}

func (s *authService) ResetPassword(ctx context.Context, username string, password string, oldPassword string, caller *domain.Actor) (string, error) {
	user, err := s.userService.FindByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("user not found")
	}

	// Only an admin-tier caller may target somebody else's account.
	if !caller.IsAdminTier() && caller.ID != user.ID {
		return "", errors.New("you can only reset your own password")
	}

	// Everyone but SUPER must prove the current password.
	if caller.Role != domain.RoleSuper {
		if oldPassword == "" {
			return "", errors.New("old password is required")
		}
		if !s.hasher.Compare(user.Password, oldPassword) {
			return "", errors.New("old password is incorrect")
		}
	}

	// userService.Update hashes the password before it reaches the database.
	_, err = s.userService.Update(ctx, user.ID, domain.UserPatch{Password: &password}, nil, caller)
	if err != nil {
		return "", err
	}

	s.cache.Del(ctx, fmt.Sprintf("user:%s", username))
	s.cache.Del(ctx, fmt.Sprintf("user:%s", user.ID))

	// Sessions started with the old password must not outlive it. Failing here leaves the
	// new password in place but the old sessions alive, so tell the caller.
	if err := s.sessions.SetTokensValidAfter(ctx, user.ID, time.Now(), refreshRevocationTTL); err != nil {
		return "", fmt.Errorf("revoke sessions: %w", err)
	}
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionPasswordReset, user.ID, caller, nil)

	return "Password reset successfully", nil
}

// refreshRevocationTTL outlives any refresh token, so a "valid after" mark never expires
// before the tokens it invalidates do.
const refreshRevocationTTL = 31 * 24 * time.Hour

func (s *authService) Logout(ctx context.Context, refreshToken string, caller *domain.Actor) error {
	if caller == nil {
		return errors.New("unauthorized")
	}
	claims, err := s.tokens.VerifyRefresh(refreshToken)
	if err != nil {
		return err
	}
	if claims.UserID != caller.ID {
		return errors.New("refresh token does not belong to you")
	}
	ttl := time.Until(claims.ExpiresAt)
	if ttl > 0 {
		if err := s.sessions.RevokeToken(ctx, claims.TokenID, ttl); err != nil {
			return fmt.Errorf("revoke token: %w", err)
		}
	}
	s.audit.Record(ctx, domain.AuditEntityUser, domain.AuditActionLogout, caller.ID, caller, nil)
	return nil
}

// checkNotRevoked rejects a refresh token that was logged out or predates the user's last
// password change. The messages are domain errors; store faults are wrapped.
func (s *authService) checkNotRevoked(ctx context.Context, claims *ports.RefreshClaims) error {
	revoked, err := s.sessions.IsTokenRevoked(ctx, claims.TokenID)
	if err != nil {
		return fmt.Errorf("check token revocation: %w", err)
	}
	if revoked {
		return errors.New("invalid or expired refresh token")
	}
	after, err := s.sessions.TokensValidAfter(ctx, claims.UserID)
	if err != nil {
		return fmt.Errorf("check token revocation: %w", err)
	}
	if !after.IsZero() && claims.IssuedAt.Unix() < after.Unix() {
		return errors.New("invalid or expired refresh token")
	}
	return nil
}

// actorFor is the identity a user's tokens carry.
func actorFor(user *domain.User) domain.Actor {
	actor := domain.Actor{
		ID:            user.ID,
		Role:          user.Role,
		TenantID:      user.TenantID,
		DepartmentIDs: user.LinkedDepartmentIDs(),
	}
	if user.Display != nil {
		actor.Name = *user.Display
	}
	return actor
}
