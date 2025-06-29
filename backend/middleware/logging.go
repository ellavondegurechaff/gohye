package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/disgoorg/bot-template/backend/utils"
)

// LoggingMiddleware logs HTTP requests in a structured format
func LoggingMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Extract user information if available
		var userID string
		var username string
		if session, ok := utils.ExtractUserSession(c); ok {
			userID = session.DiscordID
			username = session.Username
		}

		// Log level based on status code
		statusCode := c.Response().StatusCode()
		logLevel := slog.LevelInfo
		if statusCode >= 400 && statusCode < 500 {
			logLevel = slog.LevelWarn
		} else if statusCode >= 500 {
			logLevel = slog.LevelError
		}

		// Create log entry
		logger := slog.With(
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.String("query", c.Request().URI().QueryArgs().String()),
			slog.Int("status", statusCode),
			slog.Duration("duration", duration),
			slog.String("ip", utils.GetIPAddress(c)),
			slog.String("user_agent", utils.GetUserAgent(c)),
			slog.Int("size", len(c.Response().Body())),
		)

		// Add user information if available
		if userID != "" {
			logger = logger.With(
				slog.String("user_id", userID),
				slog.String("username", username),
			)
		}

		// Add error information if present
		if err != nil {
			logger = logger.With(slog.String("error", err.Error()))
		}

		// Add referer if present
		if referer := c.Get("Referer"); referer != "" {
			logger = logger.With(slog.String("referer", referer))
		}

		// Add HTMX information if present
		if c.Get("HX-Request") != "" {
			logger = logger.With(
				slog.Bool("htmx_request", true),
				slog.String("htmx_target", c.Get("HX-Target")),
				slog.String("htmx_trigger", c.Get("HX-Trigger")),
			)
		}

		// Log the request
		message := "HTTP request processed"
		if err != nil {
			message = "HTTP request failed"
			// Add more detailed error information
			slog.Error("HTTP request error details",
				slog.String("method", c.Method()),
				slog.String("path", c.Path()),
				slog.Int("status", statusCode),
				slog.String("error", err.Error()),
				slog.String("ip", utils.GetIPAddress(c)),
			)
		}

		logger.Log(c.Context(), logLevel, message)

		return err
	}
}

// AccessLogMiddleware logs access attempts for sensitive operations
func AccessLogMiddleware(operation string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract user information
		var userID string
		var username string
		if session, ok := utils.ExtractUserSession(c); ok {
			userID = session.DiscordID
			username = session.Username
		}

		// Log access attempt
		slog.Info("Admin operation attempted",
			slog.String("operation", operation),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.String("ip", utils.GetIPAddress(c)),
			slog.String("user_id", userID),
			slog.String("username", username),
			slog.String("user_agent", utils.GetUserAgent(c)),
		)

		return c.Next()
	}
}

// AuditLogMiddleware logs important administrative actions
func AuditLogMiddleware(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Extract user information
		var userID string
		var username string
		if session, ok := utils.ExtractUserSession(c); ok {
			userID = session.DiscordID
			username = session.Username
		}

		// Log the action
		statusCode := c.Response().StatusCode()
		success := err == nil && statusCode >= 200 && statusCode < 300

		slog.Info("Admin action completed",
			slog.String("action", action),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Bool("success", success),
			slog.Int("status", statusCode),
			slog.Duration("duration", time.Since(start)),
			slog.String("ip", utils.GetIPAddress(c)),
			slog.String("user_id", userID),
			slog.String("username", username),
		)

		return err
	}
}