package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// softDelete marks the live row of model as deleted by deletedBy. The model's
// gorm.DeletedAt field already confines the update to live rows.
func softDelete(ctx context.Context, db *gorm.DB, model any, id, deletedBy string) error {
	res := db.WithContext(ctx).Model(model).Where("id = ?", id).
		Updates(map[string]any{"deleted_at": time.Now(), "deleted_by": deletedBy})
	if res.Error != nil {
		return fmt.Errorf("database error: %w", res.Error)
	}
	return nil
}

// restoreRow clears the deletion mark of a soft-deleted row.
func restoreRow(ctx context.Context, db *gorm.DB, model any, id string) error {
	res := db.WithContext(ctx).Unscoped().Model(model).
		Where("id = ? AND deleted_at IS NOT NULL", id).
		Updates(map[string]any{"deleted_at": nil, "deleted_by": nil, "updated_at": time.Now()})
	if res.Error != nil {
		return fmt.Errorf("database error: %w", res.Error)
	}
	return nil
}
