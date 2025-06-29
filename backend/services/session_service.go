package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/disgoorg/bot-template/backend/config"
	"github.com/disgoorg/bot-template/backend/models"
)

const (
	SessionCookieName = "gohye_session"
	StateCookieName   = "oauth_state"
)

// SessionService handles user session management
type SessionService struct {
	config *config.WebAppConfig
}

// NewSessionService creates a new session service
func NewSessionService(cfg *config.WebAppConfig) *SessionService {
	return &SessionService{
		config: cfg,
	}
}

// CreateSession creates a new user session and sets the session cookie
func (s *SessionService) CreateSession(c *fiber.Ctx, userSession *models.UserSession) error {
	// Serialize session data
	sessionData, err := json.Marshal(userSession)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Sign the session data
	signedSession, err := s.signData(sessionData)
	if err != nil {
		return fmt.Errorf("failed to sign session: %w", err)
	}

	// Set session cookie
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    signedSession,
		Path:     "/",
		MaxAge:   int(24 * time.Hour / time.Second), // 24 hours
		Secure:   s.config.Environment == "production",
		HTTPOnly: true,
		SameSite: "Lax",
	})

	slog.Info("Session created for user",
		slog.String("user_id", userSession.DiscordID),
		slog.String("username", userSession.Username),
		slog.Bool("is_admin", userSession.IsAdmin))

	return nil
}

// GetSession retrieves and validates the user session from the request
func (s *SessionService) GetSession(c *fiber.Ctx) (*models.UserSession, error) {
	// Get session cookie
	sessionCookie := c.Cookies(SessionCookieName)
	if sessionCookie == "" {
		return nil, fmt.Errorf("no session cookie found")
	}

	// Verify and decode session data
	sessionData, err := s.verifyAndDecodeData(sessionCookie)
	if err != nil {
		return nil, fmt.Errorf("invalid session signature: %w", err)
	}

	// Unmarshal session
	var userSession models.UserSession
	if err := json.Unmarshal(sessionData, &userSession); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Check if session is expired
	if time.Now().After(userSession.ExpiresAt) {
		s.DestroySession(c)
		return nil, fmt.Errorf("session expired")
	}

	return &userSession, nil
}

// DestroySession removes the session cookie and invalidates the session
func (s *SessionService) DestroySession(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   s.config.Environment == "production",
		HTTPOnly: true,
		SameSite: "Lax",
	})

	slog.Info("Session destroyed for request",
		slog.String("ip", c.IP()),
		slog.String("user_agent", c.Get("User-Agent")))
}

// SetState sets the OAuth state parameter in a secure cookie
func (s *SessionService) SetState(c *fiber.Ctx, state string) error {
	// Sign the state
	signedState, err := s.signData([]byte(state))
	if err != nil {
		return fmt.Errorf("failed to sign state: %w", err)
	}

	// Set state cookie
	c.Cookie(&fiber.Cookie{
		Name:     StateCookieName,
		Value:    signedState,
		Path:     "/",
		MaxAge:   int(10 * time.Minute / time.Second), // 10 minutes
		Secure:   s.config.Environment == "production",
		HTTPOnly: true,
		SameSite: "Lax",
	})

	return nil
}

// GetAndClearState retrieves and clears the OAuth state parameter
func (s *SessionService) GetAndClearState(c *fiber.Ctx) (string, error) {
	// Get state cookie
	stateCookie := c.Cookies(StateCookieName)
	if stateCookie == "" {
		return "", fmt.Errorf("no state cookie found")
	}

	// Clear state cookie immediately
	c.Cookie(&fiber.Cookie{
		Name:     StateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   s.config.Environment == "production",
		HTTPOnly: true,
		SameSite: "Lax",
	})

	// Verify and decode state
	stateData, err := s.verifyAndDecodeData(stateCookie)
	if err != nil {
		return "", fmt.Errorf("invalid state signature: %w", err)
	}

	return string(stateData), nil
}

// RefreshSession extends the session expiration time
func (s *SessionService) RefreshSession(c *fiber.Ctx, userSession *models.UserSession) error {
	// Update expiration time
	userSession.ExpiresAt = time.Now().Add(24 * time.Hour)

	// Create new session cookie
	return s.CreateSession(c, userSession)
}

// signData signs data using HMAC-SHA256
func (s *SessionService) signData(data []byte) (string, error) {
	if s.config.Config.Web.SessionKey == "" {
		return "", fmt.Errorf("session key not configured")
	}

	// Create HMAC signature
	h := hmac.New(sha256.New, []byte(s.config.Config.Web.SessionKey))
	h.Write(data)
	signature := h.Sum(nil)

	// Combine data and signature
	combined := append(data, signature...)

	// Base64 encode
	return base64.URLEncoding.EncodeToString(combined), nil
}

// verifyAndDecodeData verifies the signature and returns the original data
func (s *SessionService) verifyAndDecodeData(encodedData string) ([]byte, error) {
	if s.config.Config.Web.SessionKey == "" {
		return nil, fmt.Errorf("session key not configured")
	}

	// Base64 decode
	combined, err := base64.URLEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode data: %w", err)
	}

	// Extract data and signature (signature is last 32 bytes)
	if len(combined) < 32 {
		return nil, fmt.Errorf("invalid data length")
	}

	data := combined[:len(combined)-32]
	receivedSignature := combined[len(combined)-32:]

	// Verify signature
	h := hmac.New(sha256.New, []byte(s.config.Config.Web.SessionKey))
	h.Write(data)
	expectedSignature := h.Sum(nil)

	if !hmac.Equal(receivedSignature, expectedSignature) {
		return nil, fmt.Errorf("signature verification failed")
	}

	return data, nil
}

// IsAdmin checks if the current session belongs to an admin user
func (s *SessionService) IsAdmin(userSession *models.UserSession) bool {
	return userSession != nil && userSession.IsAdmin
}

// HasPermission checks if the user has a specific permission
func (s *SessionService) HasPermission(userSession *models.UserSession, permission string) bool {
	if userSession == nil {
		return false
	}

	for _, perm := range userSession.Permissions {
		if perm == permission {
			return true
		}
	}

	return false
}