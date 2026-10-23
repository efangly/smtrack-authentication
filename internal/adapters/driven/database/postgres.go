package database

import (
	"context"
	"time"

	"github.com/efangly/smtrack-authentication/config"
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

	if err := Migrate(context.Background(), sqlDB); err != nil {
		applog.Fatal("Failed to migrate database", "error", err)
	}

	applog.Info("Database connected successfully")
	return db
}
