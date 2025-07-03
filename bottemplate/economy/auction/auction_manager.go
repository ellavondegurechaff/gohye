package auction

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	economicUtils "github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/disgoorg/disgo/bot"
	"github.com/uptrace/bun"
)

const (
	// Use centralized constants from utils for shared values
	// Keep auction-specific constants local
	IDLength   = economicUtils.AuctionIDLength
	maxRetries = economicUtils.MaxRetries
)

type Manager struct {
	repo            repositories.AuctionRepository
	UserCardRepo    repositories.UserCardRepository
	cardRepo        repositories.CardRepository
	activeAuctions  sync.Map
	notifier        *AuctionNotifier
	client          bot.Client
	minBidIncrement int64
	maxAuctionTime  time.Duration
	// Separate mutexes for different operations to prevent deadlocks
	activeMu        sync.RWMutex // Protects activeAuctions map
	txManager       *economicUtils.EconomicTransactionManager
	
	// Component managers for decomposed functionality
	idGenerator       *AuctionIDGenerator
	lifecycleManager  *AuctionLifecycleManager
	scheduler         *AuctionScheduler
	helpers           *AuctionHelpers
	
	// Quest tracking
	questTrackerFunc func(userID string)
	questSnowflakesTrackerFunc func(userID string, amount int64, source string)
}

func NewManager(repo repositories.AuctionRepository, userCardRepo repositories.UserCardRepository, cardRepo repositories.CardRepository, client bot.Client) *Manager {
	if repo == nil {
		panic("auction repository cannot be nil")
	}
	if userCardRepo == nil {
		panic("user card repository cannot be nil")
	}
	if cardRepo == nil {
		panic("card repository cannot be nil")
	}
	if client == nil {
		panic("discord client cannot be nil")
	}

	notifier := NewAuctionNotifier(client)

	m := &Manager{
		repo:            repo,
		UserCardRepo:    userCardRepo,
		cardRepo:        cardRepo,
		client:          client,
		notifier:        notifier,
		minBidIncrement: economicUtils.MinBidIncrement,
		maxAuctionTime:  economicUtils.MaxAuctionTime,
		activeAuctions:  sync.Map{},
		txManager:       economicUtils.NewEconomicTransactionManager(repo.DB()),
	}

	// Initialize component managers
	m.idGenerator = NewAuctionIDGenerator(repo, cardRepo)
	m.lifecycleManager = NewAuctionLifecycleManager(m)
	m.scheduler = NewAuctionScheduler(m)
	m.helpers = NewAuctionHelpers(m)

	// Initialize table with a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := m.InitializeTable(ctx); err != nil {
		slog.Error("Failed to initialize auctions table",
			slog.String("error", err.Error()))
	}

	// Start the scheduler
	m.scheduler.Start()

	return m
}

func (m *Manager) SetClient(client bot.Client) {
	m.client = client
	m.notifier.SetClient(client)
}

func (m *Manager) CreateAuction(ctx context.Context, cardID int64, sellerID string, startPrice int64, duration time.Duration) (*models.Auction, error) {
	// Log auction creation start only at debug level to reduce noise
	if slog.Default().Enabled(nil, slog.LevelDebug) {
		slog.Debug("Starting auction creation process",
			slog.Int64("card_id", cardID),
			slog.String("seller_id", sellerID),
			slog.Int64("start_price", startPrice),
			slog.Duration("duration", duration))
	}

	// Generate auction ID first
	auctionID, err := m.idGenerator.generateAuctionID(ctx, cardID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate auction ID: %w", err)
	}

	// Log auction ID generation only at debug level
	if slog.Default().Enabled(nil, slog.LevelDebug) {
		slog.Debug("Generated auction ID", slog.String("auction_id", auctionID))
	}

	// Create auction using lifecycle manager
	auction, err := m.lifecycleManager.createAuctionInternal(ctx, auctionID, cardID, sellerID, startPrice, duration)
	if err != nil {
		return nil, err
	}

	return auction, nil
}

func (m *Manager) PlaceBid(ctx context.Context, auctionID int64, bidderID string, amount int64) error {
	return m.txManager.WithTransaction(ctx, economicUtils.SerializableTransactionOptions(), func(ctx context.Context, tx bun.Tx) error {
		// Lock and get auction details
		auction := new(models.Auction)
		err := tx.NewSelect().
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

		// Validate and deduct bid amount from bidder
		if err := m.txManager.ValidateAndUpdateBalance(ctx, tx, economicUtils.BalanceOperationOptions{
			UserID: bidderID,
			Amount: -amount,
		}); err != nil {
			return fmt.Errorf("failed to deduct bid amount: %w", err)
		}

		// If there was a previous bidder, refund their bid
		if auction.TopBidderID != "" {
			if err := m.txManager.ValidateAndUpdateBalance(ctx, tx, economicUtils.BalanceOperationOptions{
				UserID: auction.TopBidderID,
				Amount: auction.CurrentPrice,
			}); err != nil {
				return fmt.Errorf("failed to refund previous bidder: %w", err)
			}
		}

		now := time.Now()
		timeUntilEnd := auction.EndTime.Sub(now)

		// If bid is placed in last 10 seconds, extend the auction
		if timeUntilEnd <= economicUtils.AntiSnipeTime {
			auction.EndTime = now.Add(economicUtils.AntiSnipeTime)

			// Reschedule auction end
			go m.scheduler.scheduleAuctionEnd(auctionID, economicUtils.AntiSnipeTime)
		}

		// Update auction with new bid and potentially extended end time
		_, err = tx.NewUpdate().
			Model((*models.Auction)(nil)).
			Set("top_bidder_id = ?", bidderID).
			Set("current_price = ?", amount).
			Set("previous_bidder_id = ?", auction.TopBidderID).
			Set("previous_bid_amount = ?", auction.CurrentPrice).
			Set("last_bid_time = ?", now).
			Set("end_time = ?", auction.EndTime).
			Set("bid_count = bid_count + 1").
			Where("id = ?", auctionID).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to update auction: %w", err)
		}

		// Send notifications after successful commit (moved outside transaction)
		go func() {
			m.notifier.NotifyBid(auctionID, bidderID, amount)
			if auction.TopBidderID != "" {
				m.notifier.NotifyOutbid(auctionID, auction.TopBidderID, bidderID, amount)
			}
		}()

		return nil
	})
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

	return m.lifecycleManager.cancelAuctionInternal(ctx, auction)
}

func (m *Manager) GetActiveAuctions(ctx context.Context) ([]*models.Auction, error) {
	// Get auctions from database
	auctions, err := m.repo.GetActive(ctx)
	if err != nil {
		slog.Error("Failed to get active auctions from database",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get active auctions: %w", err)
	}

	// Log auction summary instead of individual auction details
	slog.Info("Fetched active auctions", slog.Int("count", len(auctions)))
	
	// Log detailed auction information only at debug level
	if slog.Default().Enabled(nil, slog.LevelDebug) {
		for _, auction := range auctions {
			slog.Debug("Found auction",
				slog.Int64("id", auction.ID),
				slog.String("status", string(auction.Status)),
				slog.Time("end_time", auction.EndTime),
				slog.Int64("current_price", auction.CurrentPrice))
		}
	}

	// Log in-memory auction count with read lock
	var memoryAuctionCount int
	m.activeMu.RLock()
	m.activeAuctions.Range(func(key, value interface{}) bool {
		memoryAuctionCount++
		return true
	})
	m.activeMu.RUnlock()

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

	// Recover active auctions with proper locking
	m.activeMu.Lock()
	for _, auction := range auctions {
		m.activeAuctions.Store(auction.ID, auction)
		remainingTime := time.Until(auction.EndTime)
		if remainingTime > 0 {
			m.scheduler.scheduleAuctionEnd(auction.ID, remainingTime)
		} else {
			// Schedule immediate completion for expired auctions
			go func(aid int64) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				
				if err := m.lifecycleManager.completeAuction(ctx, aid); err != nil {
					slog.Error("Failed to complete recovered expired auction",
						slog.Int64("auction_id", aid),
						slog.String("error", err.Error()))
				}
			}(auction.ID)
		}
	}
	m.activeMu.Unlock()

	return nil
}

// Add this function to initialize the table
func (m *Manager) InitializeTable(ctx context.Context) error {
	return m.helpers.initializeTable(ctx)
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

// SetQuestTracker sets the quest tracking function for auction wins
func (m *Manager) SetQuestTracker(trackerFunc func(userID string)) {
	m.questTrackerFunc = trackerFunc
}

// SetQuestSnowflakesTracker sets the quest tracking function for snowflakes earned
func (m *Manager) SetQuestSnowflakesTracker(trackerFunc func(userID string, amount int64, source string)) {
	m.questSnowflakesTrackerFunc = trackerFunc
}

// Shutdown gracefully stops all auction manager processes
func (m *Manager) Shutdown() {
	m.scheduler.Shutdown()
	slog.Info("Auction manager shutdown completed")
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

func (m *Manager) GetMarketPrice(ctx context.Context, cardID int64) (int64, error) {
	return m.helpers.getMarketPrice(ctx, cardID)
}

func (m *Manager) GetUserCardByName(ctx context.Context, userID string, cardName string) (*models.UserCard, error) {
	return m.helpers.getUserCardByName(ctx, userID, cardName)
}