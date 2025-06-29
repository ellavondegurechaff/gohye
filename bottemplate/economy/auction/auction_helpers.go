package auction

import (
	"context"
	"fmt"
	"math"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	economicUtils "github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/disgoorg/bot-template/bottemplate/utils"
)

// AuctionHelpers provides utility functions for auction operations
type AuctionHelpers struct {
	manager *Manager
}

// NewAuctionHelpers creates a new helpers instance
func NewAuctionHelpers(manager *Manager) *AuctionHelpers {
	return &AuctionHelpers{
		manager: manager,
	}
}


// getMarketPrice calculates the market price for a card based on auction history
func (h *AuctionHelpers) getMarketPrice(ctx context.Context, cardID int64) (int64, error) {
	// Get recent auction history for this card
	auctions, err := h.manager.repo.GetRecentCompletedAuctions(ctx, cardID, 5)
	if err != nil {
		return 0, fmt.Errorf("failed to get auction history: %w", err)
	}

	if len(auctions) == 0 {
		// If no auction history, use base price calculation
		card, err := h.manager.cardRepo.GetByID(ctx, cardID)
		if err != nil {
			return economicUtils.MinPrice, nil
		}

		basePrice := economicUtils.InitialBasePrice * int64(math.Pow(economicUtils.LevelMultiplier, float64(card.Level-1)))
		return basePrice, nil
	}

	// Calculate average price from recent auctions
	var totalPrice int64
	for _, a := range auctions {
		totalPrice += a.CurrentPrice
	}
	avgPrice := totalPrice / int64(len(auctions))

	// Ensure price is within bounds
	if avgPrice < economicUtils.MinPrice {
		return economicUtils.MinPrice, nil
	}
	if avgPrice > economicUtils.MaxPrice {
		return economicUtils.MaxPrice, nil
	}

	return avgPrice, nil
}

// getUserCardByName finds a user's card by name using enhanced search functionality
func (h *AuctionHelpers) getUserCardByName(ctx context.Context, userID string, cardName string) (*models.UserCard, error) {
	// Try direct query first (optimized approach)
	if card, err := h.manager.cardRepo.GetByQuery(ctx, cardName); err == nil {
		// Check if user owns this card
		if userCard, err := h.manager.UserCardRepo.GetUserCard(ctx, userID, card.ID); err == nil && userCard.Amount > 0 {
			return userCard, nil
		}
	}

	// Fallback to comprehensive search within user's cards
	userCards, err := h.manager.UserCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user cards: %w", err)
	}

	// Filter for cards with amount > 0 and create lookup map
	cardIDs := make([]int64, 0, len(userCards))
	userCardMap := make(map[int64]*models.UserCard)
	for _, uc := range userCards {
		if uc.Amount > 0 {
			cardIDs = append(cardIDs, uc.CardID)
			userCardMap[uc.CardID] = uc
		}
	}

	if len(cardIDs) == 0 {
		return nil, fmt.Errorf("no cards available for auction")
	}

	// Get card details for user's owned cards only
	cards, err := h.manager.cardRepo.GetByIDs(ctx, cardIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get card details: %w", err)
	}

	// Use enhanced search filters with user-specific filtering
	filters := utils.ParseSearchQuery(cardName)
	filters.SortBy = utils.SortByLevel
	filters.SortDesc = true

	searchResults := utils.WeightedSearchWithMulti(cards, filters, userCardMap)

	if len(searchResults) == 0 {
		return nil, fmt.Errorf("no matching owned cards found")
	}

	// Return the UserCard for the best match
	bestCard := searchResults[0]
	if userCard, exists := userCardMap[bestCard.ID]; exists {
		return userCard, nil
	}

	return nil, fmt.Errorf("internal error: card found but user ownership not matched")
}


// initializeTable creates the necessary database tables for auctions
func (h *AuctionHelpers) initializeTable(ctx context.Context) error {
	db := h.manager.repo.DB()

	// Create auctions table
	_, err := db.NewCreateTable().
		Model((*models.Auction)(nil)).
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create auctions table: %w", err)
	}

	// Create auction_bids table
	_, err = db.NewCreateTable().
		Model((*models.AuctionBid)(nil)).
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create auction_bids table: %w", err)
	}

	// Create indexes for auction_bids table
	_, err = db.NewCreateIndex().
		Model((*models.AuctionBid)(nil)).
		Index("idx_auction_bids_auction_id").
		Column("auction_id").
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create auction_id index: %w", err)
	}

	_, err = db.NewCreateIndex().
		Model((*models.AuctionBid)(nil)).
		Index("idx_auction_bids_bidder_id").
		Column("bidder_id").
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create bidder_id index: %w", err)
	}

	return nil
}