package logger

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
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

type LogType string

const (
	TypeCommand LogType = "CMD"
	TypeDB      LogType = "DB"
	TypeSystem  LogType = "SYS"
	TypeError   LogType = "ERR"
)

type CustomHandler struct {
	opts        *slog.HandlerOptions
	startTime   time.Time
	attrs       []slog.Attr
	serviceName string
}

func NewHandler(serviceName string) *CustomHandler {
	return &CustomHandler{
		opts:        &slog.HandlerOptions{Level: slog.LevelInfo},
		startTime:   time.Now(),
		attrs:       make([]slog.Attr, 0),
		serviceName: serviceName,
	}
}

func (h *CustomHandler) Handle(_ context.Context, r slog.Record) error {
	// Skip debug logs by default
	if r.Level == slog.LevelDebug {
		return nil
	}

	// Skip noisy logs
	if shouldSkipLog(&r) {
		return nil
	}

	timestamp := time.Now().Format("15:04:05.000")
	logType := getLogType(&r)

	// Format: [TIME][LEVEL][TYPE] Message {metadata}
	fmt.Printf("[%s][%s][%s] %s%s\n",
		timestamp,
		h.formatLevel(r.Level),
		logType,
		h.buildMessage(&r),
		h.buildMetadata(&r),
	)

	return nil
}

func (h *CustomHandler) formatLevel(level slog.Level) string {
	var color, text string
	switch level {
	case slog.LevelDebug:
		color, text = colorPurple, "DBG"
	case slog.LevelInfo:
		color, text = colorGreen, "INF"
	case slog.LevelWarn:
		color, text = colorYellow, "WRN"
	case slog.LevelError:
		color, text = colorRed, "ERR"
	}
	return fmt.Sprintf("%s%s%s", color, text, colorReset)
}

func (h *CustomHandler) buildMessage(r *slog.Record) string {
	parts := []string{r.Message}

	// Add context information
	if cmdName := getCommandName(r); cmdName != "" {
		if userName := getUserName(r); userName != "" {
			parts = append(parts, fmt.Sprintf("(%s by %s)", cmdName, userName))
		}
	}

	// Add error context
	if r.Level == slog.LevelError {
		if loc := getErrorLocation(r); loc != "" {
			parts = append(parts, fmt.Sprintf("at %s", loc))
		}
		if err := getErrorDetails(r); err != "" {
			parts = append(parts, fmt.Sprintf("error=%s", err))
		}
	}

	return strings.Join(parts, " ")
}

func (h *CustomHandler) buildMetadata(r *slog.Record) string {
	metadata := make(map[string]string)

	// Add duration if present
	if took := time.Since(h.startTime).Milliseconds(); took > 0 {
		metadata["took"] = fmt.Sprintf("%dms", took)
	}

	// Add important attributes
	r.Attrs(func(a slog.Attr) bool {
		if !isInternalAttr(a.Key) && a.Value.String() != "" {
			metadata[a.Key] = a.Value.String()
		}
		return true
	})

	if len(metadata) == 0 {
		return ""
	}

	parts := make([]string, 0, len(metadata))
	for k, v := range metadata {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return fmt.Sprintf(" {%s}", strings.Join(parts, ", "))
}

func shouldSkipLog(r *slog.Record) bool {
	// Skip common noise
	noisePatterns := []string{
		"ratelimit",
		"bucket",
		"gateway event",
		"binary message",
		"heartbeat",
	}

	msg := strings.ToLower(r.Message)
	for _, pattern := range noisePatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

func getLogType(r *slog.Record) LogType {
	var logType LogType = TypeSystem
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "type" {
			switch a.Value.String() {
			case "cmd":
				logType = TypeCommand
			case "db":
				logType = TypeDB
			case "error":
				logType = TypeError
			}
			return false
		}
		return true
	})
	return logType
}

func getSourceLocation() (string, int) {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "", 0
	}
	return filepath.Base(file), line
}

func isInternalAttr(key string) bool {
	internal := []string{"type", "name", "user_name", "status"}
	for _, k := range internal {
		if k == key {
			return true
		}
	}
	return false
}

func getStatus(r *slog.Record) string {
	var status string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "status" {
			status = a.Value.String()
			return false
		}
		return true
	})
	return status
}

func getUserName(r *slog.Record) string {
	var name string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "user_name" {
			name = a.Value.String()
			return false
		}
		return true
	})
	return name
}

func getCommandName(r *slog.Record) string {
	var name string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "name" {
			name = a.Value.String()
			return false
		}
		return true
	})
	return name
}

func getErrorDetails(r *slog.Record) string {
	var details string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "error" {
			details = fmt.Sprintf("%v", a.Value)
			return false
		}
		return true
	})
	return details
}

func getErrorLocation(r *slog.Record) string {
	var location string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "error_location" {
			location = a.Value.String()
			return false
		}
		return true
	})
	if location == "" && r.Level == slog.LevelError {
		if file, line := getSourceLocation(); file != "" {
			location = fmt.Sprintf("%s:%d", file, line)
		}
	}
	return location
}

func (h *CustomHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CustomHandler{
		opts:        h.opts,
		startTime:   h.startTime,
		attrs:       append(h.attrs, attrs...),
		serviceName: h.serviceName,
	}
}

func (h *CustomHandler) WithGroup(name string) slog.Handler {
	return &CustomHandler{
		opts:        h.opts,
		startTime:   h.startTime,
		attrs:       h.attrs,
		serviceName: h.serviceName,
	}
}
