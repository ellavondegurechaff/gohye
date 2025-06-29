package utils

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/disgoorg/bot-template/backend/models"
)

// SendJSON sends a JSON response using Fiber
func SendJSON(c *fiber.Ctx, statusCode int, data interface{}) error {
	return c.Status(statusCode).JSON(data)
}

// SendSuccess sends a successful JSON response
func SendSuccess(c *fiber.Ctx, data interface{}, message string) error {
	response := models.NewSuccessResponse(data, message)
	return SendJSON(c, http.StatusOK, response)
}

// SendCreated sends a created resource JSON response
func SendCreated(c *fiber.Ctx, data interface{}, message string) error {
	response := models.NewSuccessResponse(data, message)
	return SendJSON(c, http.StatusCreated, response)
}

// SendError sends an error JSON response
func SendError(c *fiber.Ctx, statusCode int, code, message string, details map[string]string) error {
	response := models.NewErrorResponse(code, message, details)
	return SendJSON(c, statusCode, response)
}

// SendBadRequest sends a bad request error response
func SendBadRequest(c *fiber.Ctx, message string, details map[string]string) error {
	return SendError(c, http.StatusBadRequest, "BAD_REQUEST", message, details)
}

// SendUnauthorized sends an unauthorized error response
func SendUnauthorized(c *fiber.Ctx, message string) error {
	return SendError(c, http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

// SendForbidden sends a forbidden error response
func SendForbidden(c *fiber.Ctx, message string) error {
	return SendError(c, http.StatusForbidden, "FORBIDDEN", message, nil)
}

// SendNotFound sends a not found error response
func SendNotFound(c *fiber.Ctx, message string) error {
	return SendError(c, http.StatusNotFound, "NOT_FOUND", message, nil)
}

// SendConflict sends a conflict error response
func SendConflict(c *fiber.Ctx, message string, details map[string]string) error {
	return SendError(c, http.StatusConflict, "CONFLICT", message, details)
}

// SendInternalServerError sends an internal server error response
func SendInternalServerError(c *fiber.Ctx, message string) error {
	return SendError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", message, nil)
}

// SendUnprocessableEntity sends an unprocessable entity error response
func SendUnprocessableEntity(c *fiber.Ctx, message string, details map[string]string) error {
	return SendError(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", message, details)
}

// SendPaginated sends a paginated JSON response
func SendPaginated(c *fiber.Ctx, data interface{}, pagination *models.PaginationInfo, message string) error {
	response := models.NewPaginatedResponse(data, pagination, message)
	return SendJSON(c, http.StatusOK, response)
}

// SendNoContent sends a no content response
func SendNoContent(c *fiber.Ctx) error {
	return c.SendStatus(http.StatusNoContent)
}

// HandleValidationErrors converts validation errors to API response
func HandleValidationErrors(c *fiber.Ctx, errors []models.ValidationError) error {
	details := make(map[string]string)
	for _, err := range errors {
		details[err.FileName] = err.Description
	}
	return SendUnprocessableEntity(c, "Validation failed", details)
}

// ExtractUserSession extracts user session from Fiber context
func ExtractUserSession(c *fiber.Ctx) (*models.UserSession, bool) {
	session := c.Locals("user")
	if session == nil {
		return nil, false
	}
	
	userSession, ok := session.(*models.UserSession)
	return userSession, ok
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c *fiber.Ctx) bool {
	session, ok := ExtractUserSession(c)
	if !ok {
		return false
	}
	return session.IsAdmin
}

// GetIPAddress extracts the client IP address
func GetIPAddress(c *fiber.Ctx) string {
	// Check X-Forwarded-For header first
	if xff := c.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Check X-Real-IP header
	if xri := c.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fallback to connection remote address
	return c.IP()
}

// GetUserAgent extracts the user agent
func GetUserAgent(c *fiber.Ctx) string {
	return c.Get("User-Agent")
}