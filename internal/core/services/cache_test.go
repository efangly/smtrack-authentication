package services

import (
	"context"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const storedHash = "hashed:secret"

func userWithHash(id string) domain.User {
	return domain.User{
		ID: id, Username: "u-" + id, Role: domain.RoleUser, Password: storedHash, TenantID: "T1",
	}
}

// assertNoHash fails if any user reachable from v carries a password.
func assertNoHash(t *testing.T, v any) {
	t.Helper()
	var users []domain.User
	switch x := v.(type) {
	case domain.User:
		users = []domain.User{x}
	case []domain.User:
		users = x
	case map[string]any:
		assert.NotContains(t, x, "password")
		return
	default:
		t.Fatalf("unexpected cached type %T", v)
	}
	for _, u := range users {
		assert.Empty(t, u.Password, "user %s cached with its password hash", u.ID)
	}
}

func TestUserService_FindAll_CachesWithoutSecrets(t *testing.T) {
	calls := 0
	repo := &mockUserRepo{
		FindAllFn: func(context.Context, ports.UserFilter) ([]domain.User, error) {
			calls++
			return []domain.User{userWithHash("U1"), userWithHash("U2")}, nil
		},
	}
	svc, cache, _ := newTestUserService(repo)

	_, err := svc.FindAll(context.Background(), superCaller)
	require.NoError(t, err)
	require.Contains(t, cache.stored, "user")
	assertNoHash(t, cache.stored["user"])
	assert.Equal(t, listTTL, cache.ttls["user"])

	// Second read is served from the cache.
	users, err := svc.FindAll(context.Background(), superCaller)
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, 1, calls)
}

func TestUserService_FindOne_CachesWithoutSecrets(t *testing.T) {
	calls := 0
	repo := &mockUserRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			calls++
			u := userWithHash(id)
			return &u, nil
		},
	}
	svc, cache, _ := newTestUserService(repo)

	_, err := svc.FindOne(context.Background(), "U1", superCaller)
	require.NoError(t, err)
	require.Contains(t, cache.stored, "user:U1")
	assertNoHash(t, cache.stored["user:U1"])
	assert.Equal(t, itemTTL, cache.ttls["user:U1"])

	_, err = svc.FindOne(context.Background(), "U1", superCaller)
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "second read should hit the cache")
}

func TestUser_WithoutSecrets_DoesNotMutateOriginal(t *testing.T) {
	u := userWithHash("U1")
	clean := u.WithoutSecrets()

	assert.Empty(t, clean.Password)
	assert.Equal(t, storedHash, u.Password)
}

func TestUserService_AuditSnapshotsCarryNoPassword(t *testing.T) {
	var snapshots []any
	f := newUserFixture(&mockUserRepo{
		CreateFn:   echoUser,
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) { u := userWithHash(id); return &u, nil },
		DeleteFn:   func(_ context.Context, id, _ string) (*domain.User, error) { u := userWithHash(id); return &u, nil },
		FindAnyByIDFn: func(_ context.Context, id string) (*domain.User, error) {
			u := deletedUserValue(id)
			return &u, nil
		},
		RestoreFn: func(_ context.Context, id string) (*domain.User, error) { u := userWithHash(id); return &u, nil },
	})
	f.audit.RecordFn = func(_ context.Context, _ domain.AuditEntity, _ domain.AuditAction, _ string, _ *domain.Actor, changes any) {
		snapshots = append(snapshots, changes)
	}

	_, err := f.svc.Create(context.Background(), &domain.User{Username: "u", Password: "plain-password-123", Role: domain.RoleUser, TenantID: "T1"}, superCaller)
	require.NoError(t, err)
	_, err = f.svc.Remove(context.Background(), "U1", superCaller)
	require.NoError(t, err)
	_, err = f.svc.Restore(context.Background(), "U1", superCaller)
	require.NoError(t, err)

	require.Len(t, snapshots, 3)
	for _, s := range snapshots {
		assertNoHash(t, s)
	}
}

func deletedUserValue(id string) domain.User {
	u := userWithHash(id)
	now := u.CreatedAt
	u.DeletedAt = &now
	return u
}

// ADR 0002: anything that changes a tenant or department also clears users,
// because user listings embed them.
func TestCascadeInvalidation(t *testing.T) {
	deptSvc, cache, _ := newTestDepartmentService(&mockDepartmentRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Department, error) {
			return &domain.Department{ID: id, TenantID: "T1"}, nil
		},
		UpdateFn: func(_ context.Context, id string, _ domain.DepartmentPatch) (*domain.Department, error) {
			return &domain.Department{ID: id}, nil
		},
	})
	_, err := deptSvc.Update(context.Background(), "D1", domain.DepartmentPatch{Name: ptr("A")}, superCaller)
	require.NoError(t, err)
	assert.Subset(t, cache.deleted, []string{"department", "tenant", "user"})

	tenantSvc, cache, _, _ := newTestTenantService(&mockTenantRepo{
		FindByIDFn: tenantsExist,
		UpdateFn: func(_ context.Context, id string, _ domain.TenantPatch) (*domain.Tenant, error) {
			return &domain.Tenant{ID: id}, nil
		},
	})
	_, err = tenantSvc.Update(context.Background(), "T1", domain.TenantPatch{Name: ptr("T")}, nil, superCaller)
	require.NoError(t, err)
	assert.Subset(t, cache.deleted, []string{"department", "tenant", "user"})
}
