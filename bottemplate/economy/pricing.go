package economy

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	lru "github.com/hashicorp/golang-lru"
	"github.com/uptrace/bun"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	batchSize    = 50
	numWorkers   = 3
	queryTimeout = 60 * time.Second

	MinimumActiveOwners = 1
	MinimumTotalCopies  = 1

	maxConcurrentBatches = 5
	workerPoolSize       = 10
	cacheSize            = 10000 // Limit cache size

	MinQueryBatchSize = 100
	MaxRetries        = 3
	MinPrice          = 500     // Minimum price floor
	MaxPrice          = 1000000 // Maximum price ceiling

	InitialBasePrice  = 1000 // Base price for level 1 cards
	LevelMultiplier   = 1.5  // Price multiplier per level
	ScarcityBaseValue = 100  // Base value for scarcity calculation
	ActivityBaseValue = 50   // Base value for activity calculation

	InitialPricingTimeout = 5 * time.Minute  // Longer timeout for initial pricing
	BatchQueryTimeout     = 30 * time.Second // Timeout for batch queries

	statsQueryTimeout = 10 * time.Second
	maxBatchSize      = 25 // Reduced batch size for faster processing
	parallelQueries   = 4  // Number of parallel stat queries
)

type PriceCalculator struct {
	db        *database.DB
	cache     *lru.Cache
	config    PricingConfig
	sem       *semaphore.Weighted
	logger    *log.Logger
	statsRepo repositories.EconomyStatsRepository
}

type cachedPrice struct {
	price     int64
	timestamp time.Time
}

// PricingConfig holds all configuration for price calculation
type PricingConfig struct {
	BasePrice           int64         // Base price for level 1 cards
	LevelMultiplier     float64       // Price multiplier per level
	ScarcityWeight      float64       // Weight for scarcity impact
	ActivityWeight      float64       // Weight for activity impact
	MinPrice            int64         // Absolute minimum price
	MaxPrice            int64         // Absolute maximum price
	MinActiveOwners     int           // Minimum active owners for price calculation
	MinTotalCopies      int           // Minimum total copies for price calculation
	BaseMultiplier      float64       // Base price multiplier
	ScarcityImpact      float64       // Price reduction per copy
	DistributionImpact  float64       // Impact for distribution
	HoardingThreshold   float64       // Supply threshold for hoarding
	HoardingImpact      float64       // Price increase for hoarding
	ActivityImpact      float64       // Impact for activity
	OwnershipImpact     float64       // Impact per owner
	RarityMultiplier    float64       // Increase per rarity level
	PriceUpdateInterval time.Duration // How often to update prices
	InactivityThreshold time.Duration // Time before considering owner inactive
	CacheExpiration     time.Duration // How long to cache prices
}

type PricePoint struct {
	Price        int64
	ActiveOwners int
	Timestamp    time.Time
	PriceChange  float64
}

type MarketStats struct {
	MinPrice24h        int64   `bun:"minprice24h"`
	MaxPrice24h        int64   `bun:"maxprice24h"`
	AvgPrice24h        float64 `bun:"avgprice24h"`
	UniqueOwners       int     `bun:"uniqueowners"`
	PriceChangePercent float64 `bun:"pricechangepercent"`
}

type CardStats struct {
	CardID           int64   `bun:"card_id"`
	TotalCopies      int     `bun:"total_copies"`
	UniqueOwners     int     `bun:"unique_owners"`
	ActiveOwners     int     `bun:"active_owners"`
	ActiveCopies     int     `bun:"active_copies"`
	MaxCopiesPerUser int     `bun:"max_copies_per_user"`
	AvgCopiesPerUser float64 `bun:"avg_copies_per_user"`
	PriceReason      string  `bun:"price_reason"`
}

type PriceFactors struct {
	ScarcityFactor     float64
	DistributionFactor float64
	HoardingFactor     float64
	ActivityFactor     float64
	Reason             string
}

type ProcessingStats struct {
	StartTime      time.Time
	CardCount      int
	BatchCount     int
	ProcessedCards int32 // Using int32 for atomic operations
	Errors         int32 // Using int32 for atomic operations
}

func NewPriceCalculator(db *database.DB, config PricingConfig, statsRepo repositories.EconomyStatsRepository) *PriceCalculator {
	cache, _ := lru.New(cacheSize)
	return &PriceCalculator{
		db:        db,
		cache:     cache,
		config:    config,
		sem:       semaphore.NewWeighted(maxConcurrentBatches),
		logger:    log.Default(),
		statsRepo: statsRepo,
	}
}

func (pc *PriceCalculator) CalculateCardPrice(ctx context.Context, cardID int64) (int64, error) {
	// Get old price for comparison
	oldPrice, err := pc.GetLastPrice(ctx, cardID)
	if err != nil {
		pc.logger.Printf("Warning: Could not get old price: %v", err)
	}

	// Get card details
	var card models.Card
	err = pc.db.BunDB().NewSelect().
		Model(&card).
		Where("id = ?", cardID).
		Scan(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get card: %w", err)
	}

	log.Printf("[%s] [DEBUG] [MARKET] Found card %d with level %d",
		time.Now().Format("15:04:05"), cardID, card.Level)

	// Get active owners count
	activeOwners, err := pc.getActiveOwnersCount(ctx, cardID)
	if err != nil {
		return 0, err
	}

	log.Printf("[%s] [DEBUG] [MARKET] Card %d has %d active owners",
		time.Now().Format("15:04:05"), cardID, activeOwners)

	// Calculate price components
	basePrice := pc.calculateBasePrice(card)
	ownershipModifier := pc.calculateOwnershipModifier(activeOwners)
	rarityModifier := pc.calculateRarityModifier(card.Level)

	// Calculate final price
	finalPrice := int64(float64(basePrice) * ownershipModifier * rarityModifier)
	finalPrice = pc.applyPriceLimits(finalPrice)

	log.Printf("[%s] [DEBUG] [MARKET] Card %d price calculation: base=%d, ownership=%.2f, rarity=%.2f, final=%d",
		time.Now().Format("15:04:05"), cardID, basePrice, ownershipModifier, rarityModifier, finalPrice)

	// Update market stats
	if err := pc.updateMarketStats(ctx, cardID, finalPrice, oldPrice); err != nil {
		pc.logger.Printf("Warning: Failed to update market stats: %v", err)
	}

	return finalPrice, nil
}

func (pc *PriceCalculator) getActiveOwnersCount(ctx context.Context, cardID int64) (int, error) {
	count, err := pc.db.BunDB().NewSelect().
		TableExpr("user_cards uc").
		Join("JOIN users u ON uc.user_id = u.discord_id").
		Where("uc.card_id = ?", cardID).
		Where("u.last_daily > ?", time.Now().Add(-pc.config.InactivityThreshold)).
		Count(ctx)

	return int(count), err
}

func (pc *PriceCalculator) calculateBasePrice(card models.Card) int64 {
	// Base price calculation with level scaling
	basePrice := InitialBasePrice * int64(math.Pow(LevelMultiplier, float64(card.Level-1)))
	return max(basePrice, MinPrice)
}

func (pc *PriceCalculator) calculateOwnershipModifier(activeOwners int) float64 {
	if activeOwners < MinimumActiveOwners {
		return 1.0
	}

	// Inverse logarithmic scaling for scarcity
	scarcityMod := 2.0 - math.Log10(float64(activeOwners)+1)/2.0
	return math.Max(1.0, scarcityMod)
}

func (pc *PriceCalculator) calculateRarityModifier(level int) float64 {
	// Exponential scaling for rarity
	return math.Pow(1.5, float64(level-1))
}

func (pc *PriceCalculator) applyPriceLimits(price int64) int64 {
	if price < MinPrice {
		return MinPrice
	}
	if price > MaxPrice {
		return MaxPrice
	}
	return price
}

func (pc *PriceCalculator) GetLastPrice(ctx context.Context, cardID int64) (int64, error) {
	var history models.CardMarketHistory
	err := pc.db.BunDB().NewSelect().
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

func (pc *PriceCalculator) UpdateCardPrice(ctx context.Context, cardID int64) error {
	cacheKey := fmt.Sprintf("price:%d", cardID)
	if cached, ok := pc.cache.Get(cacheKey); ok {
		if c, ok := cached.(cachedPrice); ok {
			if time.Since(c.timestamp) < pc.config.CacheExpiration {
				return nil // Skip update if cache is still valid
			}
		}
	}

	var card models.Card
	err := pc.db.BunDB().NewSelect().
		Model(&card).
		Where("id = ?", cardID).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to get card: %w", err)
	}

	activeOwners, err := pc.getActiveOwnersCount(ctx, cardID)
	if err != nil {
		return fmt.Errorf("failed to get active owners count: %w", err)
	}

	price, err := pc.CalculateCardPrice(ctx, cardID)
	if err != nil {
		return err
	}

	history := &models.CardMarketHistory{
		CardID:       cardID,
		Price:        price,
		ActiveOwners: activeOwners,
		Timestamp:    time.Now(),
	}

	_, err = pc.db.BunDB().NewInsert().
		Model(history).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to store price history: %w", err)
	}

	pc.cache.Add(cacheKey, cachedPrice{
		price:     price,
		timestamp: time.Now(),
	})

	return nil
}

func (pc *PriceCalculator) GetPriceHistory(ctx context.Context, cardID int64, days int) ([]PricePoint, error) {
	var histories []models.CardMarketHistory
	err := pc.db.BunDB().NewSelect().
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
		currentPrice, err := pc.CalculateCardPrice(ctx, cardID)
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

// Add helper method to get latest price
func (pc *PriceCalculator) GetLatestPrice(ctx context.Context, cardID int64) (int64, error) {
	var history models.CardMarketHistory
	err := pc.db.BunDB().NewSelect().
		Model(&history).
		Where("card_id = ?", cardID).
		Order("timestamp DESC").
		Limit(1).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			// Calculate new price if no history exists
			return pc.CalculateCardPrice(ctx, cardID)
		}
		return 0, fmt.Errorf("failed to fetch latest price: %w", err)
	}

	return history.Price, nil
}

func (pc *PriceCalculator) StartPriceUpdateJob(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(pc.config.PriceUpdateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := pc.UpdateAllPrices(ctx); err != nil {
					log.Printf("Error updating prices: %v", err)
				}
			}
		}
	}()
}

// Add this to prevent repeated runs
var isUpdating atomic.Bool

func (pc *PriceCalculator) UpdateAllPrices(ctx context.Context) error {
	// Add mutex to prevent concurrent updates
	var updateMutex sync.Mutex
	updateMutex.Lock()
	defer updateMutex.Unlock()

	// Get active cards with timeout
	cardIDs, err := pc.GetActiveCards(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active cards: %w", err)
	}

	// Initialize processing stats
	stats := &ProcessingStats{
		StartTime:      time.Now(),
		CardCount:      len(cardIDs),
		BatchCount:     (len(cardIDs) + batchSize - 1) / batchSize,
		ProcessedCards: 0,
		Errors:         0,
	}

	// Process in batches with proper error handling
	batchSize := 50
	for i := 0; i < len(cardIDs); i += batchSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			end := i + batchSize
			if end > len(cardIDs) {
				end = len(cardIDs)
			}

			batchNum := i/batchSize + 1
			if err := pc.processBatch(ctx, cardIDs[i:end], stats, batchNum); err != nil {
				log.Printf("Failed to process batch: %v", err)
				atomic.AddInt32(&stats.Errors, 1)
				continue // Continue with next batch instead of failing completely
			}
		}
	}

	// Log final statistics
	log.Printf("Price update completed: Total cards: %d, Processed cards: %d, Errors: %d, Time taken: %v",
		stats.CardCount, int(atomic.LoadInt32(&stats.ProcessedCards)), int(atomic.LoadInt32(&stats.Errors)), time.Since(stats.StartTime))

	return nil
}

func (pc *PriceCalculator) getCardStats(ctx context.Context, cardIDs []int64) (map[int64]CardStats, error) {
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

			batchStats, err := pc.processStatsBatch(queryCtx, batch)
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

func (pc *PriceCalculator) processStatsBatch(ctx context.Context, cardIDs []int64) (map[int64]CardStats, error) {
	var stats []CardStats
	err := pc.db.BunDB().NewSelect().
		TableExpr("cards c").
		ColumnExpr("c.id as card_id").
		ColumnExpr("COALESCE(COUNT(uc.id), 0) as total_copies").
		ColumnExpr("COALESCE(COUNT(DISTINCT uc.user_id), 0) as unique_owners").
		ColumnExpr(`COALESCE(COUNT(DISTINCT CASE 
				WHEN u.last_daily > ? THEN uc.user_id 
				ELSE NULL 
				END), 0) as active_owners`, time.Now().Add(-pc.config.InactivityThreshold)).
		ColumnExpr(`COALESCE(COUNT(CASE 
				WHEN u.last_daily > ? THEN 1 
				ELSE NULL 
				END), 0) as active_copies`, time.Now().Add(-pc.config.InactivityThreshold)).
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

func (pc *PriceCalculator) calculatePriceFactors(stats CardStats) PriceFactors {
	// Ensure minimum values to prevent division by zero
	safeActiveOwners := math.Max(1.0, float64(stats.ActiveOwners))
	safeUniqueOwners := math.Max(1.0, float64(stats.UniqueOwners))
	safeActiveCopies := math.Max(1.0, float64(stats.ActiveCopies))

	// 1. Scarcity Factor - prevent division by zero
	scarcityFactor := math.Max(0.5, 1.0-(safeActiveCopies*pc.config.ScarcityImpact))

	// 2. Distribution Factor - prevent NaN
	distributionRatio := safeActiveCopies / safeActiveOwners
	distributionFactor := math.Max(0.5, 1.0-(math.Min(distributionRatio, 10.0)-1.0)*pc.config.DistributionImpact)

	// 3. Hoarding Impact - prevent Inf
	hoardingFactor := 1.0
	if stats.MaxCopiesPerUser > 0 {
		hoardingThreshold := math.Max(1.0, safeActiveCopies*pc.config.HoardingThreshold)
		if float64(stats.MaxCopiesPerUser) > hoardingThreshold {
			hoardingImpact := math.Min(
				(float64(stats.MaxCopiesPerUser)/safeActiveCopies)*pc.config.HoardingImpact,
				2.0, // Cap the maximum hoarding impact
			)
			hoardingFactor = 1.0 + hoardingImpact
		}
	}

	// 4. Activity Factor - prevent division by zero
	activityFactor := math.Max(0.5, safeActiveOwners/safeUniqueOwners)
	activityFactor = math.Min(activityFactor, 2.0) // Cap the maximum activity factor

	// Validate all factors are within reasonable bounds
	factors := PriceFactors{
		ScarcityFactor:     math.Max(0.1, math.Min(scarcityFactor, 3.0)),
		DistributionFactor: math.Max(0.1, math.Min(distributionFactor, 3.0)),
		HoardingFactor:     math.Max(0.1, math.Min(hoardingFactor, 3.0)),
		ActivityFactor:     math.Max(0.1, math.Min(activityFactor, 3.0)),
	}

	factors.Reason = fmt.Sprintf(
		"Price factors: Scarcity (%.2fx), Distribution (%.2fx), Hoarding (%.2fx), Activity (%.2fx)\n"+
			"Based on %d total copies, %d active owners, and %.1f average copies per user",
		factors.ScarcityFactor,
		factors.DistributionFactor,
		factors.HoardingFactor,
		factors.ActivityFactor,
		stats.TotalCopies,
		stats.ActiveOwners,
		stats.AvgCopiesPerUser,
	)

	return factors
}

func (pc *PriceCalculator) processBatch(ctx context.Context, cardIDs []int64, procStats *ProcessingStats, batchNum int) error {
	batchStart := time.Now()

	// Create a shorter timeout for batch processing
	batchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := pc.sem.Acquire(batchCtx, 1); err != nil {
		return err
	}
	defer pc.sem.Release(1)

	// Get card stats with optimized batch size
	cardStats, err := pc.getCardStats(batchCtx, cardIDs)
	if err != nil {
		return fmt.Errorf("failed to get card stats: %w", err)
	}

	// Process prices in parallel
	g, gctx := errgroup.WithContext(batchCtx)
	pricesChan := make(chan struct {
		cardID int64
		price  int64
	}, len(cardIDs))

	for _, cardID := range cardIDs {
		cardID := cardID // Capture for goroutine
		g.Go(func() error {
			stats, ok := cardStats[cardID]
			if !ok {
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

			factors := pc.calculatePriceFactors(stats)
			price := pc.calculateFinalPrice(models.Card{ID: cardID}, factors)

			select {
			case pricesChan <- struct {
				cardID int64
				price  int64
			}{cardID, price}:
			case <-gctx.Done():
				return gctx.Err()
			}
			return nil
		})
	}

	// Close prices channel when all calculations complete
	go func() {
		g.Wait()
		close(pricesChan)
	}()

	// Collect and store prices
	var histories []*models.CardMarketHistory
	for price := range pricesChan {
		history := &models.CardMarketHistory{
			CardID:    price.cardID,
			Price:     price.price,
			Timestamp: time.Now(),
		}
		histories = append(histories, history)
		atomic.AddInt32(&procStats.ProcessedCards, 1)
	}

	if err := g.Wait(); err != nil {
		atomic.AddInt32(&procStats.Errors, 1)
		return fmt.Errorf("error calculating prices: %w", err)
	}

	// Batch insert histories
	if len(histories) > 0 {
		_, err = pc.db.BunDB().NewInsert().
			Model(&histories).
			Exec(batchCtx)

		if err != nil {
			atomic.AddInt32(&procStats.Errors, 1)
			return fmt.Errorf("failed to insert histories: %w", err)
		}
	}

	log.Printf("[%s] [INFO] [MARKET] Batch %d completed in %v",
		time.Now().Format("15:04:05"),
		batchNum,
		time.Since(batchStart))

	return nil
}

func (pc *PriceCalculator) calculateFinalPrice(card models.Card, factors PriceFactors) int64 {
	// Start with base price
	basePrice := float64(pc.config.BasePrice)

	// Apply level multiplier safely
	levelMultiplier := math.Max(1.0, math.Pow(pc.config.LevelMultiplier, float64(card.Level-1)))
	basePrice *= levelMultiplier

	// Apply rarity multiplier with safety bounds
	rarityMultiplier := 1.0 + (math.Max(0, float64(card.Level-1)) * pc.config.RarityMultiplier)
	rarityMultiplier = math.Max(1.0, math.Min(rarityMultiplier, 5.0))
	basePrice *= rarityMultiplier

	// Track price calculation with safety checks
	price := basePrice
	priceSteps := []struct {
		factor float64
		name   string
	}{
		{factors.ScarcityFactor, "Scarcity"},
		{factors.DistributionFactor, "Distribution"},
		{factors.HoardingFactor, "Hoarding"},
		{factors.ActivityFactor, "Activity"},
	}

	for _, step := range priceSteps {
		oldPrice := price
		safeFactor := math.Max(0.1, math.Min(step.factor, 3.0))
		price *= safeFactor

		// Ensure price stays within reasonable bounds after each step
		price = math.Max(float64(pc.config.MinPrice), math.Min(price, float64(pc.config.MaxPrice)))

		log.Printf("[MARKET] Card %d: %s factor %.2f changed price from %d to %d",
			card.ID, step.name, safeFactor, int64(oldPrice), int64(price))
	}

	// Final bounds check
	finalPrice := int64(math.Max(float64(pc.config.MinPrice),
		math.Min(float64(pc.config.MaxPrice), price)))

	// Log final calculation
	log.Printf("[MARKET] Card %d final price calculation:"+
		"\n- Base Price: %.2f"+
		"\n- Rarity Multiplier: %.2f"+
		"\n- Scarcity Factor: %.2f"+
		"\n- Distribution Factor: %.2f"+
		"\n- Hoarding Factor: %.2f"+
		"\n- Activity Factor: %.2f"+
		"\n- Final Price: %d (took %dms)",
		card.ID, basePrice, rarityMultiplier,
		factors.ScarcityFactor, factors.DistributionFactor,
		factors.HoardingFactor, factors.ActivityFactor,
		finalPrice, time.Since(time.Now()).Milliseconds())

	return finalPrice
}

// Helper function to fetch card stats
func (pc *PriceCalculator) fetchCardStats(ctx context.Context, cardID int64, result interface{}) error {
	return pc.db.BunDB().NewSelect().
		TableExpr("user_cards uc").
		ColumnExpr("COUNT(*) as total_copies").
		ColumnExpr("COUNT(DISTINCT uc.user_id) as unique_owners").
		ColumnExpr(`COUNT(DISTINCT CASE 
			WHEN u.last_daily > ? THEN uc.user_id 
			ELSE NULL 
			END) as active_owners`, time.Now().Add(-pc.config.InactivityThreshold)).
		ColumnExpr(`COUNT(CASE 
			WHEN u.last_daily > ? THEN 1 
			ELSE NULL 
			END) as active_copies`, time.Now().Add(-pc.config.InactivityThreshold)).
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

// Validate price factors
func (pc *PriceCalculator) validatePriceFactors(factors PriceFactors) error {
	if factors.ScarcityFactor <= 0 ||
		factors.DistributionFactor <= 0 ||
		factors.HoardingFactor <= 0 ||
		factors.ActivityFactor <= 0 {
		return fmt.Errorf("invalid negative or zero price factors")
	}

	if factors.ScarcityFactor > 10 ||
		factors.DistributionFactor > 10 ||
		factors.HoardingFactor > 10 ||
		factors.ActivityFactor > 10 {
		return fmt.Errorf("price factors exceeded maximum threshold")
	}

	return nil
}

func (stats *CardStats) Validate() error {
	if stats.TotalCopies < 0 ||
		stats.UniqueOwners < 0 ||
		stats.ActiveOwners < 0 ||
		stats.ActiveCopies < 0 ||
		stats.MaxCopiesPerUser < 0 {
		return fmt.Errorf("invalid negative values in card stats")
	}

	if stats.UniqueOwners > stats.TotalCopies {
		return fmt.Errorf("unique owners cannot exceed total copies")
	}

	if stats.ActiveOwners > stats.UniqueOwners {
		return fmt.Errorf("active owners cannot exceed unique owners")
	}

	if stats.ActiveCopies > stats.TotalCopies {
		return fmt.Errorf("active copies cannot exceed total copies")
	}

	return nil
}

func (pc *PriceCalculator) InitializeCardPrices(ctx context.Context) error {
	// Create or update the table schema
	_, err := pc.db.BunDB().NewCreateTable().
		Model((*models.CardMarketHistory)(nil)).
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create market history table: %w", err)
	}

	// Create necessary indexes
	_, err = pc.db.BunDB().NewCreateIndex().
		Model((*models.CardMarketHistory)(nil)).
		Index("idx_card_market_history_card_id_timestamp").
		Column("card_id", "timestamp").
		IfNotExists().
		Exec(ctx)

	if err != nil {
		return err
	}

	// Check if we need to initialize prices
	count, err := pc.db.BunDB().NewSelect().
		Model((*models.CardMarketHistory)(nil)).
		Count(ctx)

	if err != nil {
		return fmt.Errorf("failed to check market history: %w", err)
	}

	// Only initialize if no price history exists
	if count == 0 {
		return pc.performInitialPricing(ctx)
	}

	return nil
}

func (pc *PriceCalculator) calculateInitialPrice(card models.Card, stats CardStats) int64 {
	// Ensure we have valid stats to prevent division by zero
	if stats.TotalCopies == 0 {
		stats.TotalCopies = 1
	}
	if stats.UniqueOwners == 0 {
		stats.UniqueOwners = 1
	}

	// Start with a meaningful base price
	basePrice := float64(InitialBasePrice) * math.Pow(LevelMultiplier, float64(card.Level-1))

	// Scarcity value (fewer copies = higher price)
	scarcityMultiplier := math.Max(0.5, 2.0-(float64(stats.TotalCopies)/ScarcityBaseValue))

	// Distribution value (more unique owners = higher price)
	distributionMultiplier := 1.0
	distributionRatio := float64(stats.UniqueOwners) / float64(stats.TotalCopies)
	distributionMultiplier = math.Max(1.0, 1.0+distributionRatio)

	// Activity multiplier (more active owners = higher price)
	activityMultiplier := 1.0
	if stats.UniqueOwners > 0 {
		activeRatio := float64(stats.ActiveOwners) / float64(stats.UniqueOwners)
		activityMultiplier = math.Max(0.5, 1.0+activeRatio)
	}

	// Rarity multiplier based on level
	rarityMultiplier := 1.0 + (float64(card.Level) * 0.2)

	// Calculate final price with all multipliers
	finalPrice := basePrice *
		scarcityMultiplier *
		distributionMultiplier *
		activityMultiplier *
		rarityMultiplier

	// Ensure minimum price and convert to int64
	return pc.applyPriceLimits(int64(math.Max(finalPrice, float64(MinPrice))))
}

func (pc *PriceCalculator) performInitialPricing(ctx context.Context) error {
	// Create a longer timeout context for initial pricing
	ctx, cancel := context.WithTimeout(ctx, InitialPricingTimeout)
	defer cancel()

	log.Printf("[%s] [INFO] [MARKET] Starting initial card price initialization",
		time.Now().Format("15:04:05"))

	// Optimize the query by breaking it into smaller parts
	var cardIDs []int64
	err := pc.db.BunDB().NewSelect().
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

		err := pc.db.BunDB().NewSelect().
			TableExpr("cards c").
			ColumnExpr("c.*").
			ColumnExpr("COALESCE(COUNT(uc.id), 0) as total_copies").
			ColumnExpr("COALESCE(COUNT(DISTINCT uc.user_id), 0) as unique_owners").
			ColumnExpr(`COALESCE(COUNT(DISTINCT CASE 
				WHEN u.last_daily > ? THEN uc.user_id 
				ELSE NULL 
				END), 0) as active_owners`, time.Now().Add(-pc.config.InactivityThreshold)).
			ColumnExpr(`COALESCE(COUNT(CASE 
				WHEN u.last_daily > ? THEN 1 
				ELSE NULL 
				END), 0) as active_copies`, time.Now().Add(-pc.config.InactivityThreshold)).
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
			initialPrice := pc.calculateInitialPrice(card.Card, stats)
			factors := pc.calculatePriceFactors(stats)

			histories[j] = &models.CardMarketHistory{
				CardID:             card.ID,
				Price:              initialPrice,
				IsActive:           stats.ActiveOwners >= MinimumActiveOwners,
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

			log.Printf("[%s] [INFO] [MARKET] Initialized card %d price: %d (Level: %d, Copies: %d, Owners: %d/%d)",
				time.Now().Format("15:04:05"),
				card.ID,
				initialPrice,
				card.Level,
				stats.TotalCopies,
				stats.ActiveOwners,
				stats.UniqueOwners,
			)
		}

		// Insert batch
		_, err = pc.db.BunDB().NewInsert().
			Model(&histories).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to insert batch: %w", err)
		}

		log.Printf("[%s] [INFO] [MARKET] Processed batch %d-%d of %d cards",
			time.Now().Format("15:04:05"),
			i, end-1, len(cardIDs))
	}

	return nil
}

func (pc *PriceCalculator) GetMarketStats(ctx context.Context, cardID int64, currentPrice int64) (*MarketStats, error) {
	var stats MarketStats
	err := pc.db.BunDB().NewSelect().
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

func (pc *PriceCalculator) GetActiveCards(ctx context.Context) ([]int64, error) {
	var cardIDs []int64
	err := pc.db.BunDB().NewSelect().
		TableExpr("cards c"). // Add explicit table reference
		ColumnExpr("c.id").   // Use table alias
		Join("JOIN user_cards uc ON c.id = uc.card_id").
		Join("JOIN users u ON uc.user_id = u.discord_id").
		Where("u.last_daily > ?", time.Now().Add(-pc.config.InactivityThreshold)).
		GroupExpr("c.id").
		Having("COUNT(DISTINCT uc.user_id) >= ?", MinimumActiveOwners).
		Order("c.id ASC").
		Scan(ctx, &cardIDs)

	if err != nil {
		return nil, fmt.Errorf("failed to get active cards: %w", err)
	}

	return cardIDs, nil
}

// Make these methods public
func (pc *PriceCalculator) GetCardStats(ctx context.Context, cardIDs []int64) (map[int64]CardStats, error) {
	return pc.getCardStats(ctx, cardIDs)
}

func (pc *PriceCalculator) CalculatePriceFactors(stats CardStats) PriceFactors {
	return pc.calculatePriceFactors(stats)
}

// BatchCalculateCardPrices calculates prices for multiple cards efficiently
func (pc *PriceCalculator) BatchCalculateCardPrices(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	prices := make(map[int64]int64)
	oldPrices := make(map[int64]int64)

	// Get old prices
	for _, cardID := range cardIDs {
		oldPrice, _ := pc.GetLastPrice(ctx, cardID)
		oldPrices[cardID] = oldPrice
	}

	// Get card details in bulk
	var cards []models.Card
	err := pc.db.BunDB().NewSelect().
		Model(&cards).
		Where("id IN (?)", bun.In(cardIDs)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cards: %w", err)
	}

	// Get market stats in bulk
	stats, err := pc.getCardStats(ctx, cardIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get card stats: %w", err)
	}

	for _, card := range cards {
		cardStats := stats[card.ID]

		// Pass the entire card object to calculateBasePrice
		basePrice := pc.calculateBasePrice(card)

		// Enhanced market modifiers
		marketMod := 1.0
		if cardStats.TotalCopies > 0 {
			scarcityMod := math.Max(0.5, 2.0-float64(cardStats.TotalCopies)/1000.0)

			activityMod := 1.0
			if cardStats.ActiveOwners > 0 {
				activityMod = 1.0 + float64(cardStats.ActiveOwners)/100.0
			}

			marketMod = scarcityMod * activityMod
		}

		finalPrice := int64(float64(basePrice) * marketMod)
		finalPrice = max(MinPrice, min(MaxPrice, finalPrice))

		prices[card.ID] = finalPrice

		log.Printf("[MARKET] Card %d price calculated: base=%d, market_mod=%.2f, final=%d",
			card.ID, basePrice, marketMod, finalPrice)
	}

	// Update market stats for batch
	for cardID, price := range prices {
		if err := pc.updateMarketStats(ctx, cardID, price, oldPrices[cardID]); err != nil {
			pc.logger.Printf("Warning: Failed to update market stats for card %d: %v", cardID, err)
		}
	}

	return prices, nil
}

func (pc *PriceCalculator) getBatchActiveOwnersCount(ctx context.Context, cardIDs []int64) (map[int64]int, error) {
	type result struct {
		CardID      int64 `bun:"card_id"`
		ActiveCount int   `bun:"active_count"`
	}

	var results []result
	err := pc.db.BunDB().NewSelect().
		TableExpr("user_cards uc").
		ColumnExpr("uc.card_id, COUNT(DISTINCT uc.user_id) as active_count").
		Join("JOIN users u ON uc.user_id = u.discord_id").
		Where("uc.card_id IN (?)", bun.In(cardIDs)).
		Where("u.last_daily > ?", time.Now().Add(-pc.config.InactivityThreshold)).
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

func (pc *PriceCalculator) GetDB() *database.DB {
	return pc.db
}

func (pc *PriceCalculator) CalculateCardPricesBatch(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	prices := make(map[int64]int64)

	// Get card stats for the entire batch
	stats, err := pc.getCardStats(ctx, cardIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get card stats: %w", err)
	}

	// Get all cards in one query
	var cards []models.Card
	err = pc.db.BunDB().NewSelect().
		Model(&cards).
		Where("id IN (?)", bun.In(cardIDs)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cards: %w", err)
	}

	// Calculate prices
	for _, card := range cards {
		cardStats, ok := stats[card.ID]
		if !ok {
			cardStats = CardStats{
				CardID:           card.ID,
				TotalCopies:      1,
				UniqueOwners:     1,
				ActiveOwners:     1,
				ActiveCopies:     1,
				MaxCopiesPerUser: 1,
				AvgCopiesPerUser: 1.0,
			}
		}

		// Calculate price factors
		factors := pc.calculatePriceFactors(cardStats)

		// Calculate final price with validation
		finalPrice := pc.calculateFinalPrice(card, factors)
		if finalPrice <= 0 {
			finalPrice = int64(pc.config.MinPrice)
		}

		prices[card.ID] = finalPrice
	}

	return prices, nil
}

// Add a new validation function
func (pc *PriceCalculator) ValidateCardPrice(ctx context.Context, cardID int64, price int64) error {
	// Get historical prices
	var history []models.CardMarketHistory
	err := pc.db.BunDB().NewSelect().
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
			log.Printf("[MARKET] WARNING: Card %d price %d significantly deviates from average %.2f (±%.2f)",
				cardID, price, avg, stdDev)

			// Optional: Adjust price to be within bounds
			if price > int64(avg+stdDev*2) {
				price = int64(avg + stdDev*2)
			} else if price < int64(avg-stdDev*2) {
				price = int64(avg - stdDev*2)
			}
		}
	}

	return nil
}

// Add this method to update market stats after price calculations
func (pc *PriceCalculator) updateMarketStats(ctx context.Context, cardID int64, newPrice int64, oldPrice int64) error {
	stats, err := pc.statsRepo.GetLatest(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			// Initialize new stats if none exist
			stats = &models.EconomyStats{
				Timestamp:          time.Now(),
				PriceVolatility:    0,
				MarketVolume:       0,
				AverageDailyTrades: 0,
			}
			return pc.statsRepo.Create(ctx, stats)
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

	return pc.statsRepo.Create(ctx, stats)
}
