package api

import (
	"go.uber.org/zap"

	"github.com/novriyantoAli/cuanku/backend/internal/config"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/health"
)

// ProvideHealthChecker adapts configuration into the identity fields the
// health endpoint reports. It lives here (not in the health package) because
// only the REST transport serves health.
func ProvideHealthChecker(pinger health.Pinger, cfg *config.Config, logger *zap.Logger) *health.Checker {
	return health.NewChecker(pinger, health.CheckerConfig{
		Service: cfg.App.Name,
		Version: cfg.App.Version,
	}, logger)
}
