package auction

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/disgo/bot"
)

const (
	MinBidIncrement = 100
	MaxAuctionTime  = 24 * time.Hour
	MinAuctionTime  = 1 * time.Hour
)

type Manager struct {
	repo            repositories.AuctionRepository
	activeAuctions  sync.Map
	notifier        *AuctionNotifier
	client          bot.Client
	minBidIncrement int64
	maxAuctionTime  time.Duration
	cleanupTicker   *time.Ticker
}

func NewManager(repo repositories.AuctionRepository) *Manager {
	m := &Manager{
		repo:            repo,
		minBidIncrement: MinBidIncrement,
		maxAuctionTime:  MaxAuctionTime,
		notifier:        NewAuctionNotifier(),
		cleanupTicker:   time.NewTicker(1 * time.Minute),
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
	slog.Info("Creating new auction",
		slog.Int64("card_id", cardID),
		slog.String("seller_id", sellerID),
		slog.Int64("start_price", startPrice),
		slog.Duration("duration", duration))

	if duration > m.maxAuctionTime {
		return nil, fmt.Errorf("auction duration cannot exceed %v", m.maxAuctionTime)
	}
	if duration < MinAuctionTime {
		return nil, fmt.Errorf("auction duration must be at least %v", MinAuctionTime)
	}

	auction := &models.Auction{
		CardID:       cardID,
		SellerID:     sellerID,
		StartPrice:   startPrice,
		CurrentPrice: startPrice,
		MinIncrement: m.minBidIncrement,
		Status:       models.AuctionStatusActive,
		StartTime:    time.Now(),
		EndTime:      time.Now().Add(duration),
	}

	if err := m.repo.Create(ctx, auction); err != nil {
		slog.Error("Failed to create auction in database",
			slog.String("error", err.Error()),
			slog.Int64("card_id", cardID))
		return nil, fmt.Errorf("failed to create auction: %w", err)
	}

	slog.Info("Auction created successfully",
		slog.Int64("auction_id", auction.ID),
		slog.Time("end_time", auction.EndTime))

	m.activeAuctions.Store(auction.ID, auction)
	go m.scheduleAuctionEnd(auction.ID, duration)

	return auction, nil
}

func (m *Manager) PlaceBid(ctx context.Context, auctionID int64, bidderID string, amount int64) error {
	auction, err := m.repo.GetByID(ctx, auctionID)
	if err != nil {
		return fmt.Errorf("failed to get auction: %w", err)
	}

	if auction.Status != models.AuctionStatusActive {
		return fmt.Errorf("auction is not active")
	}

	if auction.SellerID == bidderID {
		return fmt.Errorf("seller cannot bid on their own auction")
	}

	minValidBid := auction.CurrentPrice + auction.MinIncrement
	if amount < minValidBid {
		return fmt.Errorf("bid must be at least %d (current price + minimum increment)", minValidBid)
	}

	previousBidder := auction.TopBidderID

	if err := m.repo.UpdateBid(ctx, auctionID, bidderID, amount); err != nil {
		return fmt.Errorf("failed to update bid: %w", err)
	}

	m.notifier.NotifyBid(auctionID, bidderID, amount)
	if previousBidder != "" {
		m.notifier.NotifyOutbid(auctionID, previousBidder, bidderID, amount)
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

	// Update auction status to completed
	auction.Status = models.AuctionStatusCompleted

	// Update the auction in the database
	if err := m.repo.CompleteAuction(ctx, auctionID); err != nil {
		return fmt.Errorf("failed to complete auction: %w", err)
	}

	// Remove from active auctions map
	m.activeAuctions.Delete(auctionID)

	// Notify users about auction completion
	m.notifier.NotifyEnd(auctionID, auction.TopBidderID, auction.CurrentPrice)

	slog.Info("Auction completed successfully",
		slog.Int64("auction_id", auctionID),
		slog.String("winner_id", auction.TopBidderID),
		slog.Int64("final_price", auction.CurrentPrice))

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
	_, err := m.repo.DB().NewCreateTable().
		Model((*models.Auction)(nil)).
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create auctions table: %w", err)
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

		// Get all active auctions
		auctions, err := m.repo.GetActive(ctx)
		if err != nil {
			slog.Error("Failed to get active auctions during cleanup",
				slog.String("error", err.Error()))
			cancel()
			continue
		}

		now := time.Now()
		for _, auction := range auctions {
			// Check if auction has ended
			if now.After(auction.EndTime) {
				if err := m.completeAuction(ctx, auction.ID); err != nil {
					slog.Error("Failed to complete expired auction",
						slog.Int64("auction_id", auction.ID),
						slog.String("error", err.Error()))
				} else {
					slog.Info("Successfully completed expired auction",
						slog.Int64("auction_id", auction.ID))
				}
			}
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
