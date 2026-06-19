package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.SugaredLogger. SugaredLogger lets you use printf-style calls
// (e.g. logger.Infow("msg", "key", value)) which are slightly more ergonomic
// than the fully-typed zap.Logger API.
type Logger struct {
	*zap.SugaredLogger
}

// New creates a logger for the given environment.
//   - "production"  → JSON output, Info level by default
//   - anything else → coloured console output, Debug level by default
//
// levelOverride, if non-empty, replaces that default (e.g. "warn" in production during
// an incident, without a redeploy — set via LOG_LEVEL). Valid values are zapcore's level
// names: debug, info, warn, error, dpanic, panic, fatal.
func New(env, levelOverride string) (*Logger, error) {
	var cfg zap.Config
	if env == "production" {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	if levelOverride != "" {
		var level zapcore.Level
		if err := level.UnmarshalText([]byte(levelOverride)); err != nil {
			return nil, fmt.Errorf("invalid LOG_LEVEL %q: %w", levelOverride, err)
		}
		cfg.Level = zap.NewAtomicLevelAt(level)
	}

	zapLogger, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{zapLogger.Sugar()}, nil
}

// Sync flushes any buffered log entries. Call defer logger.Sync() in main().
func (l *Logger) Sync() {
	// zap.SugaredLogger.Sync() returns an error we intentionally ignore on exit.
	_ = l.SugaredLogger.Sync()
}
