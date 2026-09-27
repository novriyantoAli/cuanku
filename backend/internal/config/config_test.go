package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/novriyantoAli/cuanku/backend/internal/config"
)

// testDSN carries deliberately no userinfo: these tests are about configuration
// plumbing, not credentials, and a credential-shaped literal is both a gosec G101
// finding and a bad example in a public repository. The pass-through behaviour
// that actually matters is covered separately by
// TestDSNValueIsPassedThroughVerbatim.
const testDSN = "postgres://127.0.0.1:5432/cuanku_test?sslmode=disable"

func TestLoadReadsDSNFromEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", testDSN)

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, testDSN, cfg.Database.URL, "the DSN must come from DATABASE_URL")
}

func TestLoadFailsWithoutDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := config.Load("")

	require.Error(t, err, "an unset DSN must fail loudly instead of guessing an address")
	assert.Contains(t, err.Error(), "DATABASE_URL", "the error must name the variable to set")
}

// The DSN has no default in code or in config.sample.yaml: this guards against
// someone reintroducing one (the dev VM address is DHCP-assigned).
func TestDSNHasNoDefaultWhenNotConfigured(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	cfg, err := config.Load("")

	assert.Nil(t, cfg)
	require.Error(t, err)
}

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", testDSN)

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, config.EnvDevelopment, cfg.Env())
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "0.0.0.0:8080", cfg.Server.Addr())
	assert.Equal(t, 25, cfg.Database.MaxOpenConns)
	assert.Equal(t, config.DefaultConnectTimeout, cfg.Database.ConnectTimeout)
	assert.Equal(t, "info", cfg.Logger.Level)
	assert.Equal(t, []string{"http://localhost:5173"}, cfg.CORS.AllowedOrigins)
}

func TestEnvironmentOverridesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", testDSN)
	t.Setenv("PORT", "9090")
	t.Setenv("APP_ENV", config.EnvProduction)
	t.Setenv("LOGGER_LEVEL", "warn")
	t.Setenv("SERVER_HOST", "127.0.0.1")

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "127.0.0.1:9090", cfg.Server.Addr())
	assert.True(t, cfg.App.IsProduction())
	assert.Equal(t, "warn", cfg.Logger.Level)
}

func TestCORSOriginsAreSplitAndTrimmedFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", testDSN)
	t.Setenv("CORS_ALLOWED_ORIGINS", " http://localhost:5173, https://cuanku.example ,,")

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, []string{"http://localhost:5173", "https://cuanku.example"}, cfg.CORS.AllowedOrigins)
}

// Viper must hand the DSN through exactly as the environment had it. An earlier
// version of this test asserted only that *some* value arrived, which cannot fail
// when viper rewrites the string — and the DSN is a URL, so a rewritten `#`, `$`
// or `%` is a connection that fails in a way nobody can read.
func TestDSNValueIsPassedThroughVerbatim(t *testing.T) {
	const nasty = "not a url: $HOME/still#here?x=%20 & quoted 'value'"
	t.Setenv("DATABASE_URL", nasty)

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, nasty, cfg.Database.URL)
}

func TestConfigFileIsReadButEnvironmentWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "server:\n  port: 7000\nlogger:\n  level: debug\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	t.Setenv("DATABASE_URL", testDSN)
	t.Setenv("LOGGER_LEVEL", "error")

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, 7000, cfg.Server.Port, "the file wins over the built-in default")
	assert.Equal(t, "error", cfg.Logger.Level, "the environment wins over the file")
}

func TestLoadFailsOnMissingConfigFile(t *testing.T) {
	t.Setenv("DATABASE_URL", testDSN)

	_, err := config.Load(filepath.Join(t.TempDir(), "nope.yaml"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config file")
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	cfg := &config.Config{Server: config.Server{Port: 70000}}
	cfg.Database.URL = testDSN

	err := cfg.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.port")
}

func TestConfigKeepsTimeDurations(t *testing.T) {
	t.Setenv("DATABASE_URL", testDSN)

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, 15*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, time.Hour, cfg.Database.ConnMaxLifetime)
}
