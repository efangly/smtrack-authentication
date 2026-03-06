package logger

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

var (
	instance *slog.Logger
	once     sync.Once
)

// Init initializes the global JSON logger for Loki-compatible output.
// level can be "debug", "info", "warn", "error". Defaults to "info".
func Init(level string, serviceName string) {
	once.Do(func() {
		var lvl slog.Level
		switch level {
		case "debug":
			lvl = slog.LevelDebug
		case "warn":
			lvl = slog.LevelWarn
		case "error":
			lvl = slog.LevelError
		default:
			lvl = slog.LevelInfo
		}

		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     lvl,
			AddSource: true,
		})

		instance = slog.New(handler).With(
			slog.String("service", serviceName),
		)

		slog.SetDefault(instance)
	})
}

// Get returns the global logger instance.
func Get() *slog.Logger {
	if instance == nil {
		// Fallback: initialize with defaults if Init was not called
		Init("info", "auth-service")
	}
	return instance
}

// Info logs at INFO level.
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// Warn logs at WARN level.
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// Error logs at ERROR level.
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

// Debug logs at DEBUG level.
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// Fatal logs at ERROR level and exits with code 1.
func Fatal(msg string, args ...any) {
	Get().Error(msg, args...)
	os.Exit(1)
}

// InfoContext logs at INFO level with context.
func InfoContext(ctx context.Context, msg string, args ...any) {
	Get().InfoContext(ctx, msg, args...)
}

// WarnContext logs at WARN level with context.
func WarnContext(ctx context.Context, msg string, args ...any) {
	Get().WarnContext(ctx, msg, args...)
}

// ErrorContext logs at ERROR level with context.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	Get().ErrorContext(ctx, msg, args...)
}

// With returns a new logger with the given attributes.
func With(args ...any) *slog.Logger {
	return Get().With(args...)
}
