package interfaces

import (
	"context"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
)

// CardRepositoryInterface defines the interface for card repository operations
type CardRepositoryInterface interface {
	GetByID(ctx context.Context, id int64) (*models.Card, error)
	GetAll(ctx context.Context) ([]*models.Card, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*models.Card, error)
}

// UserCardRepositoryInterface defines the interface for user card repository operations  
type UserCardRepositoryInterface interface {
	GetAllByUserID(ctx context.Context, userID string) ([]*models.UserCard, error)
}

// SpacesServiceInterface defines the interface for spaces service operations
type SpacesServiceInterface interface {
	GetSpacesConfig() utils.SpacesConfig
}

// CardOperationsServiceInterface defines the interface for common card operations
type CardOperationsServiceInterface interface {
	// GetUserCardsWithDetails fetches user cards with card details and applies filtering
	GetUserCardsWithDetails(ctx context.Context, userID string, query string) ([]*models.UserCard, []*models.Card, error)
	
	// GetMissingCards returns cards the user doesn't own, with optional filtering
	GetMissingCards(ctx context.Context, userID string, query string) ([]*models.Card, error)
	
	// GetCardDifferences returns card differences between two users
	GetCardDifferences(ctx context.Context, userID, targetUserID string, mode string) ([]*models.Card, error)
	
	// SearchCardsInCollection searches within a specific collection of cards
	SearchCardsInCollection(ctx context.Context, cards []*models.Card, filters utils.SearchFilters) []*models.Card
	
	// BuildCardMappings creates optimized lookup maps for card operations
	BuildCardMappings(userCards []*models.UserCard, cards []*models.Card) (map[int64]*models.UserCard, map[int64]*models.Card)
}