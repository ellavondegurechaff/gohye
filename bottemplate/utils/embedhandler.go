// File: utils/embedhandler.go

package utils

import (
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// ResponseHandler provides standardized response methods for commands and components
type ResponseHandler struct{}

var EH = &ResponseHandler{}

// ErrorType represents different categories of errors for consistent handling
type ErrorType int

const (
	// UserError - User input issues, validation failures, parameter problems
	UserError ErrorType = iota
	// SystemError - Database failures, network issues, internal server errors
	SystemError
	// NotFoundError - Requested resources don't exist
	NotFoundError
	// PermissionError - Unauthorized actions, access denied
	PermissionError
	// BusinessLogicError - Cooldowns, insufficient resources, game rule violations
	BusinessLogicError
)

// ErrorConfig holds configuration for error formatting
type ErrorConfig struct {
	Type        ErrorType
	Title       string
	Description string
	UseEmbed    bool
	Ephemeral   bool
}

// getErrorPrefix returns the appropriate emoji prefix for error types
func getErrorPrefix(errorType ErrorType) string {
	switch errorType {
	case UserError:
		return "⚠️"
	case SystemError:
		return "🔧"
	case NotFoundError:
		return "🔍"
	case PermissionError:
		return "🚫"
	case BusinessLogicError:
		return "⏰"
	default:
		return "❌"
	}
}

// getErrorColor returns the appropriate color for error types
func getErrorColor(errorType ErrorType) int {
	switch errorType {
	case UserError:
		return config.WarningColor
	case SystemError:
		return config.ErrorColor
	case NotFoundError:
		return config.InfoColor
	case PermissionError:
		return config.ErrorColor
	case BusinessLogicError:
		return config.WarningColor
	default:
		return config.ErrorColor
	}
}

// CreateErrorEmbed creates a standard error embed for command events
func (h *ResponseHandler) CreateErrorEmbed(event *handler.CommandEvent, message string) error {
	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Description: message,
			Color:       config.ErrorColor,
		}},
	})
}

// CreateSuccessEmbed creates a standard success embed for command events
func (h *ResponseHandler) CreateSuccessEmbed(event *handler.CommandEvent, message string) error {
	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Description: message,
			Color:       config.SuccessColor,
		}},
	})
}

// CreateInfoEmbed creates a standard info embed for command events
func (h *ResponseHandler) CreateInfoEmbed(event *handler.CommandEvent, message string) error {
	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Description: message,
			Color:       config.InfoColor,
		}},
	})
}

// CreateEphemeralError creates an ephemeral error message for component events
func (h *ResponseHandler) CreateEphemeralError(event *handler.ComponentEvent, message string) error {
	return event.CreateMessage(discord.MessageCreate{
		Content: message,
		Flags:   discord.MessageFlagEphemeral,
	})
}

// CreateEphemeralSuccess creates an ephemeral success message for component events
func (h *ResponseHandler) CreateEphemeralSuccess(event *handler.ComponentEvent, message string) error {
	return event.CreateMessage(discord.MessageCreate{
		Content: "✅ " + message,
		Flags:   discord.MessageFlagEphemeral,
	})
}

// CreateEphemeralInfo creates an ephemeral info message for component events
func (h *ResponseHandler) CreateEphemeralInfo(event *handler.ComponentEvent, message string) error {
	return event.CreateMessage(discord.MessageCreate{
		Content: "ℹ️ " + message,
		Flags:   discord.MessageFlagEphemeral,
	})
}

// UpdateInteractionResponse updates the interaction response with an error
func (h *ResponseHandler) UpdateInteractionResponse(event *handler.CommandEvent, title, description string) error {
	_, err := event.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds: &[]discord.Embed{
			{
				Title:       "❌ " + title,
				Description: fmt.Sprintf("```diff\n- %s\n```", description),
				Color:       config.ErrorColor,
			},
		},
	})
	return err
}

// CreateError creates a detailed error embed with title and description
func (h *ResponseHandler) CreateError(event *handler.CommandEvent, title, description string) error {
	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title:       "❌ " + title,
			Description: fmt.Sprintf("```diff\n- %s\n```", description),
			Color:       config.ErrorColor,
		}},
	})
}

// HandleError provides centralized error handling for different event types
func (h *ResponseHandler) HandleError(event interface{}, message string) error {
	switch e := event.(type) {
	case *handler.CommandEvent:
		return h.CreateErrorEmbed(e, message)
	case *handler.ComponentEvent:
		return h.CreateEphemeralError(e, message)
	default:
		return fmt.Errorf("unsupported event type for error handling")
	}
}

// HandleSuccess provides centralized success handling for different event types
func (h *ResponseHandler) HandleSuccess(event interface{}, message string) error {
	switch e := event.(type) {
	case *handler.CommandEvent:
		return h.CreateSuccessEmbed(e, message)
	case *handler.ComponentEvent:
		return h.CreateEphemeralSuccess(e, message)
	default:
		return fmt.Errorf("unsupported event type for success handling")
	}
}

// === NEW STANDARDIZED ERROR METHODS ===

// CreateClassifiedError creates an error response with automatic categorization
func (h *ResponseHandler) CreateClassifiedError(event *handler.CommandEvent, errorType ErrorType, message string) error {
	prefix := getErrorPrefix(errorType)
	color := getErrorColor(errorType)

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Description: prefix + " " + message,
			Color:       color,
		}},
	})
}

// CreateUserError creates an error response for user input issues
func (h *ResponseHandler) CreateUserError(event *handler.CommandEvent, message string) error {
	return h.CreateClassifiedError(event, UserError, message)
}

// CreateSystemError creates an error response for system/technical failures
func (h *ResponseHandler) CreateSystemError(event *handler.CommandEvent, message string) error {
	return h.CreateClassifiedError(event, SystemError, message)
}

// CreateNotFoundError creates an error response for resources that don't exist
func (h *ResponseHandler) CreateNotFoundError(event *handler.CommandEvent, resource, identifier string) error {
	message := fmt.Sprintf("%s '%s' not found", resource, identifier)
	return h.CreateClassifiedError(event, NotFoundError, message)
}

// CreatePermissionError creates an error response for unauthorized actions
func (h *ResponseHandler) CreatePermissionError(event *handler.CommandEvent, action string) error {
	message := fmt.Sprintf("You don't have permission to %s", action)
	return h.CreateClassifiedError(event, PermissionError, message)
}

// CreateBusinessLogicError creates an error response for game rule violations
func (h *ResponseHandler) CreateBusinessLogicError(event *handler.CommandEvent, message string) error {
	return h.CreateClassifiedError(event, BusinessLogicError, message)
}

// CreateClassifiedComponentError creates an ephemeral error for component interactions
func (h *ResponseHandler) CreateClassifiedComponentError(event *handler.ComponentEvent, errorType ErrorType, message string) error {
	prefix := getErrorPrefix(errorType)
	return event.CreateMessage(discord.MessageCreate{
		Content: prefix + " " + message,
		Flags:   discord.MessageFlagEphemeral,
	})
}

// AutoClassifyError attempts to automatically classify errors based on message content
func (h *ResponseHandler) AutoClassifyError(event interface{}, message string) error {
	errorType := h.classifyErrorByMessage(message)

	switch e := event.(type) {
	case *handler.CommandEvent:
		return h.CreateClassifiedError(e, errorType, message)
	case *handler.ComponentEvent:
		return h.CreateClassifiedComponentError(e, errorType, message)
	default:
		return fmt.Errorf("unsupported event type for error handling")
	}
}

// classifyErrorByMessage attempts to classify error type based on message content
func (h *ResponseHandler) classifyErrorByMessage(message string) ErrorType {
	lowerMsg := strings.ToLower(message)

	// Check for not found patterns
	if strings.Contains(lowerMsg, "not found") ||
		strings.Contains(lowerMsg, "no cards found") ||
		strings.Contains(lowerMsg, "no results") ||
		strings.Contains(lowerMsg, "doesn't exist") {
		return NotFoundError
	}

	// Check for user input patterns
	if strings.Contains(lowerMsg, "invalid") ||
		strings.Contains(lowerMsg, "must be") ||
		strings.Contains(lowerMsg, "required") ||
		strings.Contains(lowerMsg, "please provide") {
		return UserError
	}

	// Check for business logic patterns
	if strings.Contains(lowerMsg, "cooldown") ||
		strings.Contains(lowerMsg, "wait") ||
		strings.Contains(lowerMsg, "insufficient") ||
		strings.Contains(lowerMsg, "limit") ||
		strings.Contains(lowerMsg, "already") {
		return BusinessLogicError
	}

	// Check for permission patterns
	if strings.Contains(lowerMsg, "permission") ||
		strings.Contains(lowerMsg, "unauthorized") ||
		strings.Contains(lowerMsg, "access denied") {
		return PermissionError
	}

	// Check for system error patterns
	if strings.Contains(lowerMsg, "failed to") ||
		strings.Contains(lowerMsg, "database") ||
		strings.Contains(lowerMsg, "connection") ||
		strings.Contains(lowerMsg, "timeout") ||
		strings.Contains(lowerMsg, "internal error") {
		return SystemError
	}

	// Default to system error for unknown patterns
	return SystemError
}

// === MIGRATION HELPERS ===

// CreateSmartError automatically classifies and creates appropriate error response
func (h *ResponseHandler) CreateSmartError(event interface{}, message string) error {
	return h.AutoClassifyError(event, message)
}

// CreateCompatibleError maintains compatibility with existing ❌ prefix style
func (h *ResponseHandler) CreateCompatibleError(event *handler.CommandEvent, message string) error {
	// Preserve existing ❌ prefix behavior for backward compatibility
	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Description: "❌ " + message,
			Color:       config.ErrorColor,
		}},
	})
}
