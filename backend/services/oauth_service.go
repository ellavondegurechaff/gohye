package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/disgoorg/bot-template/backend/config"
	"github.com/disgoorg/bot-template/backend/models"
)

// DiscordUser represents a Discord user from the API
type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Email         string `json:"email"`
}

// DiscordGuildMember represents a Discord guild member from the API
type DiscordGuildMember struct {
	User  DiscordUser `json:"user"`
	Roles []string    `json:"roles"`
	Nick  string      `json:"nick"`
}

// OAuthService handles Discord OAuth2 authentication
type OAuthService struct {
	config     *config.WebAppConfig
	httpClient *http.Client
}

// NewOAuthService creates a new OAuth service
func NewOAuthService(cfg *config.WebAppConfig) *OAuthService {
	return &OAuthService{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateAuthURL generates the Discord OAuth2 authorization URL
func (o *OAuthService) GenerateAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", o.config.Config.Web.OAuth.ClientID)
	params.Set("redirect_uri", o.config.Config.Web.OAuth.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(o.config.Config.Web.OAuth.Scopes, " "))
	params.Set("state", state)

	return "https://discord.com/api/oauth2/authorize?" + params.Encode()
}

// ExchangeCodeForToken exchanges an authorization code for an access token
func (o *OAuthService) ExchangeCodeForToken(ctx context.Context, code string) (string, error) {
	data := url.Values{}
	data.Set("client_id", o.config.Config.Web.OAuth.ClientID)
	data.Set("client_secret", o.config.Config.Web.OAuth.ClientSecret)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", o.config.Config.Web.OAuth.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://discord.com/api/oauth2/token", 
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code for token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("discord API error: %s", string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// GetUserInfo gets Discord user information using an access token
func (o *OAuthService) GetUserInfo(ctx context.Context, accessToken string) (*DiscordUser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://discord.com/api/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord API error: %s", string(body))
	}

	var user DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &user, nil
}

// GetUserGuildMember gets user's guild member information (for role checking)
func (o *OAuthService) GetUserGuildMember(ctx context.Context, accessToken string) (*DiscordGuildMember, error) {
	if o.config.Config.Web.AdminGuildID == "" {
		return nil, fmt.Errorf("admin guild ID not configured")
	}

	url := fmt.Sprintf("https://discord.com/api/users/@me/guilds/%s/member", o.config.Config.Web.AdminGuildID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create guild member request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get guild member info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user is not a member of the admin guild")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord API error: %s", string(body))
	}

	var member DiscordGuildMember
	if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
		return nil, fmt.Errorf("failed to decode guild member info: %w", err)
	}

	return &member, nil
}

// CreateUserSession creates a user session from Discord user info
func (o *OAuthService) CreateUserSession(ctx context.Context, user *DiscordUser, accessToken string) (*models.UserSession, error) {
	session := &models.UserSession{
		DiscordID:   user.ID,
		Username:    user.Username,
		Avatar:      user.Avatar,
		Email:       user.Email,
		Permissions: []string{},
		Roles:       []string{},
		IsAdmin:     false,
		ExpiresAt:   time.Now().Add(24 * time.Hour), // 24 hour session
	}

	// Check if user is an admin by user ID
	session.IsAdmin = o.isAdminUser(user.ID)
	slog.Info("Admin check by user ID",
		slog.String("user_id", user.ID),
		slog.Bool("is_admin_by_id", session.IsAdmin))

	// If not admin by user ID, check roles if guild is configured
	if !session.IsAdmin && o.config.Config.Web.AdminGuildID != "" {
		member, err := o.GetUserGuildMember(ctx, accessToken)
		if err != nil {
			slog.Warn("Failed to get guild member info for admin check",
				slog.String("user_id", user.ID),
				slog.String("error", err.Error()))
		} else {
			session.Roles = member.Roles
			session.IsAdmin = o.isAdminRole(member.Roles)
		}
	}

	// Set permissions based on admin status
	if session.IsAdmin {
		session.Permissions = []string{
			"admin.read",
			"admin.write",
			"cards.create",
			"cards.update",
			"cards.delete",
			"collections.import",
			"sync.manage",
			"users.view",
		}
	}

	slog.Info("User session created",
		slog.String("user_id", session.DiscordID),
		slog.String("username", session.Username),
		slog.Bool("is_admin", session.IsAdmin),
		slog.Int("permissions", len(session.Permissions)))

	return session, nil
}

// isAdminUser checks if a user ID is in the admin users list
func (o *OAuthService) isAdminUser(userID string) bool {
	slog.Info("Checking admin user",
		slog.String("user_id", userID),
		slog.Any("admin_users", o.config.Config.Web.AdminUsers))
	
	for _, adminID := range o.config.Config.Web.AdminUsers {
		slog.Info("Comparing admin IDs", 
			slog.String("user_id", userID),
			slog.String("admin_id", adminID))
		if adminID == userID {
			slog.Info("Admin user match found", slog.String("user_id", userID))
			return true
		}
	}
	slog.Info("No admin user match found", slog.String("user_id", userID))
	return false
}

// isAdminRole checks if any of the user's roles are admin roles
func (o *OAuthService) isAdminRole(userRoles []string) bool {
	for _, roleID := range userRoles {
		for _, adminRoleID := range o.config.Config.Web.AdminRoles {
			if roleID == adminRoleID {
				return true
			}
		}
	}
	return false
}

// GenerateState generates a random state parameter for OAuth2
func (o *OAuthService) GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ValidateState validates the OAuth2 state parameter
func (o *OAuthService) ValidateState(c *fiber.Ctx, expectedState string) bool {
	receivedState := c.Query("state")
	return receivedState == expectedState
}