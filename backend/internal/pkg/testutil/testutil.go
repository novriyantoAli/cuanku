// Package testutil holds helpers shared by tests across the repo.
package testutil

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"gorm.io/gorm"

	"github.com/novriyantoAli/cuanku/backend/internal/config"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/database"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/logger"
)

// DSNEnv is the environment variable every integration test reads.
const DSNEnv = "DATABASE_URL"

// Logger returns a zap logger that writes into the test's output, so a failing
// assertion shows the log lines that led to it.
func Logger(t *testing.T) *zap.Logger {
	t.Helper()
	return zaptest.NewLogger(t)
}

// Config builds a minimal config for tests. It never invents a DSN: tests that
// need a database must skip via RequireDatabase.
func Config(t *testing.T) *config.Config {
	t.Helper()

	cfg := &config.Config{}
	cfg.App.Name = "cuanku-test"
	cfg.App.Env = config.EnvDevelopment
	cfg.App.Version = "test"
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 0
	cfg.Logger.Level = "error"
	cfg.Database.URL = os.Getenv(DSNEnv)
	cfg.Database.ConnectTimeout = config.DefaultConnectTimeout

	return cfg
}

// RequireDatabase skips the test unless DATABASE_URL points somewhere usable,
// then returns an open pool. Integration tests are opt-in so `make test` stays
// runnable on a workstation without the dev VM (see AGENTS.md).
func RequireDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	if os.Getenv(DSNEnv) == "" {
		t.Skipf("%s is not set: skipping integration test", DSNEnv)
	}

	log := Logger(t)
	cfg := Config(t)

	db, err := database.New(context.Background(), cfg, log)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}

	t.Cleanup(func() {
		if err := database.Close(db, log); err != nil {
			t.Logf("close database: %v", err)
		}
	})

	return db
}

// RealLogger returns the production logger constructor wired to the test
// output — used to exercise logger.New itself.
func RealLogger(t *testing.T, cfg *config.Config) *zap.Logger {
	t.Helper()

	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("build logger: %v", err)
	}
	return log
}
