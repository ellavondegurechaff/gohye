package logger

import (
	"log/slog"
	"time"
)

type QueryLogger struct {
	Operation string
	Query     string
	Args      []interface{}
	StartTime time.Time
}

func NewQueryLogger(operation, query string, args ...any) *QueryLogger {
	return &QueryLogger{
		Operation: operation,
		Query:     query,
		Args:      args,
		StartTime: time.Now(),
	}
}

func (l *QueryLogger) Log(err error, rowsAffected int64) {
	duration := time.Since(l.StartTime)

	if err != nil {
		slog.Error("Query failed",
			slog.String("type", "db"),
			slog.String("operation", l.Operation),
			slog.String("query", l.Query),
			slog.Any("args", l.Args),
			slog.Duration("took", duration),
			slog.Any("error", err),
		)
		return
	}

	slog.Info("Query executed",
		slog.String("type", "db"),
		slog.String("operation", l.Operation),
		slog.String("query", l.Query),
		slog.Any("args", l.Args),
		slog.Duration("took", duration),
		slog.Int64("affected_rows", rowsAffected),
	)
}
