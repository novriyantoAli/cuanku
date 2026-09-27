// Package database owns the PostgreSQL connection pool.
package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/novriyantoAli/cuanku/backend/internal/config"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// New opens the GORM connection, configures the pool, and verifies the
// connection with a real ping so a bad DSN fails at startup rather than on
// the first request.
func New(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.URL), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormLogLevel(cfg)),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	sqlDB, err := Conn(db)
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.Database.ConnectTimeout)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	logger.Info("database connected",
		zap.Int("max_open_conns", cfg.Database.MaxOpenConns),
		zap.Int("max_idle_conns", cfg.Database.MaxIdleConns),
	)

	return db, nil
}

// Conn exposes the underlying *sql.DB. The migration runner needs it to hand the
// same connection to golang-migrate, so a migration and the API always talk to
// the same DSN.
func Conn(db *gorm.DB) (*sql.DB, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access underlying sql.DB: %w", err)
	}
	return sqlDB, nil
}

// Close releases the pool. Wired to the Fx lifecycle by the server modules.
func Close(db *gorm.DB, logger *zap.Logger) error {
	sqlDB, err := Conn(db)
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close postgres connection: %w", err)
	}
	logger.Info("database connection closed")
	return nil
}

// Ping reports whether the database answers within the caller's context.
func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := Conn(db)
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

func gormLogLevel(cfg *config.Config) gormlogger.LogLevel {
	if cfg.App.IsProduction() {
		return gormlogger.Warn
	}
	return gormlogger.Info
}
