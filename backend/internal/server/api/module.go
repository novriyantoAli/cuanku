package api

import (
	"go.uber.org/fx"

	"github.com/novriyantoAli/cuanku/backend/internal/pkg/health"
)

// Module is the Fx wiring for the REST transport: the only place that knows
// the API server's dependency graph. main.go just composes Modules, so a new
// domain adds its handler constructors here (or to its own domain Module,
// which is then aggregated in this list) and nothing else.
var Module = fx.Options(
	fx.Provide(
		// Expose the GORM-backed pinger under the narrow interface the checker
		// consumes — the "interface next to implementation" rule (Golden Rule 3).
		fx.Annotate(health.NewDBPinger, fx.As(new(health.Pinger))),
		ProvideHealthChecker,
		health.NewHandler,
		NewServer,
	),
)
