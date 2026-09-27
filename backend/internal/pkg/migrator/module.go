package migrator

import (
	"io/fs"

	"go.uber.org/fx"

	"github.com/novriyantoAli/cuanku/backend/migrations"
)

// Module is the Fx wiring for the migration runner. It exposes the embedded
// migrations as an fs.FS so Migrator never imports them directly.
var Module = fx.Options(
	fx.Provide(
		provideSource,
		New,
	),
)

func provideSource() fs.FS { return migrations.FS }
