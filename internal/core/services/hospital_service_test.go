package services

import (
	"context"
	"errors"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHospitalService(repo *mockHospitalRepo) (ports.HospitalService, *mockCache, *mockUpload, *mockMessaging) {
	cache := newMockCache()
	upload := newMockUpload()
	msg := newMockMessaging()
	svc := NewHospitalService(repo, cache, upload, msg, newMockAuditService())
	return svc, cache, upload, msg
}

// --- Create ---

func TestHospitalService_Create_Success(t *testing.T) {
	repo := &mockHospitalRepo{
		CreateFn: func(_ context.Context, h *domain.Hospital) (*domain.Hospital, error) {
			return h, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.Create(context.Background(), &domain.Hospital{HosName: "Hospital A"}, nil, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, h.ID)
	assert.Equal(t, "Hospital A", h.HosName)
	assert.False(t, h.CreatedAt.IsZero())
}

func TestHospitalService_Create_WithFile(t *testing.T) {
	repo := &mockHospitalRepo{
		CreateFn: func(_ context.Context, h *domain.Hospital) (*domain.Hospital, error) {
			return h, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.Create(context.Background(), &domain.Hospital{HosName: "H1"}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "H1", h.HosName)
}

func TestHospitalService_Create_PreservesID(t *testing.T) {
	repo := &mockHospitalRepo{
		CreateFn: func(_ context.Context, h *domain.Hospital) (*domain.Hospital, error) {
			return h, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.Create(context.Background(), &domain.Hospital{ID: "CUSTOM", HosName: "H1"}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "CUSTOM", h.ID)
}

func TestHospitalService_Create_RepoError(t *testing.T) {
	repo := &mockHospitalRepo{
		CreateFn: func(_ context.Context, _ *domain.Hospital) (*domain.Hospital, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	_, err := svc.Create(context.Background(), &domain.Hospital{HosName: "H1"}, nil, nil)
	assert.EqualError(t, err, "db error")
}

// --- FindAll ---

func TestHospitalService_FindAll_Success(t *testing.T) {
	repo := &mockHospitalRepo{
		FindAllFn: func(_ context.Context, _ ports.HospitalFilter) ([]domain.Hospital, error) {
			return []domain.Hospital{{ID: "H1"}, {ID: "H2"}}, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	hospitals, err := svc.FindAll(context.Background(), &domain.Actor{Role: "SUPER"})
	require.NoError(t, err)
	assert.Len(t, hospitals, 2)
}

func TestHospitalService_FindAll_RepoError(t *testing.T) {
	repo := &mockHospitalRepo{
		FindAllFn: func(_ context.Context, _ ports.HospitalFilter) ([]domain.Hospital, error) {
			return nil, errors.New("db error")
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	_, err := svc.FindAll(context.Background(), &domain.Actor{Role: "SUPER"})
	assert.EqualError(t, err, "db error")
}

// --- FindOne ---

func TestHospitalService_FindOne_Success(t *testing.T) {
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id, HosName: "Test Hospital"}, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.FindOne(context.Background(), "H1", superCaller)
	require.NoError(t, err)
	assert.Equal(t, "H1", h.ID)
}

func TestHospitalService_FindOne_NotFound(t *testing.T) {
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, _ string) (*domain.Hospital, error) {
			return nil, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	_, err := svc.FindOne(context.Background(), "nonexistent", superCaller)
	assert.EqualError(t, err, "hospital not found")
}

// --- Update ---

func TestHospitalService_Update_Success(t *testing.T) {
	deviceCalled := false
	legacyCalled := false
	repo := &mockHospitalRepo{
		UpdateFn: func(_ context.Context, _ string, _ domain.HospitalPatch) (*domain.Hospital, error) {
			return &domain.Hospital{ID: "H1", HosName: "Updated"}, nil
		},
	}
	svc, _, _, msg := newTestHospitalService(repo)
	msg.SendToDeviceFn = func(string, any) error { deviceCalled = true; return nil }
	msg.SendToLegacyFn = func(string, any) error { legacyCalled = true; return nil }

	h, err := svc.Update(context.Background(), "H1", domain.HospitalPatch{HosName: ptr("Updated")}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "Updated", h.HosName)
	assert.True(t, deviceCalled)
	assert.True(t, legacyCalled)
}

// --- Remove ---

func TestHospitalService_Remove_Success(t *testing.T) {
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id}, nil
		},
		DeleteFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id}, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	h, err := svc.Remove(context.Background(), "H1", nil)
	require.NoError(t, err)
	assert.Equal(t, "H1", h.ID)
}

func TestHospitalService_Remove_NotFound(t *testing.T) {
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, _ string) (*domain.Hospital, error) {
			return nil, nil
		},
	}
	svc, _, _, _ := newTestHospitalService(repo)

	_, err := svc.Remove(context.Background(), "nonexistent", nil)
	assert.EqualError(t, err, "hospital not found")
}

func TestHospitalService_Remove_WithPic(t *testing.T) {
	pic := "http://example.com/hospital/pic.jpg"
	deleteCalled := false
	repo := &mockHospitalRepo{
		FindByIDFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id, HosPic: &pic}, nil
		},
		DeleteFn: func(_ context.Context, id string) (*domain.Hospital, error) {
			return &domain.Hospital{ID: id, HosPic: &pic}, nil
		},
	}
	svc, _, upload, _ := newTestHospitalService(repo)
	upload.DeleteFn = func(string, string) error { deleteCalled = true; return nil }

	_, err := svc.Remove(context.Background(), "H1", nil)
	require.NoError(t, err)
	assert.True(t, deleteCalled)
}

// --- buildHospitalConditions ---

func TestBuildHospitalFilter(t *testing.T) {
	dev := []string{developmentHosID}
	tests := []struct {
		name       string
		caller     domain.Actor
		wantFilter ports.HospitalFilter
		wantKey    string
	}{
		{"SUPER", domain.Actor{Role: domain.RoleSuper}, ports.HospitalFilter{}, "hospital"},
		{"ADMIN", domain.Actor{Role: domain.RoleAdmin, HosID: "H1"}, ports.HospitalFilter{ID: "H1", ExcludeIDs: dev}, "hospital:H1"},
		{"LEGACY_ADMIN", domain.Actor{Role: domain.RoleLegacyAdmin, HosID: "H2"}, ports.HospitalFilter{ID: "H2", ExcludeIDs: dev}, "hospital:H2"},
		{"SERVICE", domain.Actor{Role: domain.RoleService}, ports.HospitalFilter{ExcludeIDs: dev}, "hospital:HID-DEVELOPMENT"},
		{"USER with ward", domain.Actor{Role: domain.RoleUser, WardID: "W1"}, ports.HospitalFilter{WardID: "W1"}, "hospital:W1"},
		{"GUEST without ward", domain.Actor{Role: domain.RoleGuest}, ports.HospitalFilter{}, "hospital"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, key := buildHospitalFilter(&tt.caller)
			assert.Equal(t, tt.wantFilter, filter)
			assert.Equal(t, tt.wantKey, key)
		})
	}
}
