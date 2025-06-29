package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	webmodels "github.com/disgoorg/bot-template/backend/models"
)

// CardManagementService provides card management operations for the web interface
type CardManagementService struct {
	repos         *webmodels.Repositories
	spacesService *services.SpacesService
}

// NewCardManagementService creates a new card management service
func NewCardManagementService(repos *webmodels.Repositories, spacesService *services.SpacesService) *CardManagementService {
	return &CardManagementService{
		repos:         repos,
		spacesService: spacesService,
	}
}

// SearchCards searches for cards based on the provided filters
func (cms *CardManagementService) SearchCards(ctx context.Context, req *webmodels.CardSearchRequest) ([]*webmodels.CardDTO, int64, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, 0, fmt.Errorf("invalid search request: %w", err)
	}

	// Build search filters
	filters := repositories.SearchFilters{
		Name:       req.Query,
		Level:      req.Level,
		Collection: req.Collection,
		Type:       "", // Will be determined from collection if needed
	}

	if req.Animated != nil {
		filters.Animated = *req.Animated
	}

	// Calculate offset for pagination
	offset := (req.Page - 1) * req.Limit

	// Search cards
	cards, total, err := cms.repos.Card.Search(ctx, filters, offset, req.Limit)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search cards: %w", err)
	}

	// Optimize: Batch fetch collections to avoid N+1 query problem
	collectionIDs := make([]string, 0, len(cards))
	collectionMap := make(map[string]*models.Collection)
	
	// Collect unique collection IDs
	seen := make(map[string]bool)
	for _, card := range cards {
		if !seen[card.ColID] {
			collectionIDs = append(collectionIDs, card.ColID)
			seen[card.ColID] = true
		}
	}
	
	// Batch fetch collections
	if len(collectionIDs) > 0 {
		collections, err := cms.repos.Collection.GetByIDs(ctx, collectionIDs)
		if err != nil {
			slog.Warn("Failed to batch fetch collections", slog.String("error", err.Error()))
		} else {
			for _, collection := range collections {
				collectionMap[collection.ID] = collection
			}
		}
	}

	// Convert to DTOs with pre-fetched collections
	cardDTOs := make([]*webmodels.CardDTO, len(cards))
	for i, card := range cards {
		collection := collectionMap[card.ColID]
		
		// Generate image URL (optimize: determine format based on animated flag)
		groupType := utils.GetGroupType(card.Tags)
		imageURL := cms.getOptimizedImageURL(card, groupType)

		cardDTOs[i] = webmodels.ConvertCardToDTO(card, collection, imageURL)
	}

	return cardDTOs, int64(total), nil
}

// GetCard retrieves a single card by ID
func (cms *CardManagementService) GetCard(ctx context.Context, cardID int64) (*webmodels.CardDTO, error) {
	card, err := cms.repos.Card.GetByID(ctx, cardID)
	if err != nil {
		return nil, fmt.Errorf("failed to get card: %w", err)
	}

	// Get collection info
	collection, err := cms.repos.Collection.GetByID(ctx, card.ColID)
	if err != nil {
		slog.Warn("Failed to get collection for card",
			slog.Int64("card_id", card.ID),
			slog.String("col_id", card.ColID),
			slog.String("error", err.Error()))
		collection = nil
	}

	// Generate image URL
	groupType := utils.GetGroupType(card.Tags)
	imageURL := cms.getOptimizedImageURL(card, groupType)

	return webmodels.ConvertCardToDTO(card, collection, imageURL), nil
}

// getOptimizedImageURL generates the optimized image URL with correct format
func (cms *CardManagementService) getOptimizedImageURL(card *models.Card, groupType string) string {
	// Use the new method that supports both JPG and GIF based on animated flag
	return cms.spacesService.GetCardImageURLWithFormat(card.Name, card.ColID, card.Level, groupType, card.Animated)
}

// CreateCard creates a new card
func (cms *CardManagementService) CreateCard(ctx context.Context, req *webmodels.CardCreateRequest) (*webmodels.CardDTO, error) {
	// Validate collection exists
	collection, err := cms.repos.Collection.GetByID(ctx, req.ColID)
	if err != nil {
		return nil, fmt.Errorf("collection not found: %w", err)
	}

	// Create card model
	card := &models.Card{
		Name:      req.Name,
		Level:     req.Level,
		Animated:  req.Animated,
		ColID:     req.ColID,
		Tags:      req.Tags,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create card in database
	err = cms.repos.Card.Create(ctx, card)
	if err != nil {
		return nil, fmt.Errorf("failed to create card: %w", err)
	}

	// Handle image upload if provided
	if len(req.ImageData) > 0 {
		err = cms.uploadCardImage(ctx, card, req.ImageData, req.ImageName)
		if err != nil {
			slog.Error("Failed to upload card image",
				slog.Int64("card_id", card.ID),
				slog.String("error", err.Error()))
			// Don't fail the entire operation, just log the error
		}
	}

	// Generate image URL
	groupType := utils.GetGroupType(card.Tags)
	imageURL := cms.getOptimizedImageURL(card, groupType)

	slog.Info("Card created successfully",
		slog.Int64("card_id", card.ID),
		slog.String("name", card.Name),
		slog.String("collection", collection.Name))

	return webmodels.ConvertCardToDTO(card, collection, imageURL), nil
}

// UpdateCard updates an existing card
func (cms *CardManagementService) UpdateCard(ctx context.Context, cardID int64, req *webmodels.CardUpdateRequest) (*webmodels.CardDTO, error) {
	// Get existing card
	card, err := cms.repos.Card.GetByID(ctx, cardID)
	if err != nil {
		return nil, fmt.Errorf("card not found: %w", err)
	}

	// Update fields if provided
	if req.Name != nil {
		card.Name = *req.Name
	}
	if req.Level != nil {
		card.Level = *req.Level
	}
	if req.Animated != nil {
		card.Animated = *req.Animated
	}
	if req.ColID != nil {
		// Validate new collection exists
		_, err := cms.repos.Collection.GetByID(ctx, *req.ColID)
		if err != nil {
			return nil, fmt.Errorf("collection not found: %w", err)
		}
		card.ColID = *req.ColID
	}
	if req.Tags != nil {
		card.Tags = req.Tags
	}

	card.UpdatedAt = time.Now()

	// Update card in database
	err = cms.repos.Card.Update(ctx, card)
	if err != nil {
		return nil, fmt.Errorf("failed to update card: %w", err)
	}

	// Handle image update if provided
	if len(req.ImageData) > 0 {
		err = cms.uploadCardImage(ctx, card, req.ImageData, req.ImageName)
		if err != nil {
			slog.Error("Failed to update card image",
				slog.Int64("card_id", card.ID),
				slog.String("error", err.Error()))
			// Don't fail the entire operation, just log the error
		}
	}

	// Get collection info
	collection, err := cms.repos.Collection.GetByID(ctx, card.ColID)
	if err != nil {
		slog.Warn("Failed to get collection for card",
			slog.Int64("card_id", card.ID),
			slog.String("col_id", card.ColID),
			slog.String("error", err.Error()))
		collection = nil
	}

	// Generate image URL
	groupType := utils.GetGroupType(card.Tags)
	imageURL := cms.getOptimizedImageURL(card, groupType)

	slog.Info("Card updated successfully",
		slog.Int64("card_id", card.ID),
		slog.String("name", card.Name))

	return webmodels.ConvertCardToDTO(card, collection, imageURL), nil
}

// DeleteCard deletes a card and its associated image
func (cms *CardManagementService) DeleteCard(ctx context.Context, cardID int64) error {
	// Get card to delete
	card, err := cms.repos.Card.GetByID(ctx, cardID)
	if err != nil {
		return fmt.Errorf("card not found: %w", err)
	}

	// Delete image from storage
	err = cms.spacesService.DeleteCardImage(ctx, card.ColID, card.Name, card.Level, card.Tags)
	if err != nil {
		slog.Warn("Failed to delete card image",
			slog.Int64("card_id", card.ID),
			slog.String("error", err.Error()))
		// Continue with database deletion even if image deletion fails
	}

	// Delete card from database
	err = cms.repos.Card.Delete(ctx, cardID)
	if err != nil {
		return fmt.Errorf("failed to delete card: %w", err)
	}

	slog.Info("Card deleted successfully",
		slog.Int64("card_id", card.ID),
		slog.String("name", card.Name))

	return nil
}

// BulkOperation performs a bulk operation on multiple cards
func (cms *CardManagementService) BulkOperation(ctx context.Context, req *webmodels.CardBulkOperation) error {
	switch req.Operation {
	case "delete":
		return cms.bulkDelete(ctx, req.CardIDs)
	case "update":
		return cms.bulkUpdate(ctx, req.CardIDs, req.Updates)
	case "move":
		return cms.bulkMove(ctx, req.CardIDs, req.TargetCollection)
	default:
		return fmt.Errorf("unsupported bulk operation: %s", req.Operation)
	}
}

// uploadCardImage uploads an image for a card
func (cms *CardManagementService) uploadCardImage(ctx context.Context, card *models.Card, imageData []byte, imageName string) error {
	// Use the existing SpacesService to manage the image
	result, err := cms.spacesService.ManageCardImage(ctx, services.ImageOperationUpdate, card.ID, imageData, card)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("image upload failed: %s", result.ErrorMessage)
	}

	return nil
}

// bulkDelete deletes multiple cards
func (cms *CardManagementService) bulkDelete(ctx context.Context, cardIDs []int64) error {
	for _, cardID := range cardIDs {
		err := cms.DeleteCard(ctx, cardID)
		if err != nil {
			slog.Error("Failed to delete card in bulk operation",
				slog.Int64("card_id", cardID),
				slog.String("error", err.Error()))
			// Continue with other cards
		}
	}
	return nil
}

// bulkUpdate updates multiple cards
func (cms *CardManagementService) bulkUpdate(ctx context.Context, cardIDs []int64, updates *webmodels.CardUpdateRequest) error {
	for _, cardID := range cardIDs {
		_, err := cms.UpdateCard(ctx, cardID, updates)
		if err != nil {
			slog.Error("Failed to update card in bulk operation",
				slog.Int64("card_id", cardID),
				slog.String("error", err.Error()))
			// Continue with other cards
		}
	}
	return nil
}

// bulkMove moves multiple cards to a different collection
func (cms *CardManagementService) bulkMove(ctx context.Context, cardIDs []int64, targetCollection string) error {
	updates := &webmodels.CardUpdateRequest{
		ColID: &targetCollection,
	}
	return cms.bulkUpdate(ctx, cardIDs, updates)
}