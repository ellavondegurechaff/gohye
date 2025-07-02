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
	GetByCollectionID(ctx context.Context, colID string) ([]*models.Card, error)
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
	
	// GetUserCardsWithDetailsAndFiltersWithUser fetches user cards with card details, applies filtering with user context for advanced filters like -new
	GetUserCardsWithDetailsAndFiltersWithUser(ctx context.Context, userID string, query string, user *models.User) ([]*models.UserCard, []*models.Card, utils.SearchFilters, error)
	
	// GetMissingCards returns cards the user doesn't own, with optional filtering
	GetMissingCards(ctx context.Context, userID string, query string) ([]*models.Card, error)
	
	// GetCardDifferences returns card differences between two users
	GetCardDifferences(ctx context.Context, userID, targetUserID string, mode string) ([]*models.Card, error)
	
	// SearchCardsInCollection searches within a specific collection of cards
	SearchCardsInCollection(ctx context.Context, cards []*models.Card, filters utils.SearchFilters) []*models.Card
	
	// BuildCardMappings creates optimized lookup maps for card operations
	BuildCardMappings(userCards []*models.UserCard, cards []*models.Card) (map[int64]*models.UserCard, map[int64]*models.Card)
}

// CollectionServiceInterface defines the interface for collection operations
type CollectionServiceInterface interface {
	// CalculateProgress calculates completion progress for a user's collection
	CalculateProgress(ctx context.Context, userID string, collectionID string) (*models.CollectionProgress, error)
	
	// GetCollectionLeaderboard returns top collectors for a collection
	GetCollectionLeaderboard(ctx context.Context, collectionID string, limit int) ([]*models.CollectionProgressResult, error)
	
	// CheckCompletion checks if a user has completed a collection
	CheckCompletion(ctx context.Context, userID string, collectionID string) (bool, error)
	
	// CalculateResetRequirements calculates cards needed for collection reset
	CalculateResetRequirements(ctx context.Context, userID string, collectionID string) (*models.ResetRequirements, error)
	
	// IsFragmentCollection checks if collection is fragment type
	IsFragmentCollection(ctx context.Context, collectionID string) (bool, error)
	
	// GetRandomSampleCard returns a random card from the specified collection
	GetRandomSampleCard(ctx context.Context, collectionID string) (*models.Card, error)
}