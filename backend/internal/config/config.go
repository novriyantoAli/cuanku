// Package config is the single source of truth for runtime configuration.
//
// Precedence (lowest to highest): built-in defaults -> config.yaml (optional)
// -> environment variables. Nothing in this package, and nothing downstream,
// may hardcode a DSN or any other credential: the database URL is read from
// the DATABASE_URL environment variable only (see AGENTS.md — the dev
// PostgreSQL lives in a VM whose address is DHCP-assigned, so a hardcoded
// address would rot).
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Environment names recognised by App.Env.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// DefaultConnectTimeout is the fallback budget for opening the PostgreSQL pool.
const DefaultConnectTimeout = 5 * time.Second

// Config is the root configuration tree.
type Config struct {
	App      App      `mapstructure:"app"`
	Server   Server   `mapstructure:"server"`
	Database Database `mapstructure:"database"`
	Logger   Logger   `mapstructure:"logger"`
	CORS     CORS     `mapstructure:"cors"`
}

// App holds application identity.
type App struct {
	Name    string `mapstructure:"name"`
	Env     string `mapstructure:"env"`
	Version string `mapstructure:"version"`
}

// IsProduction reports whether the app runs with production semantics.
func (a App) IsProduction() bool { return a.Env == EnvProduction }

// Server holds HTTP server settings.
type Server struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// Addr returns the host:port the HTTP server listens on.
func (s Server) Addr() string { return fmt.Sprintf("%s:%d", s.Host, s.Port) }

// Database holds PostgreSQL settings. URL has no default on purpose: an
// unset DSN must fail loudly at startup instead of silently pointing at a
// guessed address.
type Database struct {
	URL             string        `mapstructure:"url"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
}

// Logger holds logging settings.
type Logger struct {
	Level string `mapstructure:"level"`
}

// CORS holds cross-origin settings for the SvelteKit app.
type CORS struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// envBindings maps a config key to the environment variable that overrides it.
// It is a static lookup table, never mutated after init.
var envBindings = map[string][]string{
	"app.name":                 {"APP_NAME"},
	"app.env":                  {"APP_ENV"},
	"app.version":              {"APP_VERSION"},
	"server.host":              {"SERVER_HOST"},
	"server.port":              {"PORT", "SERVER_PORT"},
	"database.url":             {"DATABASE_URL"},
	"database.connect_timeout": {"DATABASE_CONNECT_TIMEOUT"},
	"logger.level":             {"LOGGER_LEVEL"},
	"cors.allowed_origins":     {"CORS_ALLOWED_ORIGINS"},
}

// Load builds a Config from configPath (optional; pass "" to skip the file),
// layered over defaults and overridden by environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	setDefaults(v)

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			var notFound viper.ConfigFileNotFoundError
			if !errors.As(err, &notFound) {
				return nil, fmt.Errorf("read config file %q: %w", configPath, err)
			}
		}
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	for key, envVars := range envBindings {
		if err := v.BindEnv(append([]string{key}, envVars...)...); err != nil {
			return nil, fmt.Errorf("bind env for %q: %w", key, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	cfg.CORS.AllowedOrigins = splitAndTrim(cfg.CORS.AllowedOrigins)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate reports configuration that cannot be used to start a server.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Database.URL) == "" {
		return errors.New("database.url is empty: set the DATABASE_URL environment variable " +
			"(see AGENTS.md — the dev PostgreSQL lives in the VirtualBox VM)")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d is out of range", c.Server.Port)
	}
	return nil
}

// Env returns the effective environment name, defaulting to development.
func (c *Config) Env() string {
	if c.App.Env == "" {
		return EnvDevelopment
	}
	return c.App.Env
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "cuanku-api")
	v.SetDefault("app.env", EnvDevelopment)
	v.SetDefault("app.version", "0.1.0")

	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 15*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.shutdown_timeout", 10*time.Second)

	// No default for database.url: see the Database doc comment.
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", time.Hour)
	v.SetDefault("database.connect_timeout", DefaultConnectTimeout)

	v.SetDefault("logger.level", "info")

	v.SetDefault("cors.allowed_origins", []string{"http://localhost:5173"})
}

func splitAndTrim(values []string) []string {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}
