package migrator_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/novriyantoAli/cuanku/backend/internal/pkg/migrator"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/testutil"
	"github.com/novriyantoAli/cuanku/backend/migrations"
)

// TestUpAppliesBaselineAndIsIdempotent is the end-to-end proof issue #1 asks
// for: the runner reaches the database named by DATABASE_URL, records a
// version, leaves the baseline table behind, and a second run is a no-op.
//
// It skips unless DATABASE_URL is set, so `make test` stays runnable on a
// workstation without the dev VM (AGENTS.md).
func TestUpAppliesBaselineAndIsIdempotent(t *testing.T) {
	db := testutil.RequireDatabase(t)
	logger := testutil.Logger(t)
	m := migrator.New(db, migrations.FS, logger)
	ctx := context.Background()

	version, _, err := m.Up(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, version, uint(1), "the baseline migration must have a version")

	applied, err := m.BaselineApplied(ctx)
	require.NoError(t, err)
	assert.True(t, applied, "the baseline canary table must exist after `up`")

	secondVersion, reapplied, err := m.Up(ctx)
	require.NoError(t, err)
	assert.False(t, reapplied, "re-running `up` must not apply anything")
	assert.Equal(t, version, secondVersion)
}

func TestVersionReportsTheRecordedSchemaVersion(t *testing.T) {
	db := testutil.RequireDatabase(t)
	m := migrator.New(db, migrations.FS, testutil.Logger(t))
	ctx := context.Background()

	_, _, err := m.Up(ctx)
	require.NoError(t, err)

	version, dirty, err := m.Version(ctx)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.GreaterOrEqual(t, version, uint(1))
}
