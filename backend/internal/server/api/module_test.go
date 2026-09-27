package api_test

import (
	"testing"

	"github.com/novriyantoAli/cuanku/backend/internal/config"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/health"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/testutil"
	"github.com/novriyantoAli/cuanku/backend/internal/server/api"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TestModuleGraphBuilds composes the real Fx graph of the API server — Module,
// not NewServer called by hand.
//
// Without this, a wiring mistake in Module (a missing provider, a mis-annotated
// interface, a constructor that no longer type-checks against its consumers)
// only shows up when someone runs `cmd/api` against a live database, as a fatal
// bootstrap error. The test needs no database: NewDBPinger stores the handle and
// never touches it, and nothing is started because fx.New does not run
// lifecycle hooks.
func TestModuleGraphBuilds(t *testing.T) {
	application := fx.New(
		fx.NopLogger,
		fx.Supply(testutil.Config(t), testutil.Logger(t), &gorm.DB{}),
		api.Module,
	)

	require.NoError(t, application.Err(), "the API Fx graph must build without a database")
}

// TestModuleProvidesEveryConstructorTheServerNeeds pins the two names the graph
// resolves by type. If someone changes a constructor's return type, this fails
// here rather than at boot.
func TestModuleProvidesEveryConstructorTheServerNeeds(t *testing.T) {
	var (
		gotConfig  *config.Config
		gotLogger  *zap.Logger
		gotChecker *health.Checker
		gotServer  *api.Server
	)

	application := fx.New(
		fx.NopLogger,
		fx.Supply(testutil.Config(t), testutil.Logger(t), &gorm.DB{}),
		api.Module,
		fx.Populate(&gotConfig, &gotLogger, &gotChecker, &gotServer),
	)

	require.NoError(t, application.Err())
	require.NotNil(t, gotConfig)
	require.NotNil(t, gotLogger)
	require.NotNil(t, gotChecker)
	require.NotNil(t, gotServer, "the graph must produce a *api.Server to start")
}
