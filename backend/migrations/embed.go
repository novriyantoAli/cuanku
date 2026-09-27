// Package migrations embeds the versioned SQL migrations for PostgreSQL.
//
// Migrations are plain SQL files, not GORM AutoMigrate: the application writes
// to FreeRADIUS's own tables (radcheck/radreply/radacct) in the same database
// (ADR-0005), so schema changes must be explicit and reviewable rather than
// inferred from struct tags.
//
// File names follow golang-migrate's convention:
// <version>_<name>.up.sql / <version>_<name>.down.sql.
package migrations

import "embed"

// FS holds every migration file, applied in version order by cmd/migration.
//
//go:embed *.sql
var FS embed.FS
