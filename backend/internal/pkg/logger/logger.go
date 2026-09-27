// Package logger builds the application-wide structured logger.
package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/novriyantoAli/cuanku/backend/internal/config"
)

// New builds a zap logger from configuration. Production uses JSON at the
// configured level; development uses human-readable console output.
func New(cfg *config.Config) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Logger.Level)
	if err != nil {
		return nil, fmt.Errorf("parse logger level %q: %w", cfg.Logger.Level, err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if cfg.App.IsProduction() {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	} else {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	}

	core := zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), level)

	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}
