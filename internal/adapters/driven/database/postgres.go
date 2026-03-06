package database

import (
	"github.com/tng-coop/auth-service/config"
	applog "github.com/tng-coop/auth-service/pkg/logger"
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

	applog.Info("Database connected successfully")
	return db
}
