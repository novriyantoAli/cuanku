package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/novriyantoAli/cuanku/backend/internal/pkg/health"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/testutil"
	"github.com/novriyantoAli/cuanku/backend/internal/server/api"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// stubPinger satisfies the health seam without a database.
type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

// recordingLifecycle captures the hooks NewServer registers, so a wiring
// regression (server never starts, or never shuts down) fails a test here.
type recordingLifecycle struct {
	hooks []fx.Hook
}

func (l *recordingLifecycle) Append(hook fx.Hook) { l.hooks = append(l.hooks, hook) }

func newTestServer(t *testing.T, pinger health.Pinger) (*api.Server, *recordingLifecycle) {
	t.Helper()

	cfg := testutil.Config(t)
	lc := &recordingLifecycle{}
	checker := health.NewChecker(pinger, health.CheckerConfig{
		Service: cfg.App.Name,
		Version: cfg.App.Version,
	}, testutil.Logger(t))

	server := api.NewServer(lc, cfg, testutil.Logger(t), health.NewHandler(checker))

	return server, lc
}

func get(t *testing.T, server *api.Server, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	server.Engine().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec
}

func TestNewServerRegistersStartAndStopHooks(t *testing.T) {
	_, lc := newTestServer(t, stubPinger{})

	require.Len(t, lc.hooks, 1)
	assert.NotNil(t, lc.hooks[0].OnStart, "the server must register an OnStart hook")
	assert.NotNil(t, lc.hooks[0].OnStop, "the server must register an OnStop hook")
}

func TestHealthzIsMountedAtRoot(t *testing.T) {
	server, _ := newTestServer(t, stubPinger{})

	rec := get(t, server, "/healthz")

	require.Equal(t, http.StatusOK, rec.Code)

	var body health.Status
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, health.StatusOK, body.Status)
	assert.Equal(t, "cuanku-test", body.Service)
	assert.Equal(t, "test", body.Version)
}

// Probes must not move when the business API version changes.
func TestHealthzIsNotServedUnderTheVersionedAPI(t *testing.T) {
	server, _ := newTestServer(t, stubPinger{})

	rec := get(t, server, "/api/v1/healthz")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHealthzReports503WhenTheDatabaseIsDown(t *testing.T) {
	server, _ := newTestServer(t, stubPinger{err: errors.New("connection refused")})

	rec := get(t, server, "/healthz")

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
