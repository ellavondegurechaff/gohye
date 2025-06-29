package handlers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/backend/config"
	webmodels "github.com/disgoorg/bot-template/backend/models"
	webservices "github.com/disgoorg/bot-template/backend/services"
	"github.com/disgoorg/bot-template/backend/utils"
)

// WebApp represents the web application with all dependencies
type WebApp struct {
	Config                    *config.WebAppConfig
	DB                        *database.DB
	Repos                     *webmodels.Repositories
	SpacesService             interface{} // Placeholder for now
	CardMgmtService           *webservices.CardManagementService
	CardImportService         *webservices.CardImportService
	SyncMgrService            *webservices.SyncManagerService
	OAuthService              *webservices.OAuthService
	SessionService            *webservices.SessionService
	CollectionImportService   *webservices.CollectionImportService
	Version                   string
	Commit                    string
}

// parseInt64 is a utility function to parse int64 from string
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// getDashboardStats retrieves dashboard statistics
func getDashboardStats(ctx context.Context, webApp *WebApp) (*webmodels.DashboardStats, error) {
	// Test database connection first
	if webApp.DB == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	
	if webApp.Repos == nil {
		return nil, fmt.Errorf("repositories are nil")
	}
	
	if webApp.Repos.Card == nil {
		return nil, fmt.Errorf("card repository is nil")
	}
	
	if webApp.Repos.Collection == nil {
		return nil, fmt.Errorf("collection repository is nil")
	}
	
	// Get total cards count using optimized COUNT query
	totalCards, err := webApp.Repos.Card.GetCardCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get card count: %w", err)
	}
	
	// Get total collections count using optimized COUNT query
	totalCollections, err := webApp.Repos.Collection.GetCollectionCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection count: %w", err)
	}
	
	// Get total users count
	totalUsers, err := webApp.Repos.User.GetUserCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user count: %w", err)
	}
	
	// TODO: Calculate sync percentage and issues
	// For now, use placeholder values
	syncPercentage := 98.5 // Placeholder
	issueCount := 0        // Placeholder
	
	// TODO: Get recent activity
	// For now, use placeholder activities
	recentActivity := []webmodels.ActivityItem{
		{
			Type:        "card_created",
			Description: "New card added to collection",
			Timestamp:   time.Now().Add(-time.Hour),
		},
		{
			Type:        "collection_updated",
			Description: "Collection metadata updated",
			Timestamp:   time.Now().Add(-2 * time.Hour),
		},
	}
	
	return &webmodels.DashboardStats{
		TotalCards:      totalCards,
		TotalCollections: totalCollections,
		TotalUsers:      totalUsers,
		SyncPercentage:  syncPercentage,
		IssueCount:      issueCount,
		RecentActivity:  recentActivity,
	}, nil
}

// processUploadedFile processes a single uploaded file
func processUploadedFile(ctx context.Context, webApp *WebApp, file *multipart.FileHeader) fiber.Map {
	// Validate file size (max 10MB)
	const maxFileSize = 10 * 1024 * 1024
	if file.Size > maxFileSize {
		return fiber.Map{
			"filename": file.Filename,
			"success":  false,
			"error":    "File too large (max 10MB)",
		}
	}
	
	// Validate file type
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	
	contentType := file.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		return fiber.Map{
			"filename": file.Filename,
			"success":  false,
			"error":    "Invalid file type (only images allowed)",
		}
	}
	
	// Open file
	src, err := file.Open()
	if err != nil {
		return fiber.Map{
			"filename": file.Filename,
			"success":  false,
			"error":    "Failed to open file",
		}
	}
	defer src.Close()
	
	// Read file data
	fileData := make([]byte, file.Size)
	_, err = src.Read(fileData)
	if err != nil {
		return fiber.Map{
			"filename": file.Filename,
			"success":  false,
			"error":    "Failed to read file",
		}
	}
	
	// TODO: Upload to spaces service
	// For now, just return success with placeholder URL
	return fiber.Map{
		"filename": file.Filename,
		"success":  true,
		"size":     file.Size,
		"type":     contentType,
		"url":      fmt.Sprintf("/uploads/%s", file.Filename), // Placeholder
	}
}

// GetSession gets the current user session
func (w *WebApp) GetSession(c *fiber.Ctx) (*webmodels.UserSession, error) {
	session, err := w.SessionService.GetSession(c)
	if err != nil {
		return nil, err
	}
	
	// Convert from service UserSession to webmodels UserSession
	webSession := &webmodels.UserSession{
		DiscordID:   session.DiscordID,
		Username:    session.Username,
		Avatar:      session.Avatar,
		Email:       session.Email,
		Roles:       session.Roles,
		Permissions: session.Permissions,
		ExpiresAt:   session.ExpiresAt,
		IsAdmin:     session.IsAdmin,
	}
	
	return webSession, nil
}

// =============================================================================
// CORE API HANDLERS (Keep - Required for Next.js)
// =============================================================================

func HealthCheck(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return utils.SendSuccess(c, fiber.Map{
			"status":  "healthy",
			"version": webApp.Version,
			"commit":  webApp.Commit,
		}, "Health check successful")
	}
}

func DiscordOAuth(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Generate state parameter
		state, err := webApp.OAuthService.GenerateState()
		if err != nil {
			slog.Error("Failed to generate OAuth state", slog.String("error", err.Error()))
			return utils.SendInternalServerError(c, "Failed to initiate authentication")
		}

		// Store state in secure cookie
		if err := webApp.SessionService.SetState(c, state); err != nil {
			slog.Error("Failed to set OAuth state", slog.String("error", err.Error()))
			return utils.SendInternalServerError(c, "Failed to initiate authentication")
		}

		// Generate Discord OAuth URL
		authURL := webApp.OAuthService.GenerateAuthURL(state)

		// Redirect to Discord
		return c.Redirect(authURL)
	}
}

func OAuthCallback(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()

		// Get and validate state
		expectedState, err := webApp.SessionService.GetAndClearState(c)
		if err != nil {
			slog.Warn("OAuth callback: invalid or missing state", slog.String("error", err.Error()))
			return c.Redirect("http://localhost:3000/login?error=invalid_state")
		}

		receivedState := c.Query("state")
		if receivedState != expectedState {
			slog.Warn("OAuth callback: state mismatch",
				slog.String("expected", expectedState),
				slog.String("received", receivedState))
			return c.Redirect("http://localhost:3000/login?error=state_mismatch")
		}

		// Check for error parameter
		if errorParam := c.Query("error"); errorParam != "" {
			slog.Warn("OAuth callback: Discord returned error",
				slog.String("error", errorParam),
				slog.String("description", c.Query("error_description")))
			return c.Redirect("http://localhost:3000/login?error=oauth_error")
		}

		// Get authorization code
		code := c.Query("code")
		if code == "" {
			slog.Warn("OAuth callback: missing authorization code")
			return c.Redirect("http://localhost:3000/login?error=missing_code")
		}

		// Exchange code for access token
		accessToken, err := webApp.OAuthService.ExchangeCodeForToken(ctx, code)
		if err != nil {
			slog.Error("OAuth callback: failed to exchange code for token",
				slog.String("error", err.Error()))
			return c.Redirect("http://localhost:3000/login?error=token_exchange_failed")
		}

		// Get user information
		user, err := webApp.OAuthService.GetUserInfo(ctx, accessToken)
		if err != nil {
			slog.Error("OAuth callback: failed to get user info",
				slog.String("error", err.Error()))
			return c.Redirect("http://localhost:3000/login?error=user_info_failed")
		}

		// Create user session
		userSession, err := webApp.OAuthService.CreateUserSession(ctx, user, accessToken)
		if err != nil {
			slog.Error("OAuth callback: failed to create user session",
				slog.String("user_id", user.ID),
				slog.String("error", err.Error()))
			return c.Redirect("http://localhost:3000/login?error=session_creation_failed")
		}

		// Check if user has admin access
		if !userSession.IsAdmin {
			slog.Warn("OAuth callback: user lacks admin privileges",
				slog.String("user_id", user.ID),
				slog.String("username", user.Username))
			return c.Redirect("http://localhost:3000/login?error=insufficient_permissions")
		}

		// Create session cookie
		if err := webApp.SessionService.CreateSession(c, userSession); err != nil {
			slog.Error("OAuth callback: failed to create session cookie",
				slog.String("user_id", user.ID),
				slog.String("error", err.Error()))
			return c.Redirect("http://localhost:3000/login?error=session_cookie_failed")
		}

		slog.Info("OAuth callback: user authenticated successfully",
			slog.String("user_id", user.ID),
			slog.String("username", user.Username),
			slog.Bool("is_admin", userSession.IsAdmin))

		// Redirect to Next.js frontend dashboard
		return c.Redirect("http://localhost:3000/dashboard")
	}
}

func Logout(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Destroy session
		webApp.SessionService.DestroySession(c)

		// Always return JSON for API calls
		return utils.SendSuccess(c, nil, "Logged out successfully")
	}
}

func ValidateSession(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get current user session
		session, err := webApp.GetSession(c)
		if err != nil {
			return utils.SendUnauthorized(c, "Invalid session")
		}

		// Check if session is valid and not expired
		if session.ExpiresAt.Before(time.Now()) {
			return utils.SendUnauthorized(c, "Session expired")
		}

		// Return session data for Next.js frontend
		return utils.SendSuccess(c, fiber.Map{
			"user":        session,
			"valid":       true,
			"expires_at":  session.ExpiresAt,
			"is_admin":    session.IsAdmin,
			"permissions": session.Permissions,
		}, "Session valid")
	}
}

// =============================================================================
// CARD MANAGEMENT API (Keep - Required for Next.js)
// =============================================================================

func CardsDetail(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Parse card ID from params
		cardIDStr := c.Params("id")
		cardID, err := parseInt64(cardIDStr)
		if err != nil {
			return utils.SendError(c, 400, "INVALID_CARD_ID", "Invalid card ID", map[string]string{
				"card_id": cardIDStr,
			})
		}

		// Get card details
		card, err := webApp.CardMgmtService.GetCard(ctx, cardID)
		if err != nil {
			slog.Error("Failed to get card details", 
				slog.Int64("card_id", cardID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 404, "CARD_NOT_FOUND", "Card not found", nil)
		}

		return utils.SendSuccess(c, card, "Card details retrieved successfully")
	}
}

func CardsCreate(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		var req webmodels.CardCreateRequest
		
		// Parse JSON body
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", "Invalid request body", map[string]string{
				"error": err.Error(),
			})
		}

		// Handle file upload if present
		if file, err := c.FormFile("image"); err == nil {
			// Read file data
			fileData, err := file.Open()
			if err == nil {
				defer fileData.Close()
				imageData := make([]byte, file.Size)
				fileData.Read(imageData)
				req.ImageData = imageData
				req.ImageName = file.Filename
			}
		}

		// Create card
		card, err := webApp.CardMgmtService.CreateCard(ctx, &req)
		if err != nil {
			slog.Error("Failed to create card", slog.String("error", err.Error()))
			return utils.SendError(c, 400, "CREATION_FAILED", "Failed to create card", map[string]string{
				"error": err.Error(),
			})
		}

		slog.Info("Card created successfully", 
			slog.Int64("card_id", card.ID),
			slog.String("name", card.Name))

		return utils.SendSuccess(c, card, "Card created successfully")
	}
}

func CardsUpdate(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Parse card ID from params
		cardIDStr := c.Params("id")
		cardID, err := parseInt64(cardIDStr)
		if err != nil {
			return utils.SendError(c, 400, "INVALID_CARD_ID", "Invalid card ID", map[string]string{
				"card_id": cardIDStr,
			})
		}

		var req webmodels.CardUpdateRequest
		
		// Parse JSON body
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", "Invalid request body", map[string]string{
				"error": err.Error(),
			})
		}

		// Handle file upload if present
		if file, err := c.FormFile("image"); err == nil {
			// Read file data
			fileData, err := file.Open()
			if err == nil {
				defer fileData.Close()
				imageData := make([]byte, file.Size)
				fileData.Read(imageData)
				req.ImageData = imageData
				req.ImageName = file.Filename
			}
		}

		// Update card
		card, err := webApp.CardMgmtService.UpdateCard(ctx, cardID, &req)
		if err != nil {
			slog.Error("Failed to update card", 
				slog.Int64("card_id", cardID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 400, "UPDATE_FAILED", "Failed to update card", map[string]string{
				"error": err.Error(),
			})
		}

		slog.Info("Card updated successfully", 
			slog.Int64("card_id", cardID),
			slog.String("name", card.Name))

		return utils.SendSuccess(c, card, "Card updated successfully")
	}
}

func CardsDelete(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Parse card ID from params
		cardIDStr := c.Params("id")
		cardID, err := parseInt64(cardIDStr)
		if err != nil {
			return utils.SendError(c, 400, "INVALID_CARD_ID", "Invalid card ID", map[string]string{
				"card_id": cardIDStr,
			})
		}

		// Get card info before deletion for logging
		card, err := webApp.CardMgmtService.GetCard(ctx, cardID)
		if err != nil {
			return utils.SendError(c, 404, "CARD_NOT_FOUND", "Card not found", nil)
		}

		// Delete card
		err = webApp.CardMgmtService.DeleteCard(ctx, cardID)
		if err != nil {
			slog.Error("Failed to delete card", 
				slog.Int64("card_id", cardID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 400, "DELETION_FAILED", "Failed to delete card", map[string]string{
				"error": err.Error(),
			})
		}

		slog.Info("Card deleted successfully", 
			slog.Int64("card_id", cardID),
			slog.String("name", card.Name))

		return utils.SendSuccess(c, nil, "Card deleted successfully")
	}
}

func CardsBulkOperation(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		var req webmodels.CardBulkOperation
		
		// Parse JSON body
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", "Invalid request body", map[string]string{
				"error": err.Error(),
			})
		}

		// Validate request
		if len(req.CardIDs) == 0 {
			return utils.SendError(c, 400, "NO_CARDS_SELECTED", "No cards selected for bulk operation", nil)
		}

		// Perform bulk operation
		err := webApp.CardMgmtService.BulkOperation(ctx, &req)
		if err != nil {
			slog.Error("Failed to perform bulk operation", 
				slog.String("operation", req.Operation),
				slog.Int("card_count", len(req.CardIDs)),
				slog.String("error", err.Error()))
			return utils.SendError(c, 400, "BULK_OPERATION_FAILED", "Failed to perform bulk operation", map[string]string{
				"operation": req.Operation,
				"error":     err.Error(),
			})
		}

		slog.Info("Bulk operation completed successfully", 
			slog.String("operation", req.Operation),
			slog.Int("card_count", len(req.CardIDs)))

		return utils.SendSuccess(c, nil, fmt.Sprintf("Bulk %s operation completed successfully", req.Operation))
	}
}

// =============================================================================
// COLLECTION MANAGEMENT API (Keep - Required for Next.js)
// =============================================================================

func CollectionsDetail(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Get collection ID from params
		collectionID := c.Params("id")
		if collectionID == "" {
			return utils.SendError(c, 400, "INVALID_COLLECTION_ID", "Collection ID is required", nil)
		}
		
		// Get pagination parameters
		page := c.QueryInt("page", 1)
		limit := c.QueryInt("limit", 24) // 24 cards per page for good grid layout
		search := c.Query("search", "")
		levelFilter := c.QueryInt("level", 0)
		typeFilter := c.Query("type", "")
		
		// Ensure valid pagination
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 24
		}
		
		// Get collection details
		collection, err := webApp.Repos.Collection.GetByID(ctx, collectionID)
		if err != nil {
			return utils.SendError(c, 404, "COLLECTION_NOT_FOUND", "Collection not found", nil)
		}
		
		// Create search filters for collection cards
		filters := repositories.SearchFilters{
			Collection: collectionID,
			Name:       search,
			Level:      levelFilter,
		}
		
		// Handle type filter
		if typeFilter == "animated" {
			filters.Animated = true
		}
		
		// Calculate offset for pagination
		offset := (page - 1) * limit
		
		// Get paginated cards
		cards, totalCount, err := webApp.Repos.Card.Search(ctx, filters, offset, limit)
		if err != nil {
			slog.Warn("Failed to get cards for collection", slog.String("collection_id", collectionID), slog.String("error", err.Error()))
			cards = []*models.Card{} // Empty fallback
			totalCount = 0
		}
		
		// Convert cards to DTOs with image URLs if we have spaces service
		cardDTOs := make([]*webmodels.CardDTO, 0, len(cards))
		if spacesService, ok := webApp.SpacesService.(*services.SpacesService); ok && spacesService != nil {
			for _, card := range cards {
				// Determine group type from collection tags
				groupType := "girlgroups" // default
				if len(collection.Tags) > 0 {
					groupType = collection.Tags[0]
				}
				
				// Generate image URL using spaces service
				imageURL := spacesService.GetCardImageURLWithFormat(
					card.Name,
					card.ColID,
					card.Level,
					groupType,
					card.Animated,
				)
				
				cardDTO := webmodels.ConvertCardToDTO(card, collection, imageURL)
				cardDTOs = append(cardDTOs, cardDTO)
			}
		} else {
			// Fallback without image URLs
			for _, card := range cards {
				cardDTO := webmodels.ConvertCardToDTO(card, collection, "")
				cardDTOs = append(cardDTOs, cardDTO)
			}
		}
		
		// Create pagination info
		pagination := webmodels.NewPaginationInfo(page, limit, int64(totalCount))
		
		// Return API response
		return utils.SendSuccess(c, fiber.Map{
			"collection": collection,
			"cards": cardDTOs,
			"pagination": pagination,
			"total_count": totalCount,
		}, "Collection details retrieved successfully")
	}
}

func CollectionsCreate(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		var req struct {
			ID          string   `json:"id"`
			Name        string   `json:"name"`
			Origin      string   `json:"origin"`
			Aliases     []string `json:"aliases"`
			Promo       bool     `json:"promo"`
			Compressed  bool     `json:"compressed"`
			Fragments   bool     `json:"fragments"`
			Tags        []string `json:"tags"`
		}
		
		// Parse JSON body
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", "Invalid request body", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Validate required fields
		if req.ID == "" || req.Name == "" {
			return utils.SendError(c, 400, "MISSING_FIELDS", "ID and Name are required", nil)
		}
		
		// Create collection with proper defaults
		collection := &models.Collection{
			ID:         req.ID,
			Name:       req.Name,
			Origin:     req.Origin,
			Aliases:    req.Aliases,
			Promo:      req.Promo,
			Compressed: req.Compressed,
			Fragments:  req.Fragments,
			Tags:       req.Tags,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		
		// Set defaults if not provided
		if collection.Origin == "" {
			collection.Origin = ""  // Empty string is the default
		}
		if collection.Aliases == nil {
			collection.Aliases = []string{collection.ID}  // Default to ID in aliases
		}
		
		// Save to database
		err := webApp.Repos.Collection.Create(ctx, collection)
		if err != nil {
			slog.Error("Failed to create collection", slog.String("error", err.Error()))
			return utils.SendError(c, 400, "CREATION_FAILED", "Failed to create collection", map[string]string{
				"error": err.Error(),
			})
		}
		
		slog.Info("Collection created successfully", 
			slog.String("collection_id", collection.ID),
			slog.String("name", collection.Name))
		
		return utils.SendSuccess(c, collection, "Collection created successfully")
	}
}

func CollectionsUpdate(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Get collection ID from params
		collectionID := c.Params("id")
		if collectionID == "" {
			return utils.SendError(c, 400, "INVALID_COLLECTION_ID", "Collection ID is required", nil)
		}
		
		var req struct {
			Name        *string  `json:"name"`
			Origin      *string  `json:"origin"`
			Aliases     []string `json:"aliases"`
			Promo       *bool    `json:"promo"`
			Compressed  *bool    `json:"compressed"`
			Fragments   *bool    `json:"fragments"`
			Tags        []string `json:"tags"`
		}
		
		// Parse JSON body
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", "Invalid request body", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Get existing collection
		collection, err := webApp.Repos.Collection.GetByID(ctx, collectionID)
		if err != nil {
			return utils.SendError(c, 404, "COLLECTION_NOT_FOUND", "Collection not found", nil)
		}
		
		// Update fields if provided
		if req.Name != nil {
			collection.Name = *req.Name
		}
		if req.Origin != nil {
			collection.Origin = *req.Origin
		}
		if req.Aliases != nil {
			collection.Aliases = req.Aliases
		}
		if req.Promo != nil {
			collection.Promo = *req.Promo
		}
		if req.Compressed != nil {
			collection.Compressed = *req.Compressed
		}
		if req.Fragments != nil {
			collection.Fragments = *req.Fragments
		}
		if req.Tags != nil {
			collection.Tags = req.Tags
		}
		
		// Update in database
		err = webApp.Repos.Collection.Update(ctx, collection)
		if err != nil {
			slog.Error("Failed to update collection", 
				slog.String("collection_id", collectionID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 400, "UPDATE_FAILED", "Failed to update collection", map[string]string{
				"error": err.Error(),
			})
		}
		
		slog.Info("Collection updated successfully", 
			slog.String("collection_id", collectionID),
			slog.String("name", collection.Name))
		
		return utils.SendSuccess(c, collection, "Collection updated successfully")
	}
}

func CollectionsDelete(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Get collection ID from params
		collectionID := c.Params("id")
		if collectionID == "" {
			return utils.SendError(c, 400, "INVALID_COLLECTION_ID", "Collection ID is required", nil)
		}
		
		// Get collection info before deletion for logging
		collection, err := webApp.Repos.Collection.GetByID(ctx, collectionID)
		if err != nil {
			return utils.SendError(c, 404, "COLLECTION_NOT_FOUND", "Collection not found", nil)
		}
		
		// Check if collection has cards
		cards, err := webApp.Repos.Card.GetByCollectionID(ctx, collectionID)
		if err == nil && len(cards) > 0 {
			return utils.SendError(c, 400, "COLLECTION_HAS_CARDS", "Cannot delete collection with existing cards", map[string]string{
				"card_count": fmt.Sprintf("%d", len(cards)),
			})
		}
		
		// Delete collection
		err = webApp.Repos.Collection.Delete(ctx, collectionID)
		if err != nil {
			slog.Error("Failed to delete collection", 
				slog.String("collection_id", collectionID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 400, "DELETION_FAILED", "Failed to delete collection", map[string]string{
				"error": err.Error(),
			})
		}
		
		slog.Info("Collection deleted successfully", 
			slog.String("collection_id", collectionID),
			slog.String("name", collection.Name))
		
		return utils.SendSuccess(c, nil, "Collection deleted successfully")
	}
}

func CollectionsImport(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Parse form data
		form, err := c.MultipartForm()
		if err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", "Invalid multipart form", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Extract form fields
		collectionID := ""
		displayName := ""
		groupType := ""
		isPromo := false
		
		if values, ok := form.Value["collection_id"]; ok && len(values) > 0 {
			collectionID = values[0]
		}
		if values, ok := form.Value["display_name"]; ok && len(values) > 0 {
			displayName = values[0]
		}
		if values, ok := form.Value["group_type"]; ok && len(values) > 0 {
			groupType = values[0]
		}
		if values, ok := form.Value["is_promo"]; ok && len(values) > 0 {
			isPromo = values[0] == "true"
		}
		
		// Validate required fields
		if collectionID == "" || displayName == "" || groupType == "" {
			return utils.SendError(c, 400, "MISSING_FIELDS", "Missing required fields", nil)
		}
		
		// Validate group type
		if groupType != "girlgroups" && groupType != "boygroups" {
			return utils.SendError(c, 400, "INVALID_GROUP_TYPE", "Group type must be 'girlgroups' or 'boygroups'", nil)
		}
		
		// Process uploaded files
		files := []*webmodels.FileUpload{}
		if fileHeaders, ok := form.File["files"]; ok {
			for _, fileHeader := range fileHeaders {
				// Open the file
				file, err := fileHeader.Open()
				if err != nil {
					return utils.SendError(c, 400, "FILE_ERROR", fmt.Sprintf("Failed to open file %s", fileHeader.Filename), nil)
				}
				defer file.Close()
				
				// Read file data
				fileData, err := io.ReadAll(file)
				if err != nil {
					return utils.SendError(c, 400, "FILE_ERROR", fmt.Sprintf("Failed to read file %s: %s", fileHeader.Filename, err.Error()), nil)
				}
				
				// Create FileUpload struct
				fileUpload := &webmodels.FileUpload{
					Name:        fileHeader.Filename,
					Size:        fileHeader.Size,
					ContentType: fileHeader.Header.Get("Content-Type"),
					Data:        fileData,
				}
				files = append(files, fileUpload)
			}
		}
		
		if len(files) == 0 {
			return utils.SendError(c, 400, "NO_FILES", "No files uploaded", nil)
		}
		
		// Create request struct
		req := &webmodels.CollectionImportRequest{
			CollectionID: collectionID,
			DisplayName:  displayName,
			GroupType:    groupType,
			IsPromo:      isPromo,
			Files:        files,
		}
		
		result, err := webApp.CollectionImportService.ProcessCollectionImport(ctx, req)
		if err != nil {
			return utils.SendError(c, 500, "IMPORT_FAILED", err.Error(), nil)
		}
		
		if !result.Success {
			return utils.SendError(c, 400, "IMPORT_FAILED", result.ErrorMessage, nil)
		}
		
		return utils.SendSuccess(c, result, "Collection imported successfully")
	}
}

// =============================================================================
// API ENDPOINTS FOR NEXT.JS (Keep - Required)
// =============================================================================

func CardsAPI(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Parse search parameters
		var searchReq webmodels.CardSearchRequest
		if err := c.QueryParser(&searchReq); err != nil {
			return utils.SendError(c, 400, "INVALID_PARAMETERS", "Invalid search parameters", map[string]string{
				"error": err.Error(),
			})
		}

		// Validate and set defaults
		if err := searchReq.Validate(); err != nil {
			return utils.SendError(c, 400, "INVALID_PARAMETERS", "Invalid search parameters", map[string]string{
				"error": err.Error(),
			})
		}

		// Search cards
		cards, total, err := webApp.CardMgmtService.SearchCards(ctx, &searchReq)
		if err != nil {
			slog.Error("Failed to search cards via API", slog.String("error", err.Error()))
			return utils.SendError(c, 500, "SEARCH_FAILED", "Failed to search cards", map[string]string{
				"error": err.Error(),
			})
		}

		// Calculate pagination info
		totalPages := (total + int64(searchReq.Limit) - 1) / int64(searchReq.Limit)
		hasMore := int64(searchReq.Page) < totalPages

		// Return API response with pagination info
		return utils.SendSuccess(c, fiber.Map{
			"cards":       cards,
			"total":       total,
			"page":        searchReq.Page,
			"limit":       searchReq.Limit,
			"total_pages": totalPages,
			"has_more":    hasMore,
			"has_prev":    searchReq.Page > 1,
		}, "Cards retrieved successfully")
	}
}

func UploadAPI(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Parse multipart form
		form, err := c.MultipartForm()
		if err != nil {
			return utils.SendError(c, 400, "INVALID_FORM", "Invalid multipart form", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Get files from form
		files := form.File["images"]
		if len(files) == 0 {
			return utils.SendError(c, 400, "NO_FILES", "No files uploaded", nil)
		}
		
		// Process each file
		results := make([]fiber.Map, 0, len(files))
		for _, file := range files {
			result := processUploadedFile(ctx, webApp, file)
			results = append(results, result)
		}
		
		return utils.SendSuccess(c, fiber.Map{
			"files": results,
			"total": len(results),
		}, "Files processed successfully")
	}
}

func DashboardStatsAPI(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Get dashboard statistics using existing getDashboardStats function
		stats, err := getDashboardStats(ctx, webApp)
		if err != nil {
			slog.Error("Failed to get dashboard stats for API", slog.String("error", err.Error()))
			return utils.SendError(c, 500, "STATS_FAILED", "Failed to retrieve dashboard statistics", map[string]string{
				"error": err.Error(),
			})
		}
		
		return utils.SendSuccess(c, stats, "Dashboard statistics retrieved successfully")
	}
}

func ActivityAPI(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// For now, return empty activity list
		// TODO: Implement real activity tracking
		activities := []webmodels.ActivityItem{}
		
		return utils.SendSuccess(c, activities, "Recent activity retrieved successfully")
	}
}

// =============================================================================
// MISSING HANDLERS (Added for compilation)
// =============================================================================

func CollectionsImportPage(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return utils.SendSuccess(c, fiber.Map{
			"message": "Collection import page - use Next.js frontend",
			"redirect": "http://localhost:3000/collections/import",
		}, "Import page available on frontend")
	}
}

func SyncStatus(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Get sync status from sync manager service
		statuses, err := webApp.SyncMgrService.GetSyncStatus(ctx)
		if err != nil {
			slog.Error("Failed to get sync status", slog.String("error", err.Error()))
			return utils.SendError(c, 500, "SYNC_STATUS_FAILED", "Failed to retrieve sync status", map[string]string{
				"error": err.Error(),
			})
		}
		
		return utils.SendSuccess(c, statuses, "Sync status retrieved successfully")
	}
}

func SyncFix(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Get collection ID from query or body
		collectionID := c.Query("collection_id")
		if collectionID == "" {
			return utils.SendError(c, 400, "MISSING_COLLECTION_ID", "Collection ID is required", nil)
		}
		
		// Fix sync issues for the collection
		updatedStatus, err := webApp.SyncMgrService.FixSyncIssues(ctx, collectionID)
		if err != nil {
			slog.Error("Failed to fix sync issues", 
				slog.String("collection_id", collectionID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 500, "SYNC_FIX_FAILED", "Failed to fix sync issues", map[string]string{
				"collection_id": collectionID,
				"error": err.Error(),
			})
		}
		
		return utils.SendSuccess(c, updatedStatus, "Sync issues fixed successfully")
	}
}

func SyncCleanup(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Cleanup orphaned files
		cleanedCount, err := webApp.SyncMgrService.CleanupOrphans(ctx)
		if err != nil {
			slog.Error("Failed to cleanup orphans", slog.String("error", err.Error()))
			return utils.SendError(c, 500, "CLEANUP_FAILED", "Failed to cleanup orphaned files", map[string]string{
				"error": err.Error(),
			})
		}
		
		return utils.SendSuccess(c, fiber.Map{
			"cleaned_count": cleanedCount,
		}, fmt.Sprintf("Cleanup completed, removed %d orphaned files", cleanedCount))
	}
}

func UsersDetail(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Get user ID from params
		userID := c.Params("id")
		if userID == "" {
			return utils.SendError(c, 400, "MISSING_USER_ID", "User ID is required", nil)
		}
		
		// Get user details
		user, err := webApp.Repos.User.GetByDiscordID(ctx, userID)
		if err != nil {
			slog.Error("Failed to get user details", 
				slog.String("user_id", userID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 404, "USER_NOT_FOUND", "User not found", map[string]string{
				"user_id": userID,
			})
		}
		
		return utils.SendSuccess(c, user, "User details retrieved successfully")
	}
}

func CollectionsAPI(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Get all collections with card counts using optimized query
		collectionsWithCounts, err := webApp.Repos.Collection.GetAllWithCardCounts(ctx)
		if err != nil {
			slog.Error("Failed to get collections for API", slog.String("error", err.Error()))
			return utils.SendError(c, 500, "COLLECTIONS_FAILED", "Failed to retrieve collections", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Convert to DTOs
		collectionDTOs := make([]webmodels.CollectionDTO, len(collectionsWithCounts))
		for i, collectionWithCount := range collectionsWithCounts {
			// Determine collection type based on tags or name
			collectionType := determineCollectionType(collectionWithCount.Collection)
			
			collectionDTOs[i] = webmodels.CollectionDTO{
				ID:             collectionWithCount.ID,
				Name:           collectionWithCount.Name,
				Description:    "", // No description field in database model
				CollectionType: collectionType,
				Origin:         collectionWithCount.Origin,
				Aliases:        collectionWithCount.Aliases,
				Promo:          collectionWithCount.Promo,
				Compressed:     collectionWithCount.Compressed,
				Fragments:      collectionWithCount.Fragments,
				Tags:           collectionWithCount.Tags,
				CardCount:      collectionWithCount.CardCount, // Optimized card count from JOIN query
				CreatedAt:      collectionWithCount.CreatedAt,
				UpdatedAt:      collectionWithCount.UpdatedAt,
			}
		}
		
		return utils.SendSuccess(c, collectionDTOs, "Collections retrieved successfully")
	}
}

// determineCollectionType infers the collection type from available data
func determineCollectionType(collection *models.Collection) string {
	// Check tags first
	for _, tag := range collection.Tags {
		tag = strings.ToLower(tag)
		if strings.Contains(tag, "girl") || strings.Contains(tag, "female") {
			return "girl_group"
		}
		if strings.Contains(tag, "boy") || strings.Contains(tag, "male") {
			return "boy_group"
		}
	}
	
	// Check name as fallback
	name := strings.ToLower(collection.Name)
	if strings.Contains(name, "girl") || strings.Contains(name, "female") {
		return "girl_group"
	}
	if strings.Contains(name, "boy") || strings.Contains(name, "male") {
		return "boy_group"
	}
	
	// Default to "other"
	return "other"
}

func CollectionCardsAPI(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Get collection ID from params
		collectionID := c.Params("id")
		if collectionID == "" {
			return utils.SendError(c, 400, "MISSING_COLLECTION_ID", "Collection ID is required", nil)
		}
		
		// Get collection info first to determine group type
		collection, err := webApp.Repos.Collection.GetByID(ctx, collectionID)
		if err != nil {
			slog.Error("Failed to get collection", 
				slog.String("collection_id", collectionID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 500, "COLLECTION_FAILED", "Failed to retrieve collection", map[string]string{
				"collection_id": collectionID,
				"error": err.Error(),
			})
		}
		
		// Get cards for this collection
		cards, err := webApp.Repos.Card.GetByCollectionID(ctx, collectionID)
		if err != nil {
			slog.Error("Failed to get cards for collection", 
				slog.String("collection_id", collectionID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 500, "CARDS_FAILED", "Failed to retrieve cards for collection", map[string]string{
				"collection_id": collectionID,
				"error": err.Error(),
			})
		}
		
		// Determine group type from collection
		groupType := ""
		if collection != nil && len(collection.Tags) > 0 {
			groupType = collection.Tags[0]
		}
		
		// Convert to DTOs with image URLs
		cardDTOs := make([]webmodels.CardDTO, len(cards))
		for i, card := range cards {
			imageURL := ""
			if webApp.SpacesService != nil {
				if spacesService, ok := webApp.SpacesService.(*services.SpacesService); ok {
					// Use the correct method with proper parameters including group type
					imageURL = spacesService.GetCardImageURLWithFormat(card.Name, card.ColID, card.Level, groupType, card.Animated)
				}
			}
			
			cardDTOs[i] = webmodels.CardDTO{
				ID:           card.ID,
				Name:         card.Name,
				Level:        card.Level,
				Animated:     card.Animated,
				ColID:        card.ColID,
				Tags:         card.Tags,
				ImageURL:     imageURL,
				CreatedAt:    card.CreatedAt,
				UpdatedAt:    card.UpdatedAt,
			}
		}
		
		return utils.SendSuccess(c, cardDTOs, "Cards retrieved successfully")
	}
}

func ProgressAPI(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get task ID from params
		taskID := c.Params("id")
		if taskID == "" {
			return utils.SendError(c, 400, "MISSING_TASK_ID", "Task ID is required", nil)
		}
		
		// For now, return placeholder progress
		// TODO: Implement actual progress tracking
		progress := fiber.Map{
			"task_id": taskID,
			"status": "completed",
			"progress": 100,
			"message": "Task completed successfully",
		}
		
		return utils.SendSuccess(c, progress, "Progress retrieved successfully")
	}
}

// =============================================================================
// CARD IMPORT API ENDPOINTS
// =============================================================================

// CardsImportValidate validates card import files without processing
func CardsImportValidate(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Parse multipart form
		form, err := c.MultipartForm()
		if err != nil {
			return utils.SendError(c, 400, "INVALID_FORM", "Invalid multipart form", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Extract form fields
		req, err := parseCardImportRequest(form)
		if err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", err.Error(), nil)
		}
		
		// Set validate only mode
		req.ValidateOnly = true
		
		// Validate files
		result, err := webApp.CardImportService.ImportCards(ctx, req)
		if err != nil {
			slog.Error("Failed to validate card import", 
				slog.String("collection_id", req.CollectionID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 400, "VALIDATION_FAILED", "Validation failed", map[string]string{
				"error": err.Error(),
			})
		}
		
		return utils.SendSuccess(c, result, "Validation completed")
	}
}

// CardsImport processes card import with full pipeline
func CardsImport(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		// Parse multipart form
		form, err := c.MultipartForm()
		if err != nil {
			return utils.SendError(c, 400, "INVALID_FORM", "Invalid multipart form", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Extract form fields and files
		req, err := parseCardImportRequest(form)
		if err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", err.Error(), nil)
		}
		
		// Process import
		result, err := webApp.CardImportService.ImportCards(ctx, req)
		if err != nil {
			slog.Error("Failed to import cards", 
				slog.String("collection_id", req.CollectionID),
				slog.String("error", err.Error()))
			return utils.SendError(c, 500, "IMPORT_FAILED", "Import failed", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Log successful import
		if result.Success {
			slog.Info("Card import completed successfully",
				slog.String("collection_id", req.CollectionID),
				slog.Int("cards_created", result.CardsCreated),
				slog.Int("files_processed", len(result.FilesUploaded)),
				slog.Int64("duration_ms", result.ProcessingTimeMs))
		}
		
		return utils.SendSuccess(c, result, "Import completed")
	}
}

// CardsBulkOperations handles enhanced bulk operations
func CardsBulkOperations(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.Context()
		
		var req webmodels.CardBatchOperation
		
		// Parse JSON body
		if err := c.BodyParser(&req); err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", "Invalid request body", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Validate request
		if err := req.Validate(); err != nil {
			return utils.SendError(c, 400, "INVALID_REQUEST", err.Error(), nil)
		}
		
		// Execute bulk operation based on type
		var result *webmodels.CardBatchResult
		var err error
		
		switch req.Operation {
		case "delete":
			result, err = webApp.executeBulkDelete(ctx, &req)
		case "update":
			result, err = webApp.executeBulkUpdate(ctx, &req)
		case "move":
			result, err = webApp.executeBulkMove(ctx, &req)
		case "level_update":
			result, err = webApp.executeBulkLevelUpdate(ctx, &req)
		case "toggle_animated":
			result, err = webApp.executeBulkToggleAnimated(ctx, &req)
		case "export":
			result, err = webApp.executeBulkExport(ctx, &req)
		default:
			return utils.SendError(c, 400, "UNSUPPORTED_OPERATION", "Unsupported bulk operation", map[string]string{
				"operation": req.Operation,
			})
		}
		
		if err != nil {
			slog.Error("Failed to execute bulk operation",
				slog.String("operation", req.Operation),
				slog.Int("card_count", len(req.CardIDs)),
				slog.String("error", err.Error()))
			return utils.SendError(c, 400, "BULK_OPERATION_FAILED", "Bulk operation failed", map[string]string{
				"operation": req.Operation,
				"error": err.Error(),
			})
		}
		
		// Log successful operation
		slog.Info("Bulk operation completed",
			slog.String("operation", req.Operation),
			slog.Int("total_cards", result.TotalCards),
			slog.Int("processed_cards", result.ProcessedCards),
			slog.Bool("success", result.Success))
		
		return utils.SendSuccess(c, result, fmt.Sprintf("Bulk %s operation completed", req.Operation))
	}
}

// CardsImportCollections handles batch import of multiple collections
func CardsImportCollections(webApp *WebApp) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse multipart form for batch collection import
		form, err := c.MultipartForm()
		if err != nil {
			return utils.SendError(c, 400, "INVALID_FORM", "Invalid multipart form", map[string]string{
				"error": err.Error(),
			})
		}
		
		// Extract collections data (JSON format)
		collectionsJSON := ""
		if values, ok := form.Value["collections"]; ok && len(values) > 0 {
			collectionsJSON = values[0]
		}
		
		if collectionsJSON == "" {
			return utils.SendError(c, 400, "MISSING_COLLECTIONS", "Collections data is required", nil)
		}
		
		// TODO: Implement batch collection import
		// For now, return placeholder
		result := fiber.Map{
			"message": "Batch collection import not yet implemented",
			"use_single_import": "/api/cards/import",
		}
		
		return utils.SendSuccess(c, result, "Batch import placeholder")
	}
}

// =============================================================================
// HELPER FUNCTIONS FOR CARD IMPORT
// =============================================================================

// parseCardImportRequest parses multipart form data into CardImportRequest
func parseCardImportRequest(form *multipart.Form) (*webmodels.CardImportRequest, error) {
	// Extract form fields
	collectionID := ""
	displayName := ""
	groupType := ""
	isPromo := false
	createCollection := false
	overwriteMode := "skip"
	
	if values, ok := form.Value["collection_id"]; ok && len(values) > 0 {
		collectionID = values[0]
	}
	if values, ok := form.Value["display_name"]; ok && len(values) > 0 {
		displayName = values[0]
	}
	if values, ok := form.Value["group_type"]; ok && len(values) > 0 {
		groupType = values[0]
	}
	if values, ok := form.Value["is_promo"]; ok && len(values) > 0 {
		isPromo = values[0] == "true"
	}
	if values, ok := form.Value["create_collection"]; ok && len(values) > 0 {
		createCollection = values[0] == "true"
	}
	if values, ok := form.Value["overwrite_mode"]; ok && len(values) > 0 {
		overwriteMode = values[0]
	}
	
	// Validate required fields
	if collectionID == "" {
		return nil, fmt.Errorf("collection_id is required")
	}
	if displayName == "" {
		return nil, fmt.Errorf("display_name is required")
	}
	if groupType == "" {
		return nil, fmt.Errorf("group_type is required")
	}
	
	// Process uploaded files
	files := []*webmodels.FileUpload{}
	if fileHeaders, ok := form.File["files"]; ok {
		for _, fileHeader := range fileHeaders {
			// Open the file
			file, err := fileHeader.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open file %s: %w", fileHeader.Filename, err)
			}
			defer file.Close()
			
			// Read file data
			fileData, err := io.ReadAll(file)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", fileHeader.Filename, err)
			}
			
			// Create FileUpload struct
			fileUpload := &webmodels.FileUpload{
				Name:        fileHeader.Filename,
				Size:        fileHeader.Size,
				ContentType: fileHeader.Header.Get("Content-Type"),
				Data:        fileData,
			}
			files = append(files, fileUpload)
		}
	}
	
	if len(files) == 0 {
		return nil, fmt.Errorf("no files uploaded")
	}
	
	// Create request struct
	req := &webmodels.CardImportRequest{
		CollectionID:     collectionID,
		DisplayName:      displayName,
		GroupType:        groupType,
		IsPromo:          isPromo,
		Files:            files,
		CreateCollection: createCollection,
		OverwriteMode:    overwriteMode,
	}
	
	return req, nil
}

// executeBulkDelete executes bulk delete operation
func (w *WebApp) executeBulkDelete(ctx context.Context, req *webmodels.CardBatchOperation) (*webmodels.CardBatchResult, error) {
	result := &webmodels.CardBatchResult{
		Operation:   req.Operation,
		TotalCards:  len(req.CardIDs),
		DryRun:      req.DryRun,
		Errors:      make([]webmodels.CardOperationError, 0),
	}
	
	for _, cardID := range req.CardIDs {
		if req.DryRun {
			// For dry run, just check if card exists
			card, err := w.CardMgmtService.GetCard(ctx, cardID)
			if err != nil {
				result.Errors = append(result.Errors, webmodels.CardOperationError{
					CardID:      cardID,
					CardName:    "",
					ErrorType:   "not_found",
					Description: "Card not found",
				})
				result.FailedCards++
			} else {
				result.PreviewResults = append(result.PreviewResults, webmodels.CardPreview{
					CardID:   cardID,
					CardName: card.Name,
					Changes: map[string]interface{}{
						"action": "delete",
					},
				})
				result.ProcessedCards++
			}
		} else {
			// Actually delete the card
			if err := w.CardMgmtService.DeleteCard(ctx, cardID); err != nil {
				result.Errors = append(result.Errors, webmodels.CardOperationError{
					CardID:      cardID,
					ErrorType:   "delete_failed",
					Description: err.Error(),
				})
				result.FailedCards++
			} else {
				result.ProcessedCards++
			}
		}
	}
	
	result.Success = result.FailedCards == 0
	return result, nil
}

// executeBulkUpdate executes bulk update operation
func (w *WebApp) executeBulkUpdate(ctx context.Context, req *webmodels.CardBatchOperation) (*webmodels.CardBatchResult, error) {
	result := &webmodels.CardBatchResult{
		Operation:   req.Operation,
		TotalCards:  len(req.CardIDs),
		DryRun:      req.DryRun,
		Errors:      make([]webmodels.CardOperationError, 0),
	}
	
	for _, cardID := range req.CardIDs {
		if req.DryRun {
			// For dry run, preview the changes
			card, err := w.CardMgmtService.GetCard(ctx, cardID)
			if err != nil {
				result.Errors = append(result.Errors, webmodels.CardOperationError{
					CardID:      cardID,
					ErrorType:   "not_found",
					Description: "Card not found",
				})
				result.FailedCards++
			} else {
				changes := make(map[string]interface{})
				if req.Updates.Name != nil {
					changes["name"] = *req.Updates.Name
				}
				if req.Updates.Level != nil {
					changes["level"] = *req.Updates.Level
				}
				if req.Updates.Animated != nil {
					changes["animated"] = *req.Updates.Animated
				}
				if req.Updates.ColID != nil {
					changes["collection"] = *req.Updates.ColID
				}
				
				result.PreviewResults = append(result.PreviewResults, webmodels.CardPreview{
					CardID:   cardID,
					CardName: card.Name,
					Changes:  changes,
				})
				result.ProcessedCards++
			}
		} else {
			// Actually update the card
			if _, err := w.CardMgmtService.UpdateCard(ctx, cardID, req.Updates); err != nil {
				result.Errors = append(result.Errors, webmodels.CardOperationError{
					CardID:      cardID,
					ErrorType:   "update_failed",
					Description: err.Error(),
				})
				result.FailedCards++
			} else {
				result.ProcessedCards++
			}
		}
	}
	
	result.Success = result.FailedCards == 0
	return result, nil
}

// executeBulkMove executes bulk move operation
func (w *WebApp) executeBulkMove(ctx context.Context, req *webmodels.CardBatchOperation) (*webmodels.CardBatchResult, error) {
	// Create update request for moving to target collection
	updates := &webmodels.CardUpdateRequest{
		ColID: &req.TargetCollection,
	}
	
	// Use bulk update with the move operation
	moveReq := &webmodels.CardBatchOperation{
		Operation: "update",
		CardIDs:   req.CardIDs,
		Updates:   updates,
		DryRun:    req.DryRun,
	}
	
	return w.executeBulkUpdate(ctx, moveReq)
}

// executeBulkLevelUpdate executes bulk level update operation
func (w *WebApp) executeBulkLevelUpdate(ctx context.Context, req *webmodels.CardBatchOperation) (*webmodels.CardBatchResult, error) {
	// Create update request for level change
	updates := &webmodels.CardUpdateRequest{
		Level: req.NewLevel,
	}
	
	// Use bulk update with the level change
	levelReq := &webmodels.CardBatchOperation{
		Operation: "update",
		CardIDs:   req.CardIDs,
		Updates:   updates,
		DryRun:    req.DryRun,
	}
	
	return w.executeBulkUpdate(ctx, levelReq)
}

// executeBulkToggleAnimated executes bulk toggle animated operation
func (w *WebApp) executeBulkToggleAnimated(ctx context.Context, req *webmodels.CardBatchOperation) (*webmodels.CardBatchResult, error) {
	result := &webmodels.CardBatchResult{
		Operation:   req.Operation,
		TotalCards:  len(req.CardIDs),
		DryRun:      req.DryRun,
		Errors:      make([]webmodels.CardOperationError, 0),
	}
	
	for _, cardID := range req.CardIDs {
		card, err := w.CardMgmtService.GetCard(ctx, cardID)
		if err != nil {
			result.Errors = append(result.Errors, webmodels.CardOperationError{
				CardID:      cardID,
				ErrorType:   "not_found",
				Description: "Card not found",
			})
			result.FailedCards++
			continue
		}
		
		// Toggle animated status
		newAnimated := !card.Animated
		
		if req.DryRun {
			result.PreviewResults = append(result.PreviewResults, webmodels.CardPreview{
				CardID:   cardID,
				CardName: card.Name,
				Changes: map[string]interface{}{
					"animated": newAnimated,
				},
			})
			result.ProcessedCards++
		} else {
			updates := &webmodels.CardUpdateRequest{
				Animated: &newAnimated,
			}
			
			if _, err := w.CardMgmtService.UpdateCard(ctx, cardID, updates); err != nil {
				result.Errors = append(result.Errors, webmodels.CardOperationError{
					CardID:      cardID,
					CardName:    card.Name,
					ErrorType:   "toggle_failed",
					Description: err.Error(),
				})
				result.FailedCards++
			} else {
				result.ProcessedCards++
			}
		}
	}
	
	result.Success = result.FailedCards == 0
	return result, nil
}

// executeBulkExport executes bulk export operation
func (w *WebApp) executeBulkExport(ctx context.Context, req *webmodels.CardBatchOperation) (*webmodels.CardBatchResult, error) {
	result := &webmodels.CardBatchResult{
		Operation:   req.Operation,
		TotalCards:  len(req.CardIDs),
		DryRun:      false, // Export is always a read operation
		Errors:      make([]webmodels.CardOperationError, 0),
	}
	
	// TODO: Implement actual export functionality
	// For now, just mark as processed
	result.ProcessedCards = len(req.CardIDs)
	result.Success = true
	
	return result, nil
}