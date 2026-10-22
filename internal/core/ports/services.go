package ports

import (
	"context"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// LoginResult represents the result of a login operation. Its JSON shape is
// dto.LoginResponse.
type LoginResult struct {
	Token         string
	RefreshToken  string
	ID            string
	Name          *string
	TenantID      string
	DepartmentIDs []string
	Role          domain.Role
	Pic           *string
}

// RefreshResult represents the result of a token refresh. Its JSON shape is
// dto.RefreshResponse.
type RefreshResult struct {
	Token        string
	RefreshToken string
}

// AuthService defines the driving port for authentication use cases
type AuthService interface {
	Register(ctx context.Context, data *domain.User, file *domain.File) (*domain.User, error)
	ValidateUser(ctx context.Context, username string, password string) (*domain.User, error)
	Login(ctx context.Context, user *domain.User) (*LoginResult, error)
	RefreshTokens(ctx context.Context, token string) (*RefreshResult, error)
	ResetPassword(ctx context.Context, username string, password string, oldPassword string, caller *domain.Actor) (string, error)
	// Logout revokes the caller's refresh token. The access token expires on its own.
	Logout(ctx context.Context, refreshToken string, caller *domain.Actor) error
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
	Update(ctx context.Context, id string, patch domain.UserPatch, file *domain.File, caller *domain.Actor) (*domain.User, error)
	Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
	Restore(ctx context.Context, id string, caller *domain.Actor) (*domain.User, error)
	// UsernameTaken reports whether username is in use, counting soft-deleted users.
	UsernameTaken(ctx context.Context, username string) (bool, error)
	// RecordLogin stamps the user's last successful login.
	RecordLogin(ctx context.Context, id string) error
	// AccessReport lists who has access to what (role, status, tenant, departments, last
	// login) for the caller's scope, for periodic access review. The call is audited.
	AccessReport(ctx context.Context, caller *domain.Actor) ([]domain.User, error)
}

// TenantService defines the driving port for tenant use cases
type TenantService interface {
	Create(ctx context.Context, tenant *domain.Tenant, file *domain.File, caller *domain.Actor) (*domain.Tenant, error)
	FindAll(ctx context.Context, caller *domain.Actor) ([]domain.Tenant, error)
	FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.Tenant, error)
	Update(ctx context.Context, id string, patch domain.TenantPatch, file *domain.File, caller *domain.Actor) (*domain.Tenant, error)
	// UpdateLabels replaces the tenant's label profile.
	UpdateLabels(ctx context.Context, id string, labels domain.Labels, caller *domain.Actor) (*domain.Tenant, error)
	Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.Tenant, error)
	Restore(ctx context.Context, id string, caller *domain.Actor) (*domain.Tenant, error)
}

// DepartmentService defines the driving port for department use cases
type DepartmentService interface {
	Create(ctx context.Context, department *domain.Department, caller *domain.Actor) (*domain.Department, error)
	FindAll(ctx context.Context, caller *domain.Actor) ([]domain.Department, error)
	FindOne(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error)
	Update(ctx context.Context, id string, patch domain.DepartmentPatch, caller *domain.Actor) (*domain.Department, error)
	Remove(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error)
	Restore(ctx context.Context, id string, caller *domain.Actor) (*domain.Department, error)
}

// AuditService defines the driving port for audit-log use cases.
type AuditService interface {
	// Record persists an audit entry. It never returns an error to the
	// caller — a failed audit write must not fail the primary operation it
	// is recording, so failures are logged and swallowed internally.
	Record(ctx context.Context, entity domain.AuditEntity, action domain.AuditAction, entityID string, caller *domain.Actor, changes any)
	FindAll(ctx context.Context, filter AuditFilter) ([]domain.AuditLog, error)
}
