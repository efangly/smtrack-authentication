package database

import (
	"time"

	"github.com/efangly/smtrack-authentication/config"
	"github.com/efangly/smtrack-authentication/internal/core/domain"
	applog "github.com/efangly/smtrack-authentication/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgresDB(cfg *config.Config) *gorm.DB {
	logLevel := logger.Silent
	if cfg.NodeEnv == "development" {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		applog.Fatal("Failed to connect to database", "error", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		applog.Fatal("Failed to get database instance", "error", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	// Recycle connections so a failover does not leave the pool holding
	// sockets to a server that is no longer primary.
	sqlDB.SetConnMaxLifetime(time.Hour)

	// AuditLogs is a table this service owns outright (unlike the Prisma-era
	// Users/Wards/Hospitals schema), so it is safe to auto-migrate.
	if err := db.AutoMigrate(&domain.AuditLog{}); err != nil {
		applog.Fatal("Failed to migrate audit log table", "error", err)
	}

	applog.Info("Database connected successfully")
	return db
}
