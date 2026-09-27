// Command migration applies the versioned SQL migrations to PostgreSQL.
//
// Usage (DSN always comes from DATABASE_URL — never from a flag or a file):
//
//	go run ./cmd/migration -direction=up
//	go run ./cmd/migration -direction=down -steps=1
//	go run ./cmd/migration -direction=version
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/novriyantoAli/cuanku/backend/internal/core"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/migrator"
)

const defaultTimeout = 60 * time.Second

// Directions accepted by -direction.
const (
	directionUp      = "up"
	directionDown    = "down"
	directionVersion = "version"
)

type options struct {
	direction string
	steps     int
	timeout   time.Duration
}

func main() {
	opts := options{}
	flag.StringVar(&opts.direction, "direction", directionUp, "up | down | version")
	flag.IntVar(&opts.steps, "steps", 0, "migrations to apply; 0 applies all (up) or 1 (down)")
	flag.DurationVar(&opts.timeout, "timeout", defaultTimeout, "overall timeout")
	flag.Parse()

	application := fx.New(
		core.CoreModule,
		migrator.Module,
		fx.Supply(opts),
		fx.Invoke(run),
	)

	if err := application.Err(); err != nil {
		log.Fatalf("cuanku-migration: %v", err)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()
	if err := application.Stop(stopCtx); err != nil {
		log.Printf("cuanku-migration: closing database: %v", err)
	}
}

func run(opts options, m *migrator.Migrator, logger *zap.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	switch opts.direction {
	case directionUp:
		return runUp(ctx, m, logger)
	case directionDown:
		return runDown(ctx, opts, m)
	case directionVersion:
		return runVersion(ctx, m, logger)
	default:
		return fmt.Errorf("unknown -direction %q (want up, down, or version)", opts.direction)
	}
}

func runUp(ctx context.Context, m *migrator.Migrator, logger *zap.Logger) error {
	version, applied, err := m.Up(ctx)
	if err != nil {
		return err
	}

	if applied {
		logger.Info("migrations applied", zap.Uint("version", version))
	}
	fmt.Printf("schema version: %d (applied: %t)\n", version, applied)
	return nil
}

func runDown(ctx context.Context, opts options, m *migrator.Migrator) error {
	steps := opts.steps
	if steps == 0 {
		steps = 1
	}
	if steps > 0 {
		steps = -steps
	}

	if err := m.Steps(ctx, steps); err != nil {
		return err
	}

	version, dirty, err := m.Version(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("schema version: %d (dirty: %t)\n", version, dirty)
	return nil
}

func runVersion(ctx context.Context, m *migrator.Migrator, logger *zap.Logger) error {
	version, dirty, err := m.Version(ctx)
	if err != nil {
		return err
	}

	baseline, err := m.BaselineApplied(ctx)
	if err != nil {
		return err
	}

	logger.Info("schema version",
		zap.Uint("version", version),
		zap.Bool("dirty", dirty),
		zap.Bool("baseline_applied", baseline),
	)
	fmt.Printf("schema version: %d (dirty: %t, baseline applied: %t)\n", version, dirty, baseline)
	return nil
}
