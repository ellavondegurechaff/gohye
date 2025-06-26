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

// getUserCardByName finds a user's card by name using search functionality
func (h *AuctionHelpers) getUserCardByName(ctx context.Context, userID string, cardName string) (*models.UserCard, error) {
	// First get all user's cards
	userCards, err := h.manager.UserCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user cards: %w", err)
	}

	// Get all cards for searching
	cards, err := h.manager.cardRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cards: %w", err)
	}

	// Create a map of owned cards for quick lookup
	ownedCards := make(map[int64]*models.UserCard)
	for _, uc := range userCards {
		if uc.Amount > 0 {
			ownedCards[uc.CardID] = uc
		}
	}

	// Use weighted search on all cards
	filters := utils.ParseSearchQuery(cardName)
	filters.SortBy = utils.SortByLevel
	filters.SortDesc = true

	searchResults := utils.WeightedSearch(cards, filters)

	// Find the first matching card that the user owns
	for _, result := range searchResults {
		if userCard, ok := ownedCards[result.ID]; ok {
			return userCard, nil
		}
	}

	return nil, fmt.Errorf("no matching owned cards found")
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