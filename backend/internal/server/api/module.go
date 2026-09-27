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
	// Fx constructs only the providers some Invoke needs. Without this, nothing
	// ever asks for *Server, so NewServer never runs, its fx.Hook is never
	// registered, Start() has nothing to start, and the process sits there
	// printing "[Fx] RUNNING" while listening on nothing. Asking for the value
	// here makes "include this module" and "start this server" the same thing —
	// so an entrypoint cannot forget it, and a test that composes the module
	// exercises the real startup path.
	fx.Invoke(func(*Server) {}),
)
