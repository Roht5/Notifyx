package logger

import (
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
//   - "production"  → JSON output, Info level
//   - anything else → coloured console output, Debug level
func New(env string) (*Logger, error) {
	var zapLogger *zap.Logger
	var err error

	if env == "production" {
		cfg := zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		zapLogger, err = cfg.Build()
	} else {
		cfg := zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		zapLogger, err = cfg.Build()
	}

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
