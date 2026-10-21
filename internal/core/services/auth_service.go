package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/efangly/smtrack-authentication/pkg/logger"
)

type authService struct {
	userService ports.UserService
	cache       ports.CachePort
	upload      ports.FileUploadPort
	tokens      ports.TokenIssuer
	hasher      ports.PasswordHasher
}

func NewAuthService(
	userService ports.UserService,
	cache ports.CachePort,
	upload ports.FileUploadPort,
	tokens ports.TokenIssuer,
	hasher ports.PasswordHasher,
) ports.AuthService {
	return &authService{
		userService: userService,
		cache:       cache,
		upload:      upload,
		tokens:      tokens,
		hasher:      hasher,
	}
}

func (s *authService) Register(ctx context.Context, data *domain.User, file *domain.File) (*domain.User, error) {
	existing, err := s.userService.FindByUsername(ctx, strings.ToLower(data.Username))
	if err != nil {
		return nil, err
	}
	if existing != nil {
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
	user, err := s.userService.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		// Hash anyway so an unknown username costs the same as a wrong
		// password — otherwise response time reveals which accounts exist.
		s.hasher.CompareDummy(password)
		return nil, nil
	}

	if !s.hasher.Compare(user.Password, password) {
		return nil, nil
	}

	// A deactivated account fails exactly like a wrong password, and only
	// after the hash comparison, so neither the message nor the timing tells
	// a caller that the account exists but is disabled.
	if !user.Status {
		return nil, nil
	}

	return user, nil
}

func (s *authService) Login(ctx context.Context, user *domain.User) (*ports.LoginResult, error) {
	actor := actorFor(user)
	pair, err := s.tokens.IssuePair(actor)
	if err != nil {
		return nil, err
	}

	return &ports.LoginResult{
		Token:        pair.Access,
		RefreshToken: pair.Refresh,
		ID:           user.ID,
		Name:         user.Display,
		HosID:        actor.HosID,
		WardID:       user.WardID,
		Role:         user.Role,
		Pic:          user.Pic,
	}, nil
}

func (s *authService) RefreshTokens(ctx context.Context, token string) (*ports.RefreshResult, error) {
	userID, err := s.tokens.VerifyRefresh(token)
	if err != nil {
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

	pair, err := s.tokens.IssuePair(actorFor(user))
	if err != nil {
		return nil, err
	}
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

	return "Password reset successfully", nil
}

// actorFor is the identity a user's tokens carry.
func actorFor(user *domain.User) domain.Actor {
	actor := domain.Actor{ID: user.ID, Role: user.Role, WardID: user.WardID}
	if user.Display != nil {
		actor.Name = *user.Display
	}
	if user.Ward != nil {
		actor.HosID = user.Ward.HosID
	}
	return actor
}
