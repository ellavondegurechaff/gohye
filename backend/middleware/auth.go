package middleware

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/disgoorg/bot-template/backend/handlers"
	"github.com/disgoorg/bot-template/backend/models"
	"github.com/disgoorg/bot-template/backend/utils"
)

// AuthRequired middleware ensures the user is authenticated
func AuthRequired(webApp *handlers.WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check for session
		session, err := webApp.GetSession(c)
		if err != nil {
			slog.Debug("Auth required: no valid session", slog.String("error", err.Error()))
			return redirectToLogin(c)
		}

		// Validate session
		if session == nil || session.DiscordID == "" {
			slog.Debug("Auth required: invalid session")
			return redirectToLogin(c)
		}

		// Store user in context
		c.Locals("user", session)
		
		slog.Debug("Auth middleware: user authenticated",
			slog.String("discord_id", session.DiscordID),
			slog.String("username", session.Username))

		return c.Next()
	}
}

// AdminRequired middleware ensures the user has admin privileges
func AdminRequired(webApp *handlers.WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user from context (should be set by AuthRequired middleware)
		user := c.Locals("user")
		if user == nil {
			slog.Warn("Admin required: no user in context")
			return utils.SendForbidden(c, "Access denied")
		}

		session, ok := user.(*models.UserSession)
		if !ok {
			slog.Warn("Admin required: invalid user session type")
			return utils.SendForbidden(c, "Access denied")
		}

		// Check if user has admin privileges
		if !session.IsAdmin {
			slog.Warn("Admin required: user lacks admin privileges",
				slog.String("discord_id", session.DiscordID),
				slog.String("username", session.Username))
			return utils.SendForbidden(c, "Admin access required")
		}

		slog.Debug("Admin middleware: user has admin access",
			slog.String("discord_id", session.DiscordID),
			slog.String("username", session.Username))

		return c.Next()
	}
}

// OptionalAuth middleware adds user info to context if authenticated, but doesn't require it
func OptionalAuth(webApp *handlers.WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Try to get session, but don't fail if not present
		session, err := webApp.GetSession(c)
		if err == nil && session != nil && session.DiscordID != "" {
			c.Locals("user", session)
			slog.Debug("Optional auth: user authenticated",
				slog.String("discord_id", session.DiscordID),
				slog.String("username", session.Username))
		}

		return c.Next()
	}
}

// RoleRequired middleware ensures the user has a specific role
func RoleRequired(role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := c.Locals("user")
		if user == nil {
			return utils.SendForbidden(c, "Authentication required")
		}

		session, ok := user.(*models.UserSession)
		if !ok {
			return utils.SendForbidden(c, "Invalid session")
		}

		// Check if user has the required role
		hasRole := false
		for _, userRole := range session.Roles {
			if strings.EqualFold(userRole, role) {
				hasRole = true
				break
			}
		}

		if !hasRole {
			slog.Warn("Role required: user lacks required role",
				slog.String("discord_id", session.DiscordID),
				slog.String("username", session.Username),
				slog.String("required_role", role),
				slog.Any("user_roles", session.Roles))
			return utils.SendForbidden(c, "Insufficient permissions")
		}

		return c.Next()
	}
}

// PermissionRequired middleware ensures the user has a specific permission
func PermissionRequired(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user := c.Locals("user")
		if user == nil {
			return utils.SendForbidden(c, "Authentication required")
		}

		session, ok := user.(*models.UserSession)
		if !ok {
			return utils.SendForbidden(c, "Invalid session")
		}

		// Check if user has the required permission
		hasPermission := false
		for _, userPerm := range session.Permissions {
			if strings.EqualFold(userPerm, permission) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			slog.Warn("Permission required: user lacks required permission",
				slog.String("discord_id", session.DiscordID),
				slog.String("username", session.Username),
				slog.String("required_permission", permission),
				slog.Any("user_permissions", session.Permissions))
			return utils.SendForbidden(c, "Insufficient permissions")
		}

		return c.Next()
	}
}

// redirectToLogin redirects to login page for web requests or returns 401 for API requests
func redirectToLogin(c *fiber.Ctx) error {
	// Check if this is an API request
	if isAPIRequest(c) {
		return utils.SendUnauthorized(c, "Authentication required")
	}

	// For web requests, redirect to login
	return c.Redirect("/login")
}

// isAPIRequest checks if the request is an API request
func isAPIRequest(c *fiber.Ctx) bool {
	path := c.Path()
	
	// Check if path starts with /api or /admin/api
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/admin/api/") {
		return true
	}

	// Check Accept header
	accept := c.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}

	// Check if it's an HTMX request
	if c.Get("HX-Request") != "" {
		return true
	}

	return false
}