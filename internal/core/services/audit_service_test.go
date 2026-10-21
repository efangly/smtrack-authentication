package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditService_Record_WritesLog(t *testing.T) {
	var saved *domain.AuditLog
	repo := &mockAuditRepo{
		CreateFn: func(_ context.Context, log *domain.AuditLog) (*domain.AuditLog, error) {
			saved = log
			return log, nil
		},
	}
	svc := NewAuditService(repo)
	caller := &domain.Actor{ID: "U1", Name: "Alice"}

	svc.Record(context.Background(), domain.AuditEntityUser, domain.AuditActionUpdate, "U1", caller,
		map[string]any{"username": "new-name"})

	require.NotNil(t, saved)
	assert.Equal(t, domain.AuditEntityUser, saved.Entity)
	assert.Equal(t, domain.AuditActionUpdate, saved.Action)
	assert.Equal(t, "U1", saved.EntityID)
	assert.Equal(t, "U1", saved.ActorID)
	assert.Equal(t, "Alice", saved.ActorName)
	assert.NotEmpty(t, saved.ID)

	var changes map[string]any
	require.NoError(t, json.Unmarshal([]byte(saved.Changes), &changes))
	assert.Equal(t, "new-name", changes["username"])
}

func TestAuditService_Record_NilCallerIsSelf(t *testing.T) {
	var saved *domain.AuditLog
	repo := &mockAuditRepo{
		CreateFn: func(_ context.Context, log *domain.AuditLog) (*domain.AuditLog, error) {
			saved = log
			return log, nil
		},
	}
	svc := NewAuditService(repo)

	svc.Record(context.Background(), domain.AuditEntityUser, domain.AuditActionCreate, "U1", nil, &domain.User{ID: "U1"})

	require.NotNil(t, saved)
	assert.Empty(t, saved.ActorID)
	assert.Equal(t, "self", saved.ActorName)
}

func TestAuditService_Record_StripsPassword(t *testing.T) {
	var saved *domain.AuditLog
	repo := &mockAuditRepo{
		CreateFn: func(_ context.Context, log *domain.AuditLog) (*domain.AuditLog, error) {
			saved = log
			return log, nil
		},
	}
	svc := NewAuditService(repo)

	svc.Record(context.Background(), domain.AuditEntityUser, domain.AuditActionUpdate, "U1", nil,
		map[string]any{"password": "s3cret", "username": "alice"})

	require.NotNil(t, saved)
	var changes map[string]any
	require.NoError(t, json.Unmarshal([]byte(saved.Changes), &changes))
	_, hasPassword := changes["password"]
	assert.False(t, hasPassword)
	assert.Equal(t, "alice", changes["username"])
}

func TestAuditService_Record_SwallowsRepoError(t *testing.T) {
	repo := &mockAuditRepo{
		CreateFn: func(_ context.Context, _ *domain.AuditLog) (*domain.AuditLog, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewAuditService(repo)

	assert.NotPanics(t, func() {
		svc.Record(context.Background(), domain.AuditEntityUser, domain.AuditActionDelete, "U1", nil, nil)
	})
}

func TestAuditService_FindAll_Passthrough(t *testing.T) {
	repo := &mockAuditRepo{
		FindAllFn: func(_ context.Context, filter ports.AuditFilter) ([]domain.AuditLog, error) {
			assert.Equal(t, "USER", filter.Entity)
			return []domain.AuditLog{{ID: "A1"}}, nil
		},
	}
	svc := NewAuditService(repo)

	logs, err := svc.FindAll(context.Background(), ports.AuditFilter{Entity: "USER"})
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}
