package domain

import "time"

// AuditAction represents the kind of mutation an audit log records.
type AuditAction string

const (
	AuditActionCreate AuditAction = "CREATE"
	AuditActionUpdate AuditAction = "UPDATE"
	AuditActionDelete AuditAction = "DELETE"
)

// AuditEntity represents the entity type an audit log records a change for.
type AuditEntity string

const (
	AuditEntityUser     AuditEntity = "USER"
	AuditEntityWard     AuditEntity = "WARD"
	AuditEntityHospital AuditEntity = "HOSPITAL"
)

// AuditLog represents an immutable record of a create/update/delete action.
type AuditLog struct {
	ID        string      `gorm:"column:id;type:varchar(100);primaryKey" json:"id"`
	Entity    AuditEntity `gorm:"column:entity;type:varchar(20);not null;index" json:"entity"`
	EntityID  string      `gorm:"column:entityId;type:varchar(100);not null;index" json:"entityId"`
	Action    AuditAction `gorm:"column:action;type:varchar(10);not null" json:"action"`
	ActorID   string      `gorm:"column:actorId;type:varchar(100)" json:"actorId"`
	ActorName string      `gorm:"column:actorName;type:varchar(150)" json:"actorName"`
	Changes   string      `gorm:"column:changes;type:text" json:"changes"`
	CreatedAt time.Time   `gorm:"column:createAt;autoCreateTime" json:"createAt"`
}

func (AuditLog) TableName() string {
	return "AuditLogs"
}
