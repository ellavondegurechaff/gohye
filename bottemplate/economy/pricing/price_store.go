package pricing

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy/utils"
	lru "github.com/hashicorp/golang-lru"
	"github.com/uptrace/bun"
)

const (
	cacheSize             = 10000            // Limit cache size
	InitialPricingTimeout = 5 * time.Minute  // Longer timeout for initial pricing
	BatchQueryTimeout     = 30 * time.Second // Timeout for batch queries
)

// PricePoint represents a point in time price data
type PricePoint struct {
	Price        int64
	ActiveOwners int
	Timestamp    time.Time
	PriceChange  float64
}

// cachedPrice represents a cached price entry
type cachedPrice struct {
	price     int64
	timestamp time.Time
}

// PriceStore handles price persistence, caching, and history management
type PriceStore struct {
	db                  *database.DB
	cache               *lru.Cache
	config              PricingConfig
	statsRepo           repositories.EconomyStatsRepository
	cacheExpiry         time.Duration
	inactivityThreshold time.Duration
}

// NewPriceStore creates a new price store
func NewPriceStore(db *database.DB, config PricingConfig, statsRepo repositories.EconomyStatsRepository, cacheExpiry time.Duration) *PriceStore {
	cache, _ := lru.New(cacheSize)
	return &PriceStore{
		db:                  db,
		cache:               cache,
		config:              config,
		statsRepo:           statsRepo,
		cacheExpiry:         cacheExpiry,
		inactivityThreshold: utils.UserInactivityThreshold,
	}
}

// GetLastPrice retrieves the most recent price for a card
func (ps *PriceStore) GetLastPrice(ctx context.Context, cardID int64) (int64, error) {
	var history models.CardMarketHistory
	err := ps.db.BunDB().NewSelect().
		Model(&history).
		Where("card_id = ?", cardID).
		Order("timestamp DESC").
		Limit(1).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return history.Price, nil
}

// GetLatestPrice retrieves the latest price for a card, calculating if none exists
func (ps *PriceStore) GetLatestPrice(ctx context.Context, cardID int64, calculator *Calculator, analyzer *MarketAnalyzer) (int64, error) {
	var history models.CardMarketHistory
	err := ps.db.BunDB().NewSelect().
		Model(&history).
		Where("card_id = ?", cardID).
		Order("timestamp DESC").
		Limit(1).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			// Need to calculate new price - get card and stats
			var card models.Card
			err = ps.db.BunDB().NewSelect().
				Model(&card).
				Where("id = ?", cardID).
				Scan(ctx)
			if err != nil {
				return 0, fmt.Errorf("failed to get card: %w", err)
			}

			// Get card stats
			statsMap, err := analyzer.GetCardStats(ctx, []int64{cardID})
			if err != nil {
				return 0, fmt.Errorf("failed to get card stats: %w", err)
			}

			stats, ok := statsMap[cardID]
			if !ok {
				// Use default stats
				stats = CardStats{
					CardID:           cardID,
					TotalCopies:      1,
					UniqueOwners:     1,
					ActiveOwners:     1,
					ActiveCopies:     1,
					MaxCopiesPerUser: 1,
					AvgCopiesPerUser: 1.0,
				}
			}

			factors := calculator.CalculatePriceFactors(stats)
			return calculator.CalculateFinalPrice(card, factors), nil
		}
		return 0, fmt.Errorf("failed to fetch latest price: %w", err)
	}

	return history.Price, nil
}

// UpdateCardPrice updates the price for a card with caching
func (ps *PriceStore) UpdateCardPrice(ctx context.Context, cardID int64, price int64, activeOwners int) error {
	cacheKey := fmt.Sprintf("price:%d", cardID)
	if cached, ok := ps.cache.Get(cacheKey); ok {
		if c, ok := cached.(cachedPrice); ok {
			if time.Since(c.timestamp) < ps.cacheExpiry {
				return nil // Skip update if cache is still valid
			}
		}
	}

	history := &models.CardMarketHistory{
		CardID:       cardID,
		Price:        price,
		ActiveOwners: activeOwners,
		Timestamp:    time.Now(),
	}

	_, err := ps.db.BunDB().NewInsert().
		Model(history).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to store price history: %w", err)
	}

	ps.cache.Add(cacheKey, cachedPrice{
		price:     price,
		timestamp: time.Now(),
	})

	return nil
}

// StoreHistories stores multiple price histories in a batch
func (ps *PriceStore) StoreHistories(ctx context.Context, histories []*models.CardMarketHistory) error {
	if len(histories) == 0 {
		return nil
	}

	_, err := ps.db.BunDB().NewInsert().
		Model(&histories).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to insert histories: %w", err)
	}

	return nil
}

// GetPriceHistory retrieves price history for a card over specified days
func (ps *PriceStore) GetPriceHistory(ctx context.Context, cardID int64, days int, calculator *Calculator, analyzer *MarketAnalyzer) ([]PricePoint, error) {
	var histories []models.CardMarketHistory
	err := ps.db.BunDB().NewSelect().
		Model(&histories).
		Where("card_id = ?", cardID).
		Where("timestamp > NOW() - INTERVAL '? DAY'", days).
		Order("timestamp DESC").
		Limit(24). // Limit to 24 points for a day
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			// Return empty slice if no history found
			return []PricePoint{}, nil
		}
		return nil, fmt.Errorf("failed to fetch price history: %w", err)
	}

	// Get current price if no history exists
	if len(histories) == 0 {
		currentPrice, err := ps.GetLatestPrice(ctx, cardID, calculator, analyzer)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate current price: %w", err)
		}

		return []PricePoint{{
			Price:        currentPrice,
			ActiveOwners: 0,
			Timestamp:    time.Now(),
			PriceChange:  0,
		}}, nil
	}

	// Convert history to price points
	points := make([]PricePoint, len(histories))
	for i, h := range histories {
		points[i] = PricePoint{
			Price:        h.Price,
			ActiveOwners: h.ActiveOwners,
			Timestamp:    h.Timestamp,
			PriceChange:  h.PriceChangePercent,
		}
	}

	return points, nil
}

// InitializeCardPrices initializes the price system and performs initial pricing if needed
func (ps *PriceStore) InitializeCardPrices(ctx context.Context, calculator *Calculator, analyzer *MarketAnalyzer) error {
	// Create or update the table schema
	_, err := ps.db.BunDB().NewCreateTable().
		Model((*models.CardMarketHistory)(nil)).
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create market history table: %w", err)
	}

	// Create necessary indexes
	_, err = ps.db.BunDB().NewCreateIndex().
		Model((*models.CardMarketHistory)(nil)).
		Index("idx_card_market_history_card_id_timestamp").
		Column("card_id", "timestamp").
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return err
	}

	// Check if we need to initialize prices
	count, err := ps.db.BunDB().NewSelect().
		Model((*models.CardMarketHistory)(nil)).
		Count(ctx)

	if err != nil {
		return fmt.Errorf("failed to check market history: %w", err)
	}

	// Only initialize if no price history exists
	if count == 0 {
		return ps.performInitialPricing(ctx, calculator, analyzer)
	}

	return nil
}

// performInitialPricing performs the initial pricing for all cards
func (ps *PriceStore) performInitialPricing(ctx context.Context, calculator *Calculator, analyzer *MarketAnalyzer) error {
	// Create a longer timeout context for initial pricing
	ctx, cancel := context.WithTimeout(ctx, InitialPricingTimeout)
	defer cancel()

	// Optimize the query by breaking it into smaller parts
	var cardIDs []int64
	err := ps.db.BunDB().NewSelect().
		Model((*models.Card)(nil)).
		Column("id").
		Order("id ASC").
		Scan(ctx, &cardIDs)

	if err != nil {
		return fmt.Errorf("failed to fetch card IDs: %w", err)
	}

	// Process cards in smaller batches
	batchSize := 50
	for i := 0; i < len(cardIDs); i += batchSize {
		end := i + batchSize
		if end > len(cardIDs) {
			end = len(cardIDs)
		}

		batchIDs := cardIDs[i:end]

		// Get stats for this batch
		var cards []struct {
			models.Card
			TotalCopies  int     `bun:"total_copies"`
			UniqueOwners int     `bun:"unique_owners"`
			ActiveOwners int     `bun:"active_owners"`
			ActiveCopies int     `bun:"active_copies"`
			MaxCopies    int     `bun:"max_copies"`
			AvgCopies    float64 `bun:"avg_copies"`
		}

		err := ps.db.BunDB().NewSelect().
			TableExpr("cards c").
			ColumnExpr("c.*").
			ColumnExpr("COALESCE(COUNT(uc.id), 0) as total_copies").
			ColumnExpr("COALESCE(COUNT(DISTINCT uc.user_id), 0) as unique_owners").
			ColumnExpr(`COALESCE(COUNT(DISTINCT CASE 
			WHEN u.last_daily > ? THEN uc.user_id 
			ELSE NULL 
			END), 0) as active_owners`, time.Now().Add(-ps.inactivityThreshold)).
			ColumnExpr(`COALESCE(COUNT(CASE 
			WHEN u.last_daily > ? THEN 1 
			ELSE NULL 
			END), 0) as active_copies`, time.Now().Add(-ps.inactivityThreshold)).
			ColumnExpr(`COALESCE((
			SELECT MAX(user_copies)
			FROM (
				SELECT COUNT(*) as user_copies
				FROM user_cards uc2
				WHERE uc2.card_id = c.id
				GROUP BY uc2.user_id
			) subq
		), 0) as max_copies`).
			ColumnExpr(`COALESCE((
			SELECT ROUND(AVG(user_copies)::numeric, 2)
			FROM (
				SELECT COUNT(*) as user_copies
				FROM user_cards uc2
				WHERE uc2.card_id = c.id
				GROUP BY uc2.user_id
			) subq
		), 0) as avg_copies`).
			Join("LEFT JOIN user_cards uc ON c.id = uc.card_id").
			Join("LEFT JOIN users u ON uc.user_id = u.discord_id").
			Where("c.id IN (?)", bun.In(batchIDs)).
			GroupExpr("c.id").
			Scan(ctx, &cards)

		if err != nil {
			return fmt.Errorf("failed to fetch batch stats: %w", err)
		}

		// Process this batch
		histories := make([]*models.CardMarketHistory, len(cards))
		for j, card := range cards {
			stats := CardStats{
				CardID:           card.ID,
				TotalCopies:      card.TotalCopies,
				UniqueOwners:     card.UniqueOwners,
				ActiveOwners:     card.ActiveOwners,
				ActiveCopies:     card.ActiveCopies,
				MaxCopiesPerUser: card.MaxCopies,
				AvgCopiesPerUser: card.AvgCopies,
			}

			// Calculate initial price based on existing data
			initialPrice := calculator.CalculateInitialPrice(card.Card, stats)
			factors := calculator.CalculatePriceFactors(stats)

			histories[j] = &models.CardMarketHistory{
				CardID:             card.ID,
				Price:              initialPrice,
				IsActive:           stats.ActiveOwners >= utils.MinimumActiveOwners,
				ActiveOwners:       stats.ActiveOwners,
				TotalCopies:        stats.TotalCopies,
				ActiveCopies:       stats.ActiveCopies,
				UniqueOwners:       stats.UniqueOwners,
				MaxCopiesPerUser:   stats.MaxCopiesPerUser,
				AvgCopiesPerUser:   stats.AvgCopiesPerUser,
				ScarcityFactor:     factors.ScarcityFactor,
				DistributionFactor: factors.DistributionFactor,
				HoardingFactor:     factors.HoardingFactor,
				ActivityFactor:     factors.ActivityFactor,
				PriceReason: fmt.Sprintf("Initial price based on Level %d, %d copies, %d owners, %d active",
					card.Level, stats.TotalCopies, stats.UniqueOwners, stats.ActiveOwners),
				Timestamp:          time.Now().UTC(),
				PriceChangePercent: 0,
			}
		}

		// Insert batch
		_, err = ps.db.BunDB().NewInsert().
			Model(&histories).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to insert batch: %w", err)
		}
	}

	return nil
}

// ValidateCardPrice validates a price against historical data
func (ps *PriceStore) ValidateCardPrice(ctx context.Context, cardID int64, price int64) error {
	// Get historical prices
	var history []models.CardMarketHistory
	err := ps.db.BunDB().NewSelect().
		Model(&history).
		Where("card_id = ?", cardID).
		Where("timestamp > NOW() - INTERVAL '24 hours'").
		Order("timestamp DESC").
		Limit(24).
		Scan(ctx)

	if err != nil {
		return fmt.Errorf("failed to get price history: %w", err)
	}

	if len(history) > 0 {
		// Calculate average and standard deviation
		var sum, sqSum float64
		for _, h := range history {
			sum += float64(h.Price)
			sqSum += float64(h.Price) * float64(h.Price)
		}
		avg := sum / float64(len(history))
		stdDev := math.Sqrt((sqSum / float64(len(history))) - (avg * avg))

		// Check if new price is within 3 standard deviations
		if math.Abs(float64(price)-avg) > stdDev*3 {
			fmt.Printf("[MARKET] WARNING: Card %d price %d significantly deviates from average %.2f (±%.2f)",
				cardID, price, avg, stdDev)
		}
	}

	return nil
}

// GetDB returns the database instance
func (ps *PriceStore) GetDB() *database.DB {
	return ps.db
}

// UpdateMarketStats updates market statistics after price calculations
func (ps *PriceStore) UpdateMarketStats(ctx context.Context, cardID int64, newPrice int64, oldPrice int64) error {
	stats, err := ps.statsRepo.GetLatest(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			// Initialize new stats if none exist
			stats = &models.EconomyStats{
				Timestamp:          time.Now(),
				PriceVolatility:    0,
				MarketVolume:       0,
				AverageDailyTrades: 0,
			}
			return ps.statsRepo.Create(ctx, stats)
		}
		return fmt.Errorf("failed to get latest stats: %w", err)
	}

	// Calculate price volatility
	if oldPrice > 0 {
		priceChange := math.Abs(float64(newPrice-oldPrice)) / float64(oldPrice)
		stats.PriceVolatility = (stats.PriceVolatility + priceChange) / 2
	}

	// Update market volume
	stats.MarketVolume += newPrice

	// Update average daily trades
	stats.AverageDailyTrades++

	return ps.statsRepo.Create(ctx, stats)
}
