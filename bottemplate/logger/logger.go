package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

type LogLevel string

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
)

type CustomHandler struct {
	opts      *slog.HandlerOptions
	startTime time.Time
}

func NewHandler() *CustomHandler {
	return &CustomHandler{
		opts:      &slog.HandlerOptions{Level: slog.LevelDebug},
		startTime: time.Now(),
	}
}

func (h *CustomHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *CustomHandler) Handle(_ context.Context, r slog.Record) error {
	timeElapsed := time.Since(h.startTime).Milliseconds()
	timestamp := time.Now().Format("15:04:05")

	var levelColor, levelText string
	switch r.Level {
	case slog.LevelDebug:
		levelColor = colorPurple
		levelText = "DEBUG"
	case slog.LevelInfo:
		levelColor = colorGreen
		levelText = "INFO"
	case slog.LevelWarn:
		levelColor = colorYellow
		levelText = "WARN"
	case slog.LevelError:
		levelColor = colorRed
		levelText = "ERROR"
	}

	// Format: [GoHYE] [15:04:05] [INFO] Message (took 123ms)
	logLine := fmt.Sprintf("%s[GoHYE]%s [%s] [%s%s%s] %s",
		colorBlue, colorReset,
		timestamp,
		levelColor, levelText, colorReset,
		r.Message,
	)

	// Add execution time for non-error messages
	if r.Level != slog.LevelError {
		logLine += fmt.Sprintf(" %s(took %dms)%s", colorCyan, timeElapsed, colorReset)
	}

	// Add any additional attributes
	attrs := make([]string, 0)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key != "" && a.Value.String() != "" {
			attrs = append(attrs, fmt.Sprintf("%s%s=%s%s",
				colorYellow, a.Key, a.Value.String(), colorReset))
		}
		return true
	})

	if len(attrs) > 0 {
		logLine += " ["
		for i, attr := range attrs {
			if i > 0 {
				logLine += " "
			}
			logLine += attr
		}
		logLine += "]"
	}

	fmt.Fprintln(os.Stdout, logLine)
	return nil
}

func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *CustomHandler) WithGroup(name string) slog.Handler {
	return h
}
