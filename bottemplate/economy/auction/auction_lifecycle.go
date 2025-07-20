package auction

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	economicUtils "github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/uptrace/bun"
)

// AuctionLifecycleManager handles auction creation, completion, and cancellation
type AuctionLifecycleManager struct {
	manager *Manager
}

// NewAuctionLifecycleManager creates a new lifecycle manager
func NewAuctionLifecycleManager(manager *Manager) *AuctionLifecycleManager {
	return &AuctionLifecycleManager{
		manager: manager,
	}
}

// createAuctionInternal handles the core auction creation logic
func (l *AuctionLifecycleManager) createAuctionInternal(ctx context.Context, auctionID string, cardID int64, sellerID string, startPrice int64, duration time.Duration) (*models.Auction, error) {
	// Execute auction creation in transaction
	var auction *models.Auction
	err := l.manager.txManager.WithTransaction(ctx, economicUtils.SerializableTransactionOptions(), func(ctx context.Context, tx bun.Tx) error {
		// Remove card from seller's inventory
		if err := l.manager.txManager.RemoveCardFromInventory(ctx, tx, economicUtils.CardOperationOptions{
			UserID: sellerID,
			CardID: cardID,
			Amount: 1,
		}); err != nil {
			return fmt.Errorf("failed to remove card from inventory: %w", err)
		}

		slog.Info("Card removed from inventory for auction",
			slog.String("user_id", sellerID),
			slog.Int64("card_id", cardID))

		// Create auction within same transaction
		auction = &models.Auction{
			AuctionID:    auctionID,
			CardID:       cardID,
			SellerID:     sellerID,
			StartPrice:   startPrice,
			CurrentPrice: startPrice,
			MinIncrement: economicUtils.MinBidIncrement,
			Status:       models.AuctionStatusActive,
			StartTime:    time.Now(),
			EndTime:      time.Now().Add(duration),
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := l.manager.repo.CreateWithTx(ctx, tx, auction); err != nil {
			return fmt.Errorf("failed to create auction: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Store in active auctions map with proper locking
	l.manager.activeMu.Lock()
	l.manager.activeAuctions.Store(auction.ID, auction)
	l.manager.activeMu.Unlock()

	slog.Info("=== Auction created successfully ===",
		slog.String("auction_id", auction.AuctionID),
		slog.String("seller_id", sellerID),
		slog.Int64("card_id", cardID))

	return auction, nil
}

// completeAuction handles the completion of an auction
func (l *AuctionLifecycleManager) completeAuction(ctx context.Context, auctionID int64) error {
	auction, err := l.manager.repo.GetByID(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction: %w", err)
	}

	if auction.Status != models.AuctionStatusActive {
		return nil // Already completed or cancelled
	}

	// Fetch card details for notification
	card, err := l.manager.cardRepo.GetByID(ctx, auction.CardID)
	if err != nil {
		return fmt.Errorf("failed to get card details: %w", err)
	}

	// Start transaction
	tx, err := l.manager.repo.DB().BeginTx(ctx, nil)
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
			completionErr = l.handleNoBidsCompletion(ctx, tx, auction)
		} else {
			// Has winning bid - transfer to winner
			completionErr = l.handleWinningBidCompletion(ctx, tx, auction)
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

	// Remove from active auctions map with proper locking
	l.manager.activeMu.Lock()
	l.manager.activeAuctions.Delete(auctionID)
	l.manager.activeMu.Unlock()

	// Send notifications with card details
	if err := l.manager.notifier.NotifyAuctionEnd(ctx, auction, card); err != nil {
		slog.Error("Failed to send auction end notification",
			slog.String("auction_id", auction.AuctionID),
			slog.String("error", err.Error()))
	}

	slog.Info("Auction completed successfully",
		slog.Int64("auction_id", auctionID),
		slog.String("winner_id", auction.TopBidderID),
		slog.Int64("final_price", auction.CurrentPrice))

	// Track quest progress for auction win if there was a winner
	if auction.TopBidderID != "" && l.manager.questTrackerFunc != nil {
		go l.manager.questTrackerFunc(auction.TopBidderID)
	}

	// Track snowflakes earned from auction sale
	if auction.TopBidderID != "" && l.manager.questSnowflakesTrackerFunc != nil {
		go l.manager.questSnowflakesTrackerFunc(auction.SellerID, auction.CurrentPrice, "auction")
	}

	return nil
}

// handleNoBidsCompletion handles completion when no bids were placed
func (l *AuctionLifecycleManager) handleNoBidsCompletion(ctx context.Context, tx bun.Tx, auction *models.Auction) error {
	// Return card to seller's inventory using standardized method
	return l.manager.txManager.AddCardToInventory(ctx, tx, economicUtils.CardOperationOptions{
		UserID: auction.SellerID,
		CardID: auction.CardID,
		Amount: 1,
	})
}

// handleWinningBidCompletion handles completion when there's a winning bid
func (l *AuctionLifecycleManager) handleWinningBidCompletion(ctx context.Context, tx bun.Tx, auction *models.Auction) error {
	// Transfer card to winner's inventory using standardized method
	if err := l.manager.txManager.AddCardToInventory(ctx, tx, economicUtils.CardOperationOptions{
		UserID: auction.TopBidderID,
		CardID: auction.CardID,
		Amount: 1,
	}); err != nil {
		return fmt.Errorf("failed to transfer card to winner: %w", err)
	}

	// Transfer winning bid amount to seller using standardized method
	if err := l.manager.txManager.ValidateAndUpdateBalance(ctx, tx, economicUtils.BalanceOperationOptions{
		UserID: auction.SellerID,
		Amount: auction.CurrentPrice,
	}); err != nil {
		return fmt.Errorf("failed to transfer balance to seller: %w", err)
	}

	return nil
}

// cancelAuctionInternal handles the core auction cancellation logic
func (l *AuctionLifecycleManager) cancelAuctionInternal(ctx context.Context, auction *models.Auction) error {
	if err := l.manager.repo.CancelAuction(ctx, auction.ID); err != nil {
		return fmt.Errorf("failed to cancel auction: %w", err)
	}

	// Remove from active auctions map with proper locking
	l.manager.activeMu.Lock()
	l.manager.activeAuctions.Delete(auction.ID)
	l.manager.activeMu.Unlock()

	return nil
}
