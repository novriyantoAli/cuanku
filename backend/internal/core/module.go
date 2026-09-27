// Package core holds the Fx providers shared by every entrypoint (API server,
// migration runner, and the servers that will be added later).
//
// It is named `core`, not `app`, because `internal/application/` is where the
// DDD domains live — two packages called "app" in one tree is a name that stops
// revealing what it holds.
package core

import (
	"context"
	"fmt"
	"os"

	"github.com/novriyantoAli/cuanku/backend/internal/config"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/database"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/logger"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ConfigFileEnv names the environment variable that points at an optional
// YAML config file. Absent file is not an error — env vars alone are enough.
const ConfigFileEnv = "CONFIG_FILE"

// CoreModule provides configuration, logging, and the database pool. Every
// entrypoint composes it, so a domain never wires infrastructure itself
// (Golden Rule 4). It is a package-level value because Fx modules are
// declarative wiring, not mutable state.
var CoreModule = fx.Options(
	fx.Provide(
		ProvideConfig,
		logger.New,
		ProvideDatabase,
	),
)

// ProvideConfig loads configuration from the optional file named by
// CONFIG_FILE, overridden by environment variables.
func ProvideConfig() (*config.Config, error) {
	cfg, err := config.Load(os.Getenv(ConfigFileEnv))
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	return cfg, nil
}

// ProvideDatabase opens the PostgreSQL pool and closes it when Fx stops.
func ProvideDatabase(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*gorm.DB, error) {
	db, err := database.New(context.Background(), cfg, log)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error { return database.Close(db, log) },
	})

	return db, nil
}
