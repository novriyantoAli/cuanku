// Package health exposes the liveness/readiness surface of the API.
//
// It is deliberately infrastructure rather than a DDD domain: it owns no
// business concept from CONTEXT.md, has no persistence of its own, and every
// other domain's availability depends on it.
package health

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/novriyantoAli/cuanku/backend/internal/pkg/database"
)

// Component status values, mirrored 1:1 by the frontend zod schema in
// frontend/src/lib/domains/health/schemas/health.schema.ts.
const (
	StatusOK       = "ok"
	StatusDegraded = "degraded"

	ComponentUp   = "up"
	ComponentDown = "down"
)

// DatabaseTimeout bounds how long a health check waits on PostgreSQL.
const DatabaseTimeout = 3 * time.Second

// Pinger is the only thing the checker needs from a dependency: the ability to
// answer "are you reachable?". Declaring it here (consumer side) is what makes
// a 200/503 test a pure unit test with no database behind it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Status is the health report returned to callers.
type Status struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	Database  string    `json:"database"`
	CheckedAt time.Time `json:"checked_at"`
}

// Healthy reports whether every dependency answered.
func (s Status) Healthy() bool { return s.Status == StatusOK }

// Checker probes the API's dependencies.
type Checker struct {
	db      Pinger
	service string
	version string
	logger  *zap.Logger
}

// CheckerConfig carries the identity fields reported by the health endpoint.
type CheckerConfig struct {
	Service string
	Version string
}

// NewChecker builds a Checker.
func NewChecker(db Pinger, cfg CheckerConfig, logger *zap.Logger) *Checker {
	return &Checker{
		db:      db,
		service: cfg.Service,
		version: cfg.Version,
		logger:  logger,
	}
}

// Check pings the database and reports the aggregate status. It never returns
// a nil Status: a failing dependency degrades the report, it does not error.
func (c *Checker) Check(ctx context.Context) Status {
	status := Status{
		Status:    StatusOK,
		Service:   c.service,
		Version:   c.version,
		Database:  ComponentUp,
		CheckedAt: time.Now().UTC(),
	}

	pingCtx, cancel := context.WithTimeout(ctx, DatabaseTimeout)
	defer cancel()

	if err := c.db.Ping(pingCtx); err != nil {
		c.logger.Warn("health check: database unreachable", zap.Error(err))
		status.Status = StatusDegraded
		status.Database = ComponentDown
	}

	return status
}

// DBPinger adapts the application's *gorm.DB to Pinger. It is the only part of
// this package that knows GORM exists.
type DBPinger struct {
	db *gorm.DB
}

// NewDBPinger builds the GORM-backed Pinger.
func NewDBPinger(db *gorm.DB) *DBPinger { return &DBPinger{db: db} }

// Ping delegates to the shared database helper so pool semantics stay in one place.
func (p *DBPinger) Ping(ctx context.Context) error { return database.Ping(ctx, p.db) }
