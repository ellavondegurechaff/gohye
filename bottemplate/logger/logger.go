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
	opts      *slog.HandlerOptions
	startTime time.Time
	attrs     []slog.Attr
	groups    []string
}

func NewHandler() *CustomHandler {
	return &CustomHandler{
		opts:      &slog.HandlerOptions{Level: slog.LevelDebug},
		startTime: time.Now(),
		attrs:     make([]slog.Attr, 0),
		groups:    make([]string, 0),
	}
}

func (h *CustomHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CustomHandler{
		opts:      h.opts,
		startTime: h.startTime,
		attrs:     append(h.attrs, attrs...),
		groups:    h.groups,
	}
}

func (h *CustomHandler) WithGroup(name string) slog.Handler {
	return &CustomHandler{
		opts:      h.opts,
		startTime: h.startTime,
		attrs:     h.attrs,
		groups:    append(h.groups, name),
	}
}

func (h *CustomHandler) Handle(_ context.Context, r slog.Record) error {
	if shouldSkipLog(&r) {
		return nil
	}

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

	// Get log type and additional information
	logType := getLogType(&r)
	status := getStatus(&r)
	userName := getUserName(&r)
	cmdName := getCommandName(&r)
	errorDetails := getErrorDetails(&r)
	errorLocation := getErrorLocation(&r)

	// Format message with source info for errors
	message := r.Message
	if r.Level == slog.LevelError {
		if errorLocation != "" {
			message = fmt.Sprintf("%s (%s)", message, errorLocation)
		}
		if errorDetails != "" {
			message = fmt.Sprintf("%s: %s", message, errorDetails)
		}
	}

	// Add command and user info if available
	if cmdName != "" && userName != "" {
		message = fmt.Sprintf("%s [%s by %s]", message, cmdName, userName)
	}

	// Add status if available
	if status != "" {
		message = fmt.Sprintf("%s [Status: %s]", message, status)
	}

	// Add elapsed time for performance tracking
	if timeElapsed > 0 {
		message = fmt.Sprintf("%s (took %dms)", message, timeElapsed)
	}

	// Build attributes string
	var attrsStr string
	if len(h.attrs) > 0 {
		for _, attr := range h.attrs {
			if !isInternalAttr(attr.Key) {
				attrsStr += fmt.Sprintf(" %s=%v", attr.Key, attr.Value)
			}
		}
	}

	fmt.Printf("%s[GoHYE] [%s] [%s%s%s] [%s] %s%s%s\n",
		colorWhite,
		timestamp,
		levelColor,
		levelText,
		colorWhite,
		logType,
		message,
		attrsStr,
		colorReset,
	)

	return nil
}

func shouldSkipLog(r *slog.Record) bool {
	// Skip only specific gateway and bucket messages
	skippedMessages := []string{
		"locking buckets",
		"unlocking buckets",
		"gateway event",
		"cleaning up bucket",
		"cleaned up rate limit buckets",
		"binary message received",
		"received gateway message",
		"opening gateway connection",
		"locking gateway rate limiter",
		"unlocking gateway rate limiter",
		"sending gateway command",
		"new request",
		"new response",
		"locking rest bucket",
		"unlocking rest bucket",
		"sending identify command name-gateway",
		"ready message received name",
		"rate limit response headers",
		"sending heartbeat",
	}

	for _, skip := range skippedMessages {
		if strings.Contains(strings.ToLower(r.Message), strings.ToLower(skip)) {
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
