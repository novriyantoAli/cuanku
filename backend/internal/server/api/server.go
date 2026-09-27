// Package api wires and runs the HTTP server. It is the composition root for
// the REST transport: domains register their handlers here and nothing else.
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/novriyantoAli/cuanku/backend/internal/config"
	"github.com/novriyantoAli/cuanku/backend/internal/middleware"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/health"
)

// Routed is the seam every HTTP handler in this repo satisfies: a thin
// adapter that mounts itself on a router group. Health implements it today;
// every domain handler implements it as it lands (#2 onwards).
type Routed interface {
	RegisterRoutes(router *gin.RouterGroup)
}

// Server owns the Gin engine and its lifecycle.
type Server struct {
	engine *gin.Engine
	http   *http.Server
	cfg    *config.Config
	logger *zap.Logger
}

// NewServer builds the HTTP server, registers global middleware, and mounts
// every route. Domain handlers are injected here as they are added (see
// AGENTS.md for how a new domain registers itself).
func NewServer(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger, healthHandler *health.Handler) *Server {
	if cfg.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(
		middleware.Recovery(logger),
		middleware.Logger(logger),
		middleware.CORS(cfg.CORS.AllowedOrigins),
	)

	s := &Server{
		engine: engine,
		cfg:    cfg,
		logger: logger,
		http: &http.Server{
			Addr:         cfg.Server.Addr(),
			Handler:      engine,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
	}

	s.setupRoutes(healthHandler)

	lc.Append(fx.Hook{
		OnStart: s.start,
		OnStop:  s.stop,
	})

	return s
}

// Engine exposes the router for tests.
func (s *Server) Engine() *gin.Engine { return s.engine }

func (s *Server) setupRoutes(healthHandler *health.Handler) {
	// Liveness/readiness live outside the versioned API surface so probes
	// never break when the API version changes.
	healthHandler.RegisterRoutes(s.engine.Group(""))

	// Versioned business API. The /api/v1 prefix is defined exactly once, so
	// a new domain registers itself by adding its handler to this slice — see
	// AGENTS.md, "Registering a new domain".
	v1 := s.engine.Group("/api/v1")
	for _, handler := range []Routed{} {
		handler.RegisterRoutes(v1)
	}
}

func (s *Server) start(_ context.Context) error {
	s.logger.Info("http server starting",
		zap.String("addr", s.http.Addr),
		zap.String("env", s.cfg.Env()),
		zap.String("version", s.cfg.App.Version),
	)

	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("http server stopped unexpectedly", zap.Error(err))
		}
	}()

	return nil
}

func (s *Server) stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, s.cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	s.logger.Info("http server stopped")
	return nil
}
