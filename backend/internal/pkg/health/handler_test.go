package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/novriyantoAli/cuanku/backend/internal/pkg/health"
)

func newHealthRouter(t *testing.T, pinger health.Pinger) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	health.NewHandler(newChecker(t, pinger)).RegisterRoutes(engine.Group(""))

	return engine
}

func TestGetReturns200AndBodyWhenHealthy(t *testing.T) {
	engine := newHealthRouter(t, &stubPinger{})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusOK, rec.Code)

	var body health.Status
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, health.StatusOK, body.Status)
	assert.Equal(t, health.ComponentUp, body.Database)
}

func TestGetReturns503WhenDatabaseIsDown(t *testing.T) {
	engine := newHealthRouter(t, &stubPinger{err: errUnreachable})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body health.Status
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, health.StatusDegraded, body.Status)
	assert.Equal(t, health.ComponentDown, body.Database)
}
