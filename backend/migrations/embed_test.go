package migrations_test

import (
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/novriyantoAli/cuanku/backend/migrations"
)

var migrationName = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.(up|down)\.sql$`)

// TestEveryMigrationIsPaired guards golang-migrate's contract: a version
// without a .down.sql cannot be rolled back, and a gap in the version sequence
// makes `migrate down` ambiguous.
func TestEveryMigrationIsPaired(t *testing.T) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	require.NoError(t, err)

	up := map[int]bool{}
	down := map[int]bool{}

	for _, entry := range entries {
		matches := migrationName.FindStringSubmatch(entry.Name())
		require.NotNilf(t, matches, "migration %q does not match <version>_<name>.up|down.sql", entry.Name())

		version, convErr := strconv.Atoi(matches[1])
		require.NoError(t, convErr)

		if matches[3] == "up" {
			up[version] = true
		} else {
			down[version] = true
		}
	}

	require.NotEmpty(t, up, "the repository must ship at least the baseline migration")

	versions := make([]int, 0, len(up))
	for version := range up {
		versions = append(versions, version)
	}
	sort.Ints(versions)

	for i, version := range versions {
		assert.True(t, down[version], "migration %d has no .down.sql", version)
		assert.Equal(t, i+1, version, "migration versions must be contiguous starting at 1")
	}
}
