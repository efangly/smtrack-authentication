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
		ID: id, Username: "u-" + id, Role: domain.RoleUser, Password: storedHash,
		Ward: &domain.Ward{ID: "W1", HosID: "H1", Users: []domain.User{{ID: "N1", Password: storedHash}}},
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
	default:
		t.Fatalf("unexpected cached type %T", v)
	}
	for _, u := range users {
		assert.Empty(t, u.Password, "user %s cached with its password hash", u.ID)
		if u.Ward != nil {
			for _, n := range u.Ward.Users {
				assert.Empty(t, n.Password, "nested user %s cached with its password hash", n.ID)
			}
		}
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
	assert.Empty(t, clean.Ward.Users[0].Password)
	assert.Equal(t, storedHash, u.Password)
	assert.Equal(t, storedHash, u.Ward.Users[0].Password, "nested slice must be copied, not cleared in place")
}

func TestUserService_AuditSnapshotsCarryNoPassword(t *testing.T) {
	var snapshots []any
	audit := newMockAuditService()
	audit.RecordFn = func(_ context.Context, _ domain.AuditEntity, _ domain.AuditAction, _ string, _ *domain.Actor, changes any) {
		snapshots = append(snapshots, changes)
	}
	repo := &mockUserRepo{
		CreateFn:   func(_ context.Context, u *domain.User) (*domain.User, error) { return u, nil },
		FindByIDFn: func(_ context.Context, id string) (*domain.User, error) { u := userWithHash(id); return &u, nil },
		DeleteFn:   func(_ context.Context, id string) (*domain.User, error) { u := userWithHash(id); return &u, nil },
	}
	svc := NewUserService(repo, &mockWardRepo{}, newMockCache(), newMockUpload(), &fakeHasher{}, audit)

	_, err := svc.Create(context.Background(), &domain.User{Username: "u", Password: "plain", Role: domain.RoleUser}, superCaller)
	require.NoError(t, err)
	_, err = svc.Remove(context.Background(), "U1", superCaller)
	require.NoError(t, err)

	require.Len(t, snapshots, 2)
	for _, s := range snapshots {
		assertNoHash(t, s)
	}
}

// ADR 0002: anything that changes a ward or hospital also clears users,
// because user listings embed their ward and hospital.
func TestCascadeInvalidation(t *testing.T) {
	wardRepo := &mockWardRepo{
		UpdateFn: func(_ context.Context, id string, _ domain.WardPatch) (*domain.Ward, error) {
			return &domain.Ward{ID: id}, nil
		},
	}
	wardSvc, cache, _ := newTestWardService(wardRepo)
	_, err := wardSvc.Update(context.Background(), "W1", domain.WardPatch{WardName: ptr("A")}, superCaller)
	require.NoError(t, err)
	assert.Subset(t, cache.deleted, []string{"ward", "hospital", "user"})

	hosRepo := &mockHospitalRepo{
		UpdateFn: func(_ context.Context, id string, _ domain.HospitalPatch) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id}, nil
		},
	}
	hosSvc, cache, _, _ := newTestHospitalService(hosRepo)
	_, err = hosSvc.Update(context.Background(), "H1", domain.HospitalPatch{HosName: ptr("H")}, nil, superCaller)
	require.NoError(t, err)
	assert.Subset(t, cache.deleted, []string{"ward", "hospital", "user"})
}
