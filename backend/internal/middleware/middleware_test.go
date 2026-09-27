package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/novriyantoAli/cuanku/backend/internal/middleware"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	engine := gin.New()
	engine.Use(middleware.CORS([]string{"http://localhost:5173"}))
	engine.GET("/probe", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	assert.Equal(t, "http://localhost:5173", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", rec.Header().Get("Vary"))
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	engine := gin.New()
	engine.Use(middleware.CORS([]string{"http://localhost:5173"}))
	engine.GET("/probe", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSAnswersPreflightWithoutReachingTheHandler(t *testing.T) {
	reached := false
	engine := gin.New()
	engine.Use(middleware.CORS([]string{"http://localhost:5173"}))
	engine.OPTIONS("/probe", func(ctx *gin.Context) { reached = true })

	req := httptest.NewRequest(http.MethodOptions, "/probe", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.False(t, reached, "preflight must be answered by the middleware, not the handler")
}

func TestRecoveryTurnsPanicInto500AndHidesTheStack(t *testing.T) {
	engine := gin.New()
	engine.Use(middleware.Recovery(zap.NewNop()))
	engine.GET("/boom", func(*gin.Context) { panic("secret internal detail") })

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"internal server error"}`, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "secret internal detail")
}

func TestLoggerRecordsTheResponseStatus(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	engine := gin.New()
	engine.Use(middleware.Logger(zap.New(core)), middleware.Recovery(zap.New(core)))
	engine.GET("/probe", func(ctx *gin.Context) { ctx.Status(http.StatusTeapot) })

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe", nil))

	require.Equal(t, http.StatusTeapot, rec.Code)
	require.Equal(t, 1, logs.Len())

	entry := logs.All()[0]
	assert.Equal(t, zapcore.WarnLevel, entry.Level, "4xx must not be logged as a server error")
	assert.Equal(t, "/probe", entry.ContextMap()["path"])
	assert.EqualValues(t, http.StatusTeapot, entry.ContextMap()["status"])
}
