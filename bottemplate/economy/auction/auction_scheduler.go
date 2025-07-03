package auction

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	economicUtils "github.com/disgoorg/bot-template/bottemplate/economy/utils"
)

// AuctionScheduler handles timing and cleanup for auctions
type AuctionScheduler struct {
	manager       *Manager
	cleanupTicker *time.Ticker
	shutdown      chan struct{}
	auctionTimers sync.Map // auctionID -> *time.Timer for cleanup
}

// NewAuctionScheduler creates a new auction scheduler
func NewAuctionScheduler(manager *Manager) *AuctionScheduler {
	return &AuctionScheduler{
		manager:       manager,
		cleanupTicker: time.NewTicker(economicUtils.CleanupInterval),
		shutdown:      make(chan struct{}),
		auctionTimers: sync.Map{},
	}
}

// Start begins the scheduler operations
func (s *AuctionScheduler) Start() {
	go s.startCleanupTicker()
}

// scheduleAuctionEnd schedules the completion of an auction after the specified duration
func (s *AuctionScheduler) scheduleAuctionEnd(auctionID int64, duration time.Duration) {
	timer := time.NewTimer(duration)
	
	// Store timer for cleanup
	s.auctionTimers.Store(auctionID, timer)
	
	go func() {
		defer func() {
			s.auctionTimers.Delete(auctionID)
			timer.Stop()
		}()
		
		select {
		case <-timer.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := s.manager.lifecycleManager.completeAuction(ctx, auctionID); err != nil {
				slog.Error("Failed to complete auction",
					slog.String("error", err.Error()),
					slog.Int64("auction_id", auctionID))
			}
		case <-s.shutdown:
			// Shutdown signal received, stop timer
			return
		}
	}()
}

// startCleanupTicker starts the background cleanup process
func (s *AuctionScheduler) startCleanupTicker() {
	go func() {
		defer s.cleanupTicker.Stop()
		
		for {
			select {
			case <-s.cleanupTicker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

				// Cleanup expired auctions
				if err := s.cleanupExpiredAuctions(ctx); err != nil {
					slog.Error("Failed to cleanup expired auctions",
						slog.String("error", err.Error()))
				}

				// Cleanup zero amount cards
				if err := s.manager.UserCardRepo.CleanupZeroAmountCards(ctx); err != nil {
					slog.Error("Failed to cleanup zero amount cards",
						slog.String("error", err.Error()))
				}

				cancel()
			case <-s.shutdown:
				return
			}
		}
	}()
}

// cleanupExpiredAuctions handles cleanup of expired auctions
func (s *AuctionScheduler) cleanupExpiredAuctions(ctx context.Context) error {
	expiredAuctions, err := s.manager.repo.GetExpiredAuctions(ctx)
	if err != nil {
		return err
	}

	for _, auction := range expiredAuctions {
		auctionCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		// Complete the auction with transfer and get updated data in the same transaction
		updatedAuction, err := s.manager.repo.CompleteAuctionWithTransferAndGet(auctionCtx, auction.ID)
		if err != nil {
			slog.Error("Failed to complete expired auction",
				slog.Int64("auction_id", auction.ID),
				slog.String("auction_code", auction.AuctionID),
				slog.String("error", err.Error()))
			cancel()
			continue
		}

		// Fetch card details for notification
		card, err := s.manager.cardRepo.GetByID(auctionCtx, updatedAuction.CardID)
		if err != nil {
			slog.Error("Failed to get card details for auction notification",
				slog.Int64("auction_id", auction.ID),
				slog.Int64("card_id", updatedAuction.CardID),
				slog.String("error", err.Error()))
			// Continue anyway, we don't want to fail the auction completion just because we couldn't get card details
			card = &models.Card{
				ID:    updatedAuction.CardID,
				Name:  fmt.Sprintf("Card #%d", updatedAuction.CardID),
				ColID: "unknown",
				Level: 1,
			}
		}

		// Remove from active auctions map with proper locking
		s.manager.activeMu.Lock()
		s.manager.activeAuctions.Delete(auction.ID)
		s.manager.activeMu.Unlock()

		// Notify after successful completion with updated auction data
		if err := s.manager.notifier.NotifyAuctionEnd(auctionCtx, updatedAuction, card); err != nil {
			slog.Error("Failed to send auction end notification",
				slog.String("auction_id", updatedAuction.AuctionID),
				slog.String("error", err.Error()))
		}

		cancel()
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// Shutdown gracefully stops all scheduler processes
func (s *AuctionScheduler) Shutdown() {
	// Signal shutdown to all goroutines
	close(s.shutdown)
	
	// Stop cleanup ticker
	if s.cleanupTicker != nil {
		s.cleanupTicker.Stop()
	}
	
	// Stop all auction timers
	s.auctionTimers.Range(func(key, value interface{}) bool {
		if timer, ok := value.(*time.Timer); ok {
			timer.Stop()
		}
		return true
	})
	
	slog.Info("Auction scheduler shutdown completed")
}