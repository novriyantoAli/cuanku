package api_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/novriyantoAli/cuanku/backend/internal/config"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/health"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/testutil"
	"github.com/novriyantoAli/cuanku/backend/internal/server/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TestModuleStartsTheServerOverTCP is the test that would have caught the API
// never starting at all.
//
// Fx constructs only the providers an Invoke needs. While api.Module merely
// *offered* *Server, nothing ever asked for it, so NewServer never ran, its
// fx.Hook was never registered, Start() had nothing to start, and the process
// printed "[Fx] RUNNING" forever without listening on anything. Asserting on the
// graph (app.Err() == nil) cannot see that: an empty graph builds fine. Only
// making a real request can.
func TestModuleStartsTheServerOverTCP(t *testing.T) {
	port := freePort(t)

	application := fx.New(
		fx.NopLogger,
		fx.Supply(bindableConfig(t, port), testutil.Logger(t), &gorm.DB{}),
		// Swap the health seam so this test needs no database, and so the status
		// it asserts is deterministic. The real NewDBPinger is still constructed
		// (it only stores the handle) — it is simply never called. Supplying a
		// zero *gorm.DB is not an option for calling it: gorm v1.31.2 panics in
		// (*DB).DB() on a handle that was never opened.
		fx.Decorate(func(health.Pinger) health.Pinger { return stubPinger{} }),
		api.Module,
		fx.StartTimeout(10*time.Second),
		fx.StopTimeout(10*time.Second),
	)
	require.NoError(t, application.Err())
	require.NoError(t, application.Start(context.Background()))
	t.Cleanup(func() { _ = application.Stop(context.Background()) })

	// Reaching this line requires the whole chain to be real: a bound listener on
	// the port, the Gin engine, the global middleware, and the mounted route. 200
	// is the pinger above answering.
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	require.NoError(t, err, "including api.Module must produce a listening server")
	t.Cleanup(func() { _ = resp.Body.Close() })

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// freePort asks the kernel for an unused port and then releases it.
//
// There is a window between releasing it and the server binding it. That is the
// price of this test being able to fail: reading the bound address back would
// mean asking the graph for *api.Server, and asking for it is itself an Invoke —
// which constructs the server and registers its lifecycle hook, hiding exactly
// the bug this test exists to catch.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	return port
}

// TestModuleGraphBuilds composes the real Fx graph of the API server — Module,
// not NewServer called by hand.
//
// It needs no database: NewDBPinger stores the handle and never touches it, and
// nothing is started because fx.New does not run lifecycle hooks.
func TestModuleGraphBuilds(t *testing.T) {
	application := fx.New(
		fx.NopLogger,
		fx.Supply(testutil.Config(t), testutil.Logger(t), &gorm.DB{}),
		api.Module,
	)

	require.NoError(t, application.Err(), "the API Fx graph must build without a database")
}

// TestModuleProvidesEveryConstructorTheServerNeeds pins the names the graph
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

// bindableConfig is a config whose server can actually bind.
func bindableConfig(t *testing.T, port int) *config.Config {
	t.Helper()

	cfg := testutil.Config(t)
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = port

	return cfg
}
