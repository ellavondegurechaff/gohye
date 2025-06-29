package middleware

import (
	"fmt"
	
	"github.com/gofiber/fiber/v2"
)

// CustomErrorHandler handles application errors
func CustomErrorHandler(c *fiber.Ctx, err error) error {
	// Default to 500 server error
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	// Try to extract Fiber error
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	// Send custom error page or JSON based on request type
	if c.Get("Accept") == "application/json" || c.Get("HX-Request") != "" {
		return c.Status(code).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    code,
				"message": message,
			},
		})
	}

	// For HTML requests, try to render error page, fallback to plain text
	renderErr := c.Status(code).Render("pages/error", fiber.Map{
		"Title":   "Error",
		"Code":    code,
		"Message": message,
	})
	
	// If template rendering fails, fallback to plain text response
	if renderErr != nil {
		return c.Status(code).SendString(fmt.Sprintf("Error %d: %s", code, message))
	}
	
	return renderErr
}

// SecurityHeaders adds security headers to responses
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Security headers
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		// CSP header for admin panel
		if c.Path() != "/login" && c.Path() != "/" {
			c.Set("Content-Security-Policy", 
				"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://unpkg.com https://cdn.tailwindcss.com; "+
				"style-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://fonts.googleapis.com; "+
				"img-src 'self' data: https://cards.hyejoobot.com https://cdn.discordapp.com; "+
				"font-src 'self' https://fonts.gstatic.com https://fonts.googleapis.com; "+
				"connect-src 'self';")
		}

		return c.Next()
	}
}