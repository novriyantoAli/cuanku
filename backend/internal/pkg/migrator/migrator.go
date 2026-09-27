// Package migrator applies the embedded SQL migrations to PostgreSQL.
package migrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/novriyantoAli/cuanku/backend/internal/pkg/database"
)

// BaselineTable is the canary table created by the baseline migration. The
// runner reads it back to confirm a migration run actually changed the schema.
const BaselineTable = "app_schema_baseline"

// ErrBaselineMissing means the schema is reachable but the baseline migration
// has not been applied — the application is pointing at an unmigrated database.
var ErrBaselineMissing = errors.New("schema baseline missing: run `make migrate-up`")

// Migrator applies migrations using the connection the application already
// owns, so a migration and the API always talk to the same DSN.
type Migrator struct {
	db     *gorm.DB
	source fs.FS
	logger *zap.Logger
}

// New builds a Migrator over the given migration source (migrations.FS).
func New(db *gorm.DB, source fs.FS, logger *zap.Logger) *Migrator {
	return &Migrator{db: db, source: source, logger: logger}
}

// Up applies every pending migration. It reports the version reached and
// whether anything was actually applied (false means "already up to date").
func (m *Migrator) Up(ctx context.Context) (version uint, applied bool, err error) {
	mig, err := m.newInstance(ctx)
	if err != nil {
		return 0, false, err
	}

	if err := mig.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return 0, false, fmt.Errorf("apply migrations: %w", err)
		}

		version, _, err := m.currentVersion(mig)
		if err != nil {
			return 0, false, err
		}
		m.logger.Info("schema already up to date", zap.Uint("version", version))
		return version, false, nil
	}

	version, dirty, err := m.currentVersion(mig)
	if err != nil {
		return 0, false, err
	}
	if dirty {
		return version, true, fmt.Errorf("schema version %d is dirty: a migration failed midway", version)
	}

	applied, err = m.BaselineApplied(ctx)
	if err != nil {
		return version, true, err
	}
	if !applied {
		return version, true, fmt.Errorf("%w (schema version %d)", ErrBaselineMissing, version)
	}

	return version, true, nil
}

// Steps applies (positive n) or rolls back (negative n) at most n migrations.
// n == 0 applies everything pending, matching `migrate up`.
func (m *Migrator) Steps(ctx context.Context, n int) error {
	mig, err := m.newInstance(ctx)
	if err != nil {
		return err
	}

	if n == 0 {
		return mig.Up()
	}

	if err := mig.Steps(n); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.logger.Info("schema already at requested version")
			return nil
		}
		return fmt.Errorf("apply %d migration steps: %w", n, err)
	}
	return nil
}

// Version returns the current schema version and whether it is dirty.
func (m *Migrator) Version(ctx context.Context) (version uint, dirty bool, err error) {
	mig, err := m.newInstance(ctx)
	if err != nil {
		return 0, false, err
	}
	return m.currentVersion(mig)
}

// BaselineApplied reports whether the baseline migration's canary table exists.
func (m *Migrator) BaselineApplied(ctx context.Context) (bool, error) {
	var exists bool
	err := m.db.WithContext(ctx).
		Raw("SELECT to_regclass(?) IS NOT NULL", BaselineTable).
		Scan(&exists).Error
	if err != nil {
		return false, fmt.Errorf("check %s: %w", BaselineTable, err)
	}
	return exists, nil
}

func (m *Migrator) currentVersion(mig migrationInstance) (uint, bool, error) {
	version, dirty, err := mig.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, fmt.Errorf("read schema version: %w", err)
	}
	return version, dirty, nil
}

// newInstance builds a migrate handle that reuses the application's *sql.DB.
// The handle is deliberately not Closed: migrate's Close also closes the
// underlying pool, which the caller still owns.
func (m *Migrator) newInstance(ctx context.Context) (migrationInstance, error) {
	sqlDB, err := database.Conn(m.db)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database before migrating: %w", err)
	}

	sourceDriver, err := iofs.New(m.source, ".")
	if err != nil {
		return nil, fmt.Errorf("open embedded migrations: %w", err)
	}

	dbDriver, err := migratepgx.WithInstance(sqlDB, &migratepgx.Config{})
	if err != nil {
		return nil, fmt.Errorf("build migration database driver: %w", err)
	}

	mig, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return nil, fmt.Errorf("build migrator: %w", err)
	}

	return mig, nil
}

// migrationInstance is the narrow surface of *migrate.Migrate the Migrator needs.
type migrationInstance interface {
	Up() error
	Steps(n int) error
	Version() (uint, bool, error)
}
