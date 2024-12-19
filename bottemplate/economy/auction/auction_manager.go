package auction

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/disgo/bot"
	"github.com/uptrace/bun"
)

const (
	MinBidIncrement = 100
	MaxAuctionTime  = 24 * time.Hour
	MinAuctionTime  = 10 * time.Second
	IDLength        = 4
	maxRetries      = 5
)

type Manager struct {
	repo            repositories.AuctionRepository
	UserCardRepo    repositories.UserCardRepository
	activeAuctions  sync.Map
	notifier        *AuctionNotifier
	client          bot.Client
	minBidIncrement int64
	maxAuctionTime  time.Duration
	cleanupTicker   *time.Ticker
	usedIDs         sync.Map
	mu              sync.Mutex
}

func NewManager(repo repositories.AuctionRepository, userCardRepo repositories.UserCardRepository, client bot.Client) *Manager {
	if repo == nil {
		panic("auction repository cannot be nil")
	}
	if userCardRepo == nil {
		panic("user card repository cannot be nil")
	}
	if client == nil {
		panic("discord client cannot be nil")
	}

	notifier := NewAuctionNotifier(client)

	m := &Manager{
		repo:            repo,
		UserCardRepo:    userCardRepo,
		client:          client,
		notifier:        notifier,
		minBidIncrement: MinBidIncrement,
		maxAuctionTime:  MaxAuctionTime,
		cleanupTicker:   time.NewTicker(15 * time.Second),
		activeAuctions:  sync.Map{},
		usedIDs:         sync.Map{},
	}

	// Initialize table with a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := m.InitializeTable(ctx); err != nil {
		slog.Error("Failed to initialize auctions table",
			slog.String("error", err.Error()))
	}

	// Start the cleanup ticker
	go m.startCleanupTicker()

	return m
}

func (m *Manager) SetClient(client bot.Client) {
	m.client = client
	m.notifier.SetClient(client)
}

func (m *Manager) CreateAuction(ctx context.Context, cardID int64, sellerID string, startPrice int64, duration time.Duration) (*models.Auction, error) {
	slog.Info("=== Starting auction creation process ===",
		slog.Int64("card_id", cardID),
		slog.String("seller_id", sellerID),
		slog.Int64("start_price", startPrice),
		slog.Duration("duration", duration))

	// Generate auction ID first
	auctionID, err := m.generateAuctionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate auction ID: %w", err)
	}

	slog.Info("Generated auction ID", slog.String("auction_id", auctionID))

	// Start transaction
	tx, err := m.repo.DB().BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock and verify card ownership within transaction
	var userCard models.UserCard
	err = tx.NewSelect().
		Model(&userCard).
		Where("user_id = ? AND card_id = ?", sellerID, cardID).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("card not found in inventory")
		}
		return nil, fmt.Errorf("failed to get card: %w", err)
	}

	// Strict amount check
	if userCard.Amount <= 0 {
		return nil, fmt.Errorf("you don't have any copies of this card available")
	}

	slog.Info("Current card state before auction",
		slog.String("user_id", sellerID),
		slog.Int64("card_id", cardID),
		slog.Int64("current_amount", userCard.Amount))

	// Update card amount within same transaction
	result, err := tx.NewUpdate().
		Model(&userCard).
		Set("amount = amount - 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ? AND amount > 0", sellerID, cardID).
		Exec(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to update card amount: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return nil, fmt.Errorf("failed to remove card from inventory")
	}

	// Verify final amount
	var updatedCard models.UserCard
	err = tx.NewSelect().
		Model(&updatedCard).
		Where("user_id = ? AND card_id = ?", sellerID, cardID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to verify updated amount: %w", err)
	}

	slog.Info("Card amount updated for auction",
		slog.String("user_id", sellerID),
		slog.Int64("card_id", cardID),
		slog.Int64("previous_amount", userCard.Amount),
		slog.Int64("new_amount", updatedCard.Amount))

	// Create auction within same transaction
	auction := &models.Auction{
		AuctionID:    auctionID,
		CardID:       cardID,
		SellerID:     sellerID,
		StartPrice:   startPrice,
		CurrentPrice: startPrice,
		MinIncrement: MinBidIncrement,
		Status:       models.AuctionStatusActive,
		StartTime:    time.Now(),
		EndTime:      time.Now().Add(duration),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := m.repo.CreateWithTx(ctx, tx, auction); err != nil {
		return nil, fmt.Errorf("failed to create auction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Store in active auctions map
	m.activeAuctions.Store(auction.ID, auction)

	slog.Info("=== Auction created successfully ===",
		slog.String("auction_id", auction.AuctionID),
		slog.String("seller_id", sellerID),
		slog.Int64("card_id", cardID))

	return auction, nil
}

// Helper method to lock a user's card
func (m *Manager) lockUserCard(ctx context.Context, tx bun.Tx, userID string, cardID int64) error {
	slog.Debug("Attempting to lock user card",
		slog.String("user_id", userID),
		slog.Int64("card_id", cardID))

	var userCard models.UserCard
	err := tx.NewSelect().
		Model(&userCard).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			slog.Error("Card not found during lock attempt",
				slog.String("user_id", userID),
				slog.Int64("card_id", cardID))
			return fmt.Errorf("card not found in inventory")
		}
		slog.Error("Failed to lock user card",
			slog.String("error", err.Error()),
			slog.String("user_id", userID),
			slog.Int64("card_id", cardID))
		return fmt.Errorf("failed to lock user card: %w", err)
	}

	if userCard.Amount <= 0 {
		slog.Error("Card amount is 0 or negative during lock",
			slog.String("user_id", userID),
			slog.Int64("card_id", cardID),
			slog.Int64("amount", userCard.Amount))
		return fmt.Errorf("insufficient card amount")
	}

	slog.Debug("Successfully locked user card",
		slog.String("user_id", userID),
		slog.Int64("card_id", cardID))
	return nil
}

// Helper method to remove card from inventory
func (m *Manager) removeCardFromInventory(ctx context.Context, tx bun.Tx, userID string, cardID int64) error {
	slog.Debug("Attempting to remove card from inventory",
		slog.String("user_id", userID),
		slog.Int64("card_id", cardID))

	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount - 1").
		Where("user_id = ? AND card_id = ? AND amount > 0", userID, cardID).
		Exec(ctx)

	if err != nil {
		slog.Error("Failed to update card amount",
			slog.String("error", err.Error()),
			slog.String("user_id", userID),
			slog.Int64("card_id", cardID))
		return fmt.Errorf("failed to update card amount: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		slog.Error("Failed to get rows affected",
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		slog.Error("No rows affected when removing card",
			slog.String("user_id", userID),
			slog.Int64("card_id", cardID))
		return fmt.Errorf("failed to remove card from inventory - card not found or amount <= 0")
	}

	slog.Debug("Successfully removed card from inventory",
		slog.String("user_id", userID),
		slog.Int64("card_id", cardID))
	return nil
}

func (m *Manager) PlaceBid(ctx context.Context, auctionID int64, bidderID string, amount int64) error {
	tx, err := m.repo.DB().BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock and get auction details
	auction := new(models.Auction)
	err = tx.NewSelect().
		Model(auction).
		Where("id = ?", auctionID).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		return fmt.Errorf("failed to get auction: %w", err)
	}

	if auction.Status != models.AuctionStatusActive {
		return fmt.Errorf("auction is not active")
	}

	if auction.SellerID == bidderID {
		return fmt.Errorf("seller cannot bid on their own auction")
	}

	// Check if user has already bid and wasn't outbid
	if auction.TopBidderID == bidderID {
		return fmt.Errorf("you are already the highest bidder")
	}

	minValidBid := auction.CurrentPrice + auction.MinIncrement
	if amount < minValidBid {
		return fmt.Errorf("bid must be at least %d (current price + minimum increment)", minValidBid)
	}

	// Lock and verify bidder's balance
	var bidder models.User
	err = tx.NewSelect().
		Model(&bidder).
		Where("id = ?", bidderID).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		return fmt.Errorf("failed to get bidder info: %w", err)
	}

	if bidder.Balance < amount {
		return fmt.Errorf("insufficient balance (%d required, has %d)", amount, bidder.Balance)
	}

	// Deduct bid amount from bidder immediately
	_, err = tx.NewUpdate().
		Model((*models.User)(nil)).
		Set("balance = balance - ?", amount).
		Where("id = ?", bidderID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to deduct bid amount: %w", err)
	}

	// If there was a previous bidder, refund their bid
	if auction.TopBidderID != "" {
		_, err = tx.NewUpdate().
			Model((*models.User)(nil)).
			Set("balance = balance + ?", auction.CurrentPrice).
			Where("id = ?", auction.TopBidderID).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to refund previous bidder: %w", err)
		}
	}

	// Update auction with new bid
	_, err = tx.NewUpdate().
		Model((*models.Auction)(nil)).
		Set("top_bidder_id = ?", bidderID).
		Set("current_price = ?", amount).
		Set("previous_bidder_id = ?", auction.TopBidderID).
		Set("previous_bid_amount = ?", auction.CurrentPrice).
		Set("last_bid_time = ?", time.Now()).
		Set("bid_count = bid_count + 1").
		Where("id = ?", auctionID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update auction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit bid transaction: %w", err)
	}

	// Send notifications after successful commit
	m.notifier.NotifyBid(auctionID, bidderID, amount)
	if auction.TopBidderID != "" {
		m.notifier.NotifyOutbid(auctionID, auction.TopBidderID, bidderID, amount)
	}

	return nil
}

func (m *Manager) CancelAuction(ctx context.Context, auctionID int64, requesterID string) error {
	auction, err := m.repo.GetByID(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction: %w", err)
	}

	if auction.SellerID != requesterID {
		return fmt.Errorf("only the seller can cancel the auction")
	}

	if auction.Status != models.AuctionStatusActive {
		return fmt.Errorf("cannot cancel completed or already cancelled auction")
	}

	if err := m.repo.CancelAuction(ctx, auctionID); err != nil {
		return fmt.Errorf("failed to cancel auction: %w", err)
	}

	m.activeAuctions.Delete(auctionID)
	return nil
}

func (m *Manager) scheduleAuctionEnd(auctionID int64, duration time.Duration) {
	timer := time.NewTimer(duration)
	<-timer.C

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := m.completeAuction(ctx, auctionID); err != nil {
		slog.Error("Failed to complete auction",
			slog.String("error", err.Error()),
			slog.Int64("auction_id", auctionID))
	}
}

func (m *Manager) completeAuction(ctx context.Context, auctionID int64) error {
	auction, err := m.repo.GetByID(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction: %w", err)
	}

	if auction.Status != models.AuctionStatusActive {
		return nil // Already completed or cancelled
	}

	// Start transaction
	tx, err := m.repo.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Handle the auction completion with retries
	maxRetries := 3
	var completionErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if auction.TopBidderID == "" {
			// No bids - return card to seller
			completionErr = m.handleNoBidsCompletion(ctx, tx, auction)
		} else {
			// Has winning bid - transfer to winner
			completionErr = m.handleWinningBidCompletion(ctx, tx, auction)
		}

		if completionErr == nil {
			break
		}

		if attempt == maxRetries-1 {
			slog.Error("Failed to complete auction after max retries",
				slog.Int64("auction_id", auctionID),
				slog.String("error", completionErr.Error()))
			return fmt.Errorf("failed to complete auction after %d attempts: %w", maxRetries, completionErr)
		}

		// Exponential backoff
		time.Sleep(time.Duration(math.Pow(2, float64(attempt))) * time.Second)
	}

	// Add verification after completion
	if auction.TopBidderID != "" {
		// Verify card transfer
		var winnerCard models.UserCard
		err = tx.NewSelect().
			Model(&winnerCard).
			Where("user_id = ? AND card_id = ?", auction.TopBidderID, auction.CardID).
			Scan(ctx)

		if err != nil || winnerCard.Amount <= 0 {
			slog.Error("Card transfer verification failed",
				slog.String("winner_id", auction.TopBidderID),
				slog.Int64("card_id", auction.CardID))
			return fmt.Errorf("card transfer verification failed")
		}

		// Verify balance transfer
		var seller, winner models.User
		err = tx.NewSelect().
			Model(&seller).
			Where("id = ?", auction.SellerID).
			Scan(ctx)

		if err != nil {
			return fmt.Errorf("failed to verify seller balance: %w", err)
		}

		err = tx.NewSelect().
			Model(&winner).
			Where("id = ?", auction.TopBidderID).
			Scan(ctx)

		if err != nil {
			return fmt.Errorf("failed to verify winner balance: %w", err)
		}
	}

	// Update auction status
	_, err = tx.NewUpdate().
		Model((*models.Auction)(nil)).
		Set("status = ?", models.AuctionStatusCompleted).
		Where("id = ?", auctionID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update auction status: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit auction completion: %w", err)
	}

	// Remove from active auctions map
	m.activeAuctions.Delete(auctionID)

	// Send notifications
	if err := m.notifier.NotifyAuctionEnd(ctx, auction); err != nil {
		slog.Error("Failed to send auction end notification",
			slog.String("auction_id", auction.AuctionID),
			slog.String("error", err.Error()))
	}

	slog.Info("Auction completed successfully",
		slog.Int64("auction_id", auctionID),
		slog.String("winner_id", auction.TopBidderID),
		slog.Int64("final_price", auction.CurrentPrice))

	return nil
}

func (m *Manager) handleNoBidsCompletion(ctx context.Context, tx bun.Tx, auction *models.Auction) error {
	// Return card to seller's inventory
	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + 1").
		Where("user_id = ? AND card_id = ?", auction.SellerID, auction.CardID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to return card to seller: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// If no rows were affected, the seller doesn't have this card entry yet
		_, err = tx.NewInsert().
			Model(&models.UserCard{
				UserID: auction.SellerID,
				CardID: auction.CardID,
				Amount: 1,
			}).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to create new card entry for seller: %w", err)
		}
	}

	return nil
}

func (m *Manager) handleWinningBidCompletion(ctx context.Context, tx bun.Tx, auction *models.Auction) error {
	// Transfer card to winner's inventory with UPSERT
	_, err := tx.NewInsert().
		Model(&models.UserCard{
			UserID:    auction.TopBidderID,
			CardID:    auction.CardID,
			Amount:    1,
			Obtained:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}).
		On("CONFLICT (user_id, card_id) DO UPDATE").
		Set("amount = user_cards.amount + 1").
		Set("updated_at = ?", time.Now()).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to transfer card to winner: %w", err)
	}

	// Transfer winning bid amount to seller
	_, err = tx.NewUpdate().
		Model((*models.User)(nil)).
		Set("balance = balance + ?", auction.CurrentPrice).
		Where("id = ?", auction.SellerID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to transfer balance to seller: %w", err)
	}

	return nil
}

func (m *Manager) GetActiveAuctions(ctx context.Context) ([]*models.Auction, error) {
	slog.Info("Starting to fetch active auctions")

	// Get auctions from database
	auctions, err := m.repo.GetActive(ctx)
	if err != nil {
		slog.Error("Failed to get active auctions from database",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get active auctions: %w", err)
	}

	// Log detailed auction information
	for _, auction := range auctions {
		slog.Info("Found auction",
			slog.Int64("id", auction.ID),
			slog.String("status", string(auction.Status)),
			slog.Time("end_time", auction.EndTime),
			slog.Int64("current_price", auction.CurrentPrice))
	}

	// Log in-memory auction count
	var memoryAuctionCount int
	m.activeAuctions.Range(func(key, value interface{}) bool {
		memoryAuctionCount++
		return true
	})

	slog.Info("Auction retrieval complete",
		slog.Int("database_auctions", len(auctions)),
		slog.Int("memory_auctions", memoryAuctionCount))

	return auctions, nil
}

func (m *Manager) RecoverActiveAuctions(ctx context.Context) error {
	auctions, err := m.repo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active auctions: %w", err)
	}

	for _, auction := range auctions {
		m.activeAuctions.Store(auction.ID, auction)
		remainingTime := time.Until(auction.EndTime)
		if remainingTime > 0 {
			go m.scheduleAuctionEnd(auction.ID, remainingTime)
		} else {
			go func(aid int64) {
				if err := m.completeAuction(ctx, aid); err != nil {
					// Log error and attempt recovery
				}
			}(auction.ID)
		}
	}

	return nil
}

// Add this function to initialize the table
func (m *Manager) InitializeTable(ctx context.Context) error {
	db := m.repo.DB()

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

func (m *Manager) GetAllActiveAuctions(ctx context.Context) ([]*models.Auction, error) {
	auctions, err := m.repo.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active auctions: %w", err)
	}

	// Filter out expired auctions
	var activeAuctions []*models.Auction
	now := time.Now()
	for _, auction := range auctions {
		if auction.Status == models.AuctionStatusActive && now.Before(auction.EndTime) {
			activeAuctions = append(activeAuctions, auction)
		}
	}

	return activeAuctions, nil
}

func (m *Manager) startCleanupTicker() {
	for range m.cleanupTicker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Cleanup expired auctions
		if err := m.cleanupExpiredAuctions(ctx); err != nil {
			slog.Error("Failed to cleanup expired auctions",
				slog.String("error", err.Error()))
		}

		// Cleanup zero amount cards
		if err := m.UserCardRepo.CleanupZeroAmountCards(ctx); err != nil {
			slog.Error("Failed to cleanup zero amount cards",
				slog.String("error", err.Error()))
		}

		cancel()
	}
}

// Add cleanup when shutting down
func (m *Manager) Shutdown() {
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}
}

// Generate a unique 4-character auction ID
func (m *Manager) generateAuctionID() (string, error) {
	for i := 0; i < maxRetries; i++ {
		// Generate 3 random bytes (24 bits)
		bytes := make([]byte, 3)
		if _, err := rand.Read(bytes); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}

		// Encode using base32 (5 bits per character)
		encoded := base32.StdEncoding.EncodeToString(bytes)

		// Take first 4 characters and convert to uppercase
		id := strings.ToUpper(encoded[:IDLength])

		// Check if ID is already in use
		if _, exists := m.usedIDs.LoadOrStore(id, true); !exists {
			return id, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique auction ID after %d attempts", maxRetries)
}

func (m *Manager) GetAuctionByAuctionID(ctx context.Context, auctionID string) (*models.Auction, error) {
	auction, err := m.repo.GetByAuctionID(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("auction not found: %w", err)
	}
	return auction, nil
}

func (m *Manager) GetByID(ctx context.Context, id int64) (*models.Auction, error) {
	auction, err := m.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("auction not found: %w", err)
	}
	return auction, nil
}

func (m *Manager) verifyCardOwnership(ctx context.Context, userID string, cardID int64) error {
	if m.UserCardRepo == nil {
		slog.Error("UserCardRepo is nil")
		return fmt.Errorf("internal error: user card repository not initialized")
	}

	userCard, err := m.UserCardRepo.GetByUserIDAndCardID(ctx, userID, cardID)
	if err != nil {
		slog.Error("Failed to verify card ownership",
			slog.String("error", err.Error()),
			slog.String("user_id", userID),
			slog.Int64("card_id", cardID))
		return fmt.Errorf("failed to verify card ownership: %w", err)
	}

	if userCard == nil {
		slog.Debug("Card not found in user's inventory",
			slog.String("user_id", userID),
			slog.Int64("card_id", cardID))
		return fmt.Errorf("you do not own this card")
	}

	if userCard.Amount <= 0 {
		slog.Debug("Card amount is 0",
			slog.String("user_id", userID),
			slog.Int64("card_id", cardID),
			slog.Int64("amount", userCard.Amount))
		return fmt.Errorf("you don't have any copies of this card available")
	}

	return nil
}

func (m *Manager) cleanupExpiredAuctions(ctx context.Context) error {
	// Get expired auctions
	expiredAuctions, err := m.repo.GetExpiredAuctions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get expired auctions: %w", err)
	}

	for _, auction := range expiredAuctions {
		// Create a new context with timeout for each auction
		auctionCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		err := m.repo.CompleteAuctionWithTransfer(auctionCtx, auction.ID)
		if err != nil {
			slog.Error("Failed to complete expired auction",
				slog.Int64("auction_id", auction.ID),
				slog.String("error", err.Error()))
			cancel()
			continue
		}

		// Notify after successful completion
		if err := m.notifier.NotifyAuctionEnd(auctionCtx, auction); err != nil {
			slog.Error("Failed to send auction end notification",
				slog.String("auction_id", auction.AuctionID),
				slog.String("error", err.Error()))
		}

		cancel()

		// Add a small delay between processing auctions
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}
