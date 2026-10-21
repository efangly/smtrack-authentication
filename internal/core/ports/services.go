package ports

import (
	"context"
	"mime/multipart"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// LoginResult represents the result of a login operation. Its JSON shape is
// dto.LoginResponse.
type LoginResult struct {
	Token        string
	RefreshToken string
	ID           string
	Name         *string
	HosID        string
	WardID       string
	Role         domain.Role
	Pic          *string
}

// RefreshResult represents the result of a token refresh. Its JSON shape is
// dto.RefreshResponse.
type RefreshResult struct {
	Token        string
	RefreshToken string
}

// AuthService defines the driving port for authentication use cases
type AuthService interface {
	Register(ctx context.Context, data *domain.User, file *multipart.FileHeader) (*domain.User, error)
	ValidateUser(ctx context.Context, username string, password string) (*domain.User, error)
	Login(ctx context.Context, user *domain.User) (*LoginResult, error)
	RefreshTokens(ctx context.Context, token string) (*RefreshResult, error)
	ResetPassword(ctx context.Context, username string, password string, oldPassword string, caller *domain.Actor) (string, error)
}

// UserService defines the driving port for user use cases
type UserService interface {
	Create(ctx context.Context, user *domain.User, caller *domain.Actor) (*domain.User, error)
	FindAll(ctx context.Context, caller *domain.Actor) ([]domain.User, error)
	// FindOne is the caller-facing lookup: it is scoped to what caller may see
	// and reports anything outside that scope as not found.
	FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
	// FindByID and FindByUsername are unscoped, uncached internal lookups for
	// the auth flows (login, refresh, reset). They return nil, nil when no
	// user matches. Never expose them to a request directly.
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader, caller *domain.Actor) (*domain.User, error)
	Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
}

// HospitalService defines the driving port for hospital use cases
type HospitalService interface {
	Create(ctx context.Context, hospital *domain.Hospital, file *multipart.FileHeader, caller *domain.Actor) (*domain.Hospital, error)
	FindAll(ctx context.Context, caller *domain.Actor) ([]domain.Hospital, error)
	FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.Hospital, error)
	Update(ctx context.Context, id string, data map[string]any, file *multipart.FileHeader, caller *domain.Actor) (*domain.Hospital, error)
	Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.Hospital, error)
}

// WardService defines the driving port for ward use cases
type WardService interface {
	Create(ctx context.Context, ward *domain.Ward, caller *domain.Actor) (*domain.Ward, error)
	FindAll(ctx context.Context, caller *domain.Actor) ([]domain.Ward, error)
	FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.Ward, error)
	Update(ctx context.Context, id string, data map[string]any, caller *domain.Actor) (*domain.Ward, error)
	Remove(ctx context.Context, id string, caller *domain.Actor) (string, error)
}

// AuditService defines the driving port for audit-log use cases.
type AuditService interface {
	// Record persists an audit entry. It never returns an error to the
	// caller — a failed audit write must not fail the primary operation it
	// is recording, so failures are logged and swallowed internally.
	Record(ctx context.Context, entity domain.AuditEntity, action domain.AuditAction, entityID string, caller *domain.Actor, changes any)
	FindAll(ctx context.Context, filter AuditFilter) ([]domain.AuditLog, error)
}
