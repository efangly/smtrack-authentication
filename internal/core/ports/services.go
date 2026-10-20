package ports

import (
	"context"
	"mime/multipart"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// JwtPayload represents the JWT token payload
type JwtPayload struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	HosID  string `json:"hosId"`
	WardID string `json:"wardId"`
}

// IsAdminTier reports whether the caller may act on records belonging to other
// users. Everyone else is restricted to their own account.
func (p *JwtPayload) IsAdminTier() bool {
	if p == nil {
		return false
	}
	switch domain.Role(p.Role) {
	case domain.RoleSuper, domain.RoleService, domain.RoleAdmin, domain.RoleLegacyAdmin:
		return true
	}
	return false
}

// CanAssign reports whether the caller may give role to a user, or modify or
// delete a user who already holds it. See domain.Role.CanAssign.
func (p *JwtPayload) CanAssign(role domain.Role) bool {
	if p == nil {
		return false
	}
	return domain.Role(p.Role).CanAssign(role)
}

// LoginResult represents the result of a login operation
type LoginResult struct {
	Token        string  `json:"token"`
	RefreshToken string  `json:"refreshToken"`
	ID           string  `json:"id"`
	Name         *string `json:"name"`
	HosID        string  `json:"hosId"`
	WardID       string  `json:"wardId"`
	Role         string  `json:"role"`
	Pic          *string `json:"pic"`
}

// RefreshResult represents the result of a token refresh
type RefreshResult struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
}

// AuthService defines the driving port for authentication use cases
type AuthService interface {
	Register(ctx context.Context, data *domain.User, file *multipart.FileHeader) (*domain.User, error)
	ValidateUser(ctx context.Context, username string, password string) (*domain.User, error)
	Login(ctx context.Context, user *domain.User) (*LoginResult, error)
	RefreshTokens(ctx context.Context, token string) (*RefreshResult, error)
	ResetPassword(ctx context.Context, username string, password string, oldPassword string, caller *JwtPayload) (string, error)
}

// UserService defines the driving port for user use cases
type UserService interface {
	Create(ctx context.Context, user *domain.User, caller *JwtPayload) (*domain.User, error)
	FindAll(ctx context.Context, caller *JwtPayload) ([]domain.User, error)
	// FindOne is the caller-facing lookup: it is scoped to what caller may see
	// and reports anything outside that scope as not found.
	FindOne(ctx context.Context, id string, caller *JwtPayload) (*domain.User, error)
	// FindByID and FindByUsername are unscoped, uncached internal lookups for
	// the auth flows (login, refresh, reset). They return nil, nil when no
	// user matches. Never expose them to a request directly.
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader, caller *JwtPayload) (*domain.User, error)
	Remove(ctx context.Context, id string, caller *JwtPayload) (*domain.User, error)
}

// HospitalService defines the driving port for hospital use cases
type HospitalService interface {
	Create(ctx context.Context, hospital *domain.Hospital, file *multipart.FileHeader, caller *JwtPayload) (*domain.Hospital, error)
	FindAll(ctx context.Context, caller *JwtPayload) ([]domain.Hospital, error)
	FindOne(ctx context.Context, id string, caller *JwtPayload) (*domain.Hospital, error)
	Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader, caller *JwtPayload) (*domain.Hospital, error)
	Remove(ctx context.Context, id string, caller *JwtPayload) (*domain.Hospital, error)
}

// WardService defines the driving port for ward use cases
type WardService interface {
	Create(ctx context.Context, ward *domain.Ward, caller *JwtPayload) (*domain.Ward, error)
	FindAll(ctx context.Context, caller *JwtPayload) ([]domain.Ward, error)
	FindOne(ctx context.Context, id string, caller *JwtPayload) (*domain.Ward, error)
	Update(ctx context.Context, id string, data map[string]any, caller *JwtPayload) (*domain.Ward, error)
	Remove(ctx context.Context, id string, caller *JwtPayload) (string, error)
}

// AuditService defines the driving port for audit-log use cases.
type AuditService interface {
	// Record persists an audit entry. It never returns an error to the
	// caller — a failed audit write must not fail the primary operation it
	// is recording, so failures are logged and swallowed internally.
	Record(ctx context.Context, entity domain.AuditEntity, action domain.AuditAction, entityID string, caller *JwtPayload, changes any)
	FindAll(ctx context.Context, filter AuditFilter) ([]domain.AuditLog, error)
}
