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
	svc := NewUserService(repo, &mockTenantRepo{FindByIDFn: tenantsExist}, &mockDepartmentRepo{}, newMockCache(), newMockUpload(), &fakeHasher{}, audit)

	_, err := svc.Update(context.Background(), "U1", domain.UserPatch{Password: ptr("newpass1-long-enough"), Display: ptr("me")}, nil, superCaller)
	require.NoError(t, err)

	// The repository only ever sees the hash.
	require.NotNil(t, stored.Password)
	assert.Equal(t, fakeHash("newpass1-long-enough"), *stored.Password)
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
		map[string]any{"tenantId": "T1", "departmentIds": []string{"D1"}, "status": false, "role": domain.RoleUser},
		userPatchChanges(domain.UserPatch{TenantID: ptr("T1"), DepartmentIDs: &[]string{"D1"}, Status: ptr(false), Role: ptr(domain.RoleUser), Password: ptr("secret")}),
	)
	assert.Equal(t,
		map[string]any{"name": "A", "legacy": true},
		departmentPatchChanges(domain.DepartmentPatch{Name: ptr("A"), Legacy: ptr(true)}),
	)
	assert.Equal(t,
		map[string]any{"name": "T", "contactName": "Bob", "pic": "http://img/x.png"},
		tenantPatchChanges(domain.TenantPatch{Name: ptr("T"), ContactName: ptr("Bob"), Pic: ptr("http://img/x.png")}),
	)
}
