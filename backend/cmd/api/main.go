// Command api runs the Cuanku HTTP API.
package main

import (
	"log"
	"time"

	"go.uber.org/fx"

	"github.com/novriyantoAli/cuanku/backend/internal/core"
	"github.com/novriyantoAli/cuanku/backend/internal/server/api"
)

func main() {
	application := fx.New(
		core.CoreModule,
		api.Module,
		fx.StartTimeout(30*time.Second),
		fx.StopTimeout(20*time.Second),
	)

	if err := application.Err(); err != nil {
		// The logger (and therefore the config) may be exactly what failed to
		// build, so report through the standard library here.
		log.Fatalf("cuanku-api: failed to bootstrap: %v", err)
	}

	application.Run()
}
