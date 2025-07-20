package pricing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/uptrace/bun"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	statsQueryTimeout = 10 * time.Second
	maxBatchSize      = 25 // Reduced batch size for faster processing
	parallelQueries   = 4  // Number of parallel stat queries
)

// MarketStats represents market statistics for a card over time
type MarketStats struct {
	MinPrice24h        int64   `bun:"minprice24h"`
	MaxPrice24h        int64   `bun:"maxprice24h"`
	AvgPrice24h        float64 `bun:"avgprice24h"`
	UniqueOwners       int     `bun:"uniqueowners"`
	PriceChangePercent float64 `bun:"pricechangepercent"`
}

// MarketAnalyzer handles market statistics and analysis
type MarketAnalyzer struct {
	db                  *database.DB
	config              PricingConfig
	inactivityThreshold time.Duration
}

// NewMarketAnalyzer creates a new market analyzer
func NewMarketAnalyzer(db *database.DB, config PricingConfig, inactivityThreshold time.Duration) *MarketAnalyzer {
	return &MarketAnalyzer{
		db:                  db,
		config:              config,
		inactivityThreshold: inactivityThreshold,
	}
}

// GetActiveCards returns all cards that have active owners
func (ma *MarketAnalyzer) GetActiveCards(ctx context.Context) ([]int64, error) {
	var cardIDs []int64
	err := ma.db.BunDB().NewSelect().
		TableExpr("cards c"). // Add explicit table reference
		ColumnExpr("c.id").   // Use table alias
		Join("JOIN user_cards uc ON c.id = uc.card_id").
		Join("JOIN users u ON uc.user_id = u.discord_id").
		Where("u.last_daily > ?", time.Now().Add(-ma.inactivityThreshold)).
		GroupExpr("c.id").
		Having("COUNT(DISTINCT uc.user_id) >= ?", utils.MinimumActiveOwners).
		Order("c.id ASC").
		Scan(ctx, &cardIDs)

	if err != nil {
		return nil, fmt.Errorf("failed to get active cards: %w", err)
	}

	return cardIDs, nil
}

// GetActiveOwnersCount returns the number of active owners for a card
func (ma *MarketAnalyzer) GetActiveOwnersCount(ctx context.Context, cardID int64) (int, error) {
	count, err := ma.db.BunDB().NewSelect().
		TableExpr("user_cards uc").
		Join("JOIN users u ON uc.user_id = u.discord_id").
		Where("uc.card_id = ?", cardID).
		Where("u.last_daily > ?", time.Now().Add(-ma.inactivityThreshold)).
		Count(ctx)

	return int(count), err
}

// GetBatchActiveOwnersCount returns active owner counts for multiple cards
func (ma *MarketAnalyzer) GetBatchActiveOwnersCount(ctx context.Context, cardIDs []int64) (map[int64]int, error) {
	type result struct {
		CardID      int64 `bun:"card_id"`
		ActiveCount int   `bun:"active_count"`
	}

	var results []result
	err := ma.db.BunDB().NewSelect().
		TableExpr("user_cards uc").
		ColumnExpr("uc.card_id, COUNT(DISTINCT uc.user_id) as active_count").
		Join("JOIN users u ON uc.user_id = u.discord_id").
		Where("uc.card_id IN (?)", bun.In(cardIDs)).
		Where("u.last_daily > ?", time.Now().Add(-ma.inactivityThreshold)).
		GroupExpr("uc.card_id").
		Scan(ctx, &results)

	if err != nil {
		return nil, err
	}

	counts := make(map[int64]int, len(cardIDs))
	for _, r := range results {
		counts[r.CardID] = r.ActiveCount
	}

	return counts, nil
}

// GetCardStats retrieves comprehensive statistics for multiple cards
func (ma *MarketAnalyzer) GetCardStats(ctx context.Context, cardIDs []int64) (map[int64]CardStats, error) {
	statsMap := make(map[int64]CardStats, len(cardIDs))

	// Split into smaller batches
	batches := make([][]int64, 0)
	for i := 0; i < len(cardIDs); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(cardIDs) {
			end = len(cardIDs)
		}
		batches = append(batches, cardIDs[i:end])
	}

	// Process batches in parallel with error group
	g, gctx := errgroup.WithContext(ctx)
	resultChan := make(chan map[int64]CardStats, len(batches))

	// Semaphore to limit concurrent queries
	sem := semaphore.NewWeighted(int64(parallelQueries))

	for _, batch := range batches {
		batch := batch // Capture for goroutine
		g.Go(func() error {
			if err := sem.Acquire(gctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			// Create timeout context for this batch
			queryCtx, cancel := context.WithTimeout(gctx, statsQueryTimeout)
			defer cancel()

			batchStats, err := ma.processStatsBatch(queryCtx, batch)
			if err != nil {
				return fmt.Errorf("batch stats error: %w", err)
			}

			select {
			case resultChan <- batchStats:
			case <-gctx.Done():
				return gctx.Err()
			}
			return nil
		})
	}

	// Close results channel when all goroutines complete
	go func() {
		g.Wait()
		close(resultChan)
	}()

	// Collect results
	for batchResult := range resultChan {
		for cardID, stats := range batchResult {
			statsMap[cardID] = stats
		}
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("error processing stats batches: %w", err)
	}

	return statsMap, nil
}

// processStatsBatch processes statistics for a batch of cards
func (ma *MarketAnalyzer) processStatsBatch(ctx context.Context, cardIDs []int64) (map[int64]CardStats, error) {
	var stats []CardStats
	err := ma.db.BunDB().NewSelect().
		TableExpr("cards c").
		ColumnExpr("c.id as card_id").
		ColumnExpr("COALESCE(COUNT(uc.id), 0) as total_copies").
		ColumnExpr("COALESCE(COUNT(DISTINCT uc.user_id), 0) as unique_owners").
		ColumnExpr(`COALESCE(COUNT(DISTINCT CASE 
			WHEN u.last_daily > ? THEN uc.user_id 
			ELSE NULL 
			END), 0) as active_owners`, time.Now().Add(-ma.inactivityThreshold)).
		ColumnExpr(`COALESCE(COUNT(CASE 
			WHEN u.last_daily > ? THEN 1 
			ELSE NULL 
			END), 0) as active_copies`, time.Now().Add(-ma.inactivityThreshold)).
		ColumnExpr(`COALESCE((
			SELECT MAX(copies)
			FROM (
				SELECT COUNT(*) as copies
				FROM user_cards uc2
				WHERE uc2.card_id = c.id
				GROUP BY uc2.user_id
			) subq
		), 0) as max_copies_per_user`).
		ColumnExpr(`COALESCE((
			SELECT AVG(copies)::numeric
			FROM (
				SELECT COUNT(*) as copies
				FROM user_cards uc2
				WHERE uc2.card_id = c.id
				GROUP BY uc2.user_id
			) subq
		), 0) as avg_copies_per_user`).
		Join("LEFT JOIN user_cards uc ON c.id = uc.card_id").
		Join("LEFT JOIN users u ON uc.user_id = u.discord_id").
		Where("c.id IN (?)", bun.In(cardIDs)).
		GroupExpr("c.id").
		Scan(ctx, &stats)

	if err != nil {
		return nil, fmt.Errorf("error fetching card stats: %w", err)
	}

	// Convert to map
	statsMap := make(map[int64]CardStats, len(stats))
	for _, stat := range stats {
		statsMap[stat.CardID] = stat
	}

	return statsMap, nil
}

// GetMarketStats retrieves market statistics for a card over the last 24 hours
func (ma *MarketAnalyzer) GetMarketStats(ctx context.Context, cardID int64, currentPrice int64) (*MarketStats, error) {
	var stats MarketStats
	err := ma.db.BunDB().NewSelect().
		Model((*models.CardMarketHistory)(nil)).
		ColumnExpr("MIN(price) as minprice24h").
		ColumnExpr("MAX(price) as maxprice24h").
		ColumnExpr("AVG(price) as avgprice24h").
		ColumnExpr("COUNT(DISTINCT active_owners) as uniqueowners").
		ColumnExpr("MAX(price_change_percentage) as pricechangepercent").
		Where("card_id = ?", cardID).
		Where("timestamp > NOW() - INTERVAL '24 hours'").
		GroupExpr("card_id").
		Scan(ctx, &stats)

	if err != nil {
		// Check if error is due to missing table - provide fallback data
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "relation") {
			// Table doesn't exist, use current price as fallback
			stats = MarketStats{
				MinPrice24h:        currentPrice,
				MaxPrice24h:        currentPrice,
				AvgPrice24h:        float64(currentPrice),
				UniqueOwners:       0,
				PriceChangePercent: 0,
			}
			return &stats, nil
		}
		return nil, fmt.Errorf("failed to fetch market stats: %w", err)
	}

	// If no records found, initialize with zero values
	if stats.MinPrice24h == 0 && stats.MaxPrice24h == 0 {
		stats = MarketStats{
			MinPrice24h:        currentPrice,
			MaxPrice24h:        currentPrice,
			AvgPrice24h:        float64(currentPrice),
			UniqueOwners:       0,
			PriceChangePercent: 0,
		}
	}

	return &stats, nil
}

// FetchCardStats is a helper method to get detailed statistics for a single card
func (ma *MarketAnalyzer) FetchCardStats(ctx context.Context, cardID int64, result interface{}) error {
	return ma.db.BunDB().NewSelect().
		TableExpr("user_cards uc").
		ColumnExpr("COUNT(*) as total_copies").
		ColumnExpr("COUNT(DISTINCT uc.user_id) as unique_owners").
		ColumnExpr(`COUNT(DISTINCT CASE 
		WHEN u.last_daily > ? THEN uc.user_id 
		ELSE NULL 
		END) as active_owners`, time.Now().Add(-ma.inactivityThreshold)).
		ColumnExpr(`COUNT(CASE 
		WHEN u.last_daily > ? THEN 1 
		ELSE NULL 
		END) as active_copies`, time.Now().Add(-ma.inactivityThreshold)).
		ColumnExpr(`COALESCE((
		SELECT COUNT(*) 
		FROM user_cards uc2 
		WHERE uc2.card_id = ? 
		GROUP BY uc2.user_id 
		ORDER BY COUNT(*) DESC 
		LIMIT 1
	), 0) as max_copies`, cardID).
		ColumnExpr(`COALESCE((
		SELECT AVG(copy_count)
		FROM (
			SELECT COUNT(*) as copy_count 
			FROM user_cards uc2 
			WHERE uc2.card_id = ? 
			GROUP BY uc2.user_id
		) t
	), 0) as avg_copies`, cardID).
		Join("LEFT JOIN users u ON uc.user_id = u.discord_id").
		Where("uc.card_id = ?", cardID).
		Scan(ctx, result)
}
