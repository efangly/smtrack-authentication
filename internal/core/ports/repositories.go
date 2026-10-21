package ports

import (
	"context"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// The *Filter types narrow a listing. Zero-value fields are not applied.

// UserFilter narrows a user listing. Hospital conditions apply through the
// user's ward.
type UserFilter struct {
	HosID         string
	ExcludeHosIDs []string
	ExcludeRoles  []domain.Role
}

// WardFilter narrows a ward listing.
type WardFilter struct {
	HosID         string
	ExcludeHosIDs []string
}

// HospitalFilter narrows a hospital listing. WardID keeps only the hospital
// that contains that ward.
type HospitalFilter struct {
	ID         string
	WardID     string
	ExcludeIDs []string
}

// Repositories set the updated-at timestamp themselves on every Update.

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	FindAll(ctx context.Context, filter UserFilter) ([]domain.User, error)
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	Update(ctx context.Context, id string, patch domain.UserPatch) (*domain.User, error)
	Delete(ctx context.Context, id string) (*domain.User, error)
}

type WardRepository interface {
	Create(ctx context.Context, ward *domain.Ward) (*domain.Ward, error)
	FindAll(ctx context.Context, filter WardFilter) ([]domain.Ward, error)
	FindByID(ctx context.Context, id string) (*domain.Ward, error)
	Update(ctx context.Context, id string, patch domain.WardPatch) (*domain.Ward, error)
	Delete(ctx context.Context, id string) error
}

type HospitalRepository interface {
	Create(ctx context.Context, hospital *domain.Hospital) (*domain.Hospital, error)
	FindAll(ctx context.Context, filter HospitalFilter) ([]domain.Hospital, error)
	FindByID(ctx context.Context, id string) (*domain.Hospital, error)
	Update(ctx context.Context, id string, patch domain.HospitalPatch) (*domain.Hospital, error)
	Delete(ctx context.Context, id string) (*domain.Hospital, error)
}

// AuditFilter narrows an audit log query. Zero-value fields are not applied.
type AuditFilter struct {
	Entity   string
	Action   string
	EntityID string
	ActorID  string
	Limit    int
	Offset   int
}

type AuditRepository interface {
	Create(ctx context.Context, log *domain.AuditLog) (*domain.AuditLog, error)
	FindAll(ctx context.Context, filter AuditFilter) ([]domain.AuditLog, error)
}
