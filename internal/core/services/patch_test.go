package services

import (
	"context"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_Update_HashesPassword(t *testing.T) {
	var stored domain.UserPatch
	var audited any
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser),
		UpdateFn: func(_ context.Context, id string, patch domain.UserPatch) (*domain.User, error) {
			stored = patch
			return &domain.User{ID: id}, nil
		},
	}
	audit := newMockAuditService()
	audit.RecordFn = func(_ context.Context, _ domain.AuditEntity, _ domain.AuditAction, _ string, _ *domain.Actor, changes any) {
		audited = changes
	}
	svc := NewUserService(repo, &mockWardRepo{}, newMockCache(), newMockUpload(), &fakeHasher{}, audit)

	_, err := svc.Update(context.Background(), "U1", domain.UserPatch{Password: ptr("newpass1"), Display: ptr("me")}, nil, superCaller)
	require.NoError(t, err)

	// The repository only ever sees the hash.
	require.NotNil(t, stored.Password)
	assert.Equal(t, fakeHash("newpass1"), *stored.Password)
	// The audit entry records the other changes but no password at all.
	assert.Equal(t, map[string]any{"display": "me"}, audited)
}

func TestUserService_Update_RejectsEmptyPassword(t *testing.T) {
	repo := &mockUserRepo{
		FindByIDFn: findUser(domain.RoleUser),
		UpdateFn: func(context.Context, string, domain.UserPatch) (*domain.User, error) {
			t.Fatal("repo.Update must not be reached")
			return nil, nil
		},
	}
	svc, _, _ := newTestUserService(repo)

	_, err := svc.Update(context.Background(), "U1", domain.UserPatch{Password: ptr("")}, nil, superCaller)
	assert.EqualError(t, err, "invalid password")
}

func TestPatchChanges_OnlySetFieldsUnderAPINames(t *testing.T) {
	assert.Equal(t, map[string]any{}, userPatchChanges(domain.UserPatch{}))
	assert.Equal(t,
		map[string]any{"wardId": "W1", "status": false, "role": domain.RoleUser},
		userPatchChanges(domain.UserPatch{WardID: ptr("W1"), Status: ptr(false), Role: ptr(domain.RoleUser), Password: ptr("secret")}),
	)
	assert.Equal(t,
		map[string]any{"wardName": "A", "wardSeq": 3},
		wardPatchChanges(domain.WardPatch{WardName: ptr("A"), WardSeq: ptr(3)}),
	)
	assert.Equal(t,
		map[string]any{"hosName": "H", "hosPic": "http://img/x.png"},
		hospitalPatchChanges(domain.HospitalPatch{HosName: ptr("H"), HosPic: ptr("http://img/x.png")}),
	)
}
