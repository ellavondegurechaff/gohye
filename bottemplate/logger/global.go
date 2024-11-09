package logger

import (
	"log/slog"
	"time"
)

// LogCommand logs command execution
func LogCommand(name string, duration time.Duration, err error) {
	attrs := []any{
		slog.String("type", "cmd"),
		slog.String("name", name),
		slog.Duration("took", duration),
	}

	if err != nil {
		slog.Error("Command failed", append(attrs, slog.Any("error", err))...)
	} else {
		slog.Info("Command executed", attrs...)
	}
}

// LogQuery logs database operations
func LogQuery(query string, duration time.Duration, err error) {
	attrs := []any{
		slog.String("type", "db"),
		slog.Duration("took", duration),
	}

	if err != nil {
		slog.Error("Query failed", append(attrs,
			slog.String("query", query),
			slog.Any("error", err),
		)...)
	} else {
		slog.Info("Query executed", append(attrs,
			slog.String("query", query),
		)...)
	}
}

// LogSystem logs system events
func LogSystem(msg string, attrs ...any) {
	baseAttrs := []any{slog.String("type", "sys")}
	slog.Info(msg, append(baseAttrs, attrs...)...)
}

// LogError logs error events
func LogError(msg string, err error, attrs ...any) {
	baseAttrs := []any{
		slog.String("type", "error"),
		slog.Any("error", err),
	}
	slog.Error(msg, append(baseAttrs, attrs...)...)
}
