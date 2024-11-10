package economy

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	lru "github.com/hashicorp/golang-lru"
	"github.com/uptrace/bun"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	batchSize    = 50
	numWorkers   = 3
	queryTimeout = 60 * time.Second

	MinimumActiveOwners = 3
	MinimumTotalCopies  = 1

	maxConcurrentBatches = 5
	workerPoolSize       = 10
	cacheSize            = 10000 // Limit cache size

	MinQueryBatchSize = 100
	MaxRetries        = 3
	MinPrice          = 0       // Minimum price floor
	MaxPrice          = 1000000 // Maximum price ceiling
)

type PriceCalculator struct {
	db     *database.DB
	cache  *lru.Cache
	config PricingConfig
	sem    *semaphore.Weighted
	logger *log.Logger
}

type cachedPrice struct {
	price     int64
	timestamp time.Time
}

type PricingConfig struct {
	BaseMultiplier      float64
	ScarcityImpact      float64
	DistributionImpact  float64
	HoardingThreshold   float64
	HoardingImpact      float64
	ActivityImpact      float64
	OwnershipImpact     float64
	RarityMultiplier    float64
	MinimumPrice        int64
	MaximumPrice        int64
	PriceUpdateInterval time.Duration
	InactivityThreshold time.Duration
	CacheExpiration     time.Duration
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
	CardID           int64
	TotalCopies      int
	UniqueOwners     int
	ActiveOwners     int
	ActiveCopies     int
	MaxCopiesPerUser int
	AvgCopiesPerUser float64
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

func NewPriceCalculator(db *database.DB, config PricingConfig) *PriceCalculator {
	cache, _ := lru.New(cacheSize)
	return &PriceCalculator{
		db:     db,
		cache:  cache,
		config: config,
		sem:    semaphore.NewWeighted(maxConcurrentBatches),
		logger: log.Default(),
	}
}

func (pc *PriceCalculator) CalculateCardPrice(ctx context.Context, cardID int64) (int64, error) {
	var card models.Card
	err := pc.db.BunDB().NewSelect().
		Model(&card).
		Where("id = ?", cardID).
		Scan(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get card: %w", err)
	}

	activeOwners, err := pc.getActiveOwnersCount(ctx, cardID)
	if err != nil {
		return 0, err
	}

	basePrice := pc.calculateBasePrice(card.Level)
	ownershipModifier := pc.calculateOwnershipModifier(activeOwners)
	rarityModifier := pc.calculateRarityModifier(card.Level)
	finalPrice := int64(float64(basePrice) * ownershipModifier * rarityModifier)

	return pc.applyPriceLimits(finalPrice), nil
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

func (pc *PriceCalculator) calculateBasePrice(level int) int64 {
	return int64(math.Pow(float64(level), 2) * pc.config.BaseMultiplier)
}

func (pc *PriceCalculator) calculateOwnershipModifier(activeOwners int) float64 {
	return math.Max(0.1, 1.0-(float64(activeOwners)*pc.config.OwnershipImpact))
}

func (pc *PriceCalculator) calculateRarityModifier(level int) float64 {
	return 1.0 + (float64(level) * pc.config.RarityMultiplier)
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
		Where("timestamp > NOW() - INTERVAL ? DAY", days).
		Order("timestamp ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

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

func (pc *PriceCalculator) UpdateAllPrices(ctx context.Context) error {
	stats := &ProcessingStats{
		StartTime: time.Now(),
	}

	// Print initial memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("[GoHYE] [%s] [INFO] [MARKET] Initial memory usage - Alloc: %v MiB, Sys: %v MiB, NumGC: %v",
		time.Now().Format("15:04:05"),
		m.Alloc/1024/1024,
		m.Sys/1024/1024,
		m.NumGC,
	)

	cardIDs, err := pc.GetActiveCards(ctx)
	if err != nil {
		return err
	}

	stats.CardCount = len(cardIDs)
	stats.BatchCount = (len(cardIDs) + batchSize - 1) / batchSize

	log.Printf("[GoHYE] [%s] [INFO] [MARKET] Starting price update for %d cards in %d batches",
		time.Now().Format("15:04:05"),
		stats.CardCount,
		stats.BatchCount,
	)

	// Process batches
	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < len(cardIDs); i += batchSize {
		end := i + batchSize
		if end > len(cardIDs) {
			end = len(cardIDs)
		}

		batch := cardIDs[i:end]
		batchNum := i/batchSize + 1

		g.Go(func() error {
			return pc.processBatch(ctx, batch, stats, batchNum)
		})
	}

	err = g.Wait()

	// Print final stats
	duration := time.Since(stats.StartTime)
	runtime.ReadMemStats(&m)
	log.Printf("[GoHYE] [%s] [INFO] [MARKET] Price update completed:\n"+
		"- Total time: %v\n"+
		"- Processed cards: %d/%d\n"+
		"- Errors: %d\n"+
		"- Final memory usage - Alloc: %v MiB, Sys: %v MiB, NumGC: %v\n"+
		"- Average processing time per card: %v",
		time.Now().Format("15:04:05"),
		duration,
		atomic.LoadInt32(&stats.ProcessedCards),
		stats.CardCount,
		atomic.LoadInt32(&stats.Errors),
		m.Alloc/1024/1024,
		m.Sys/1024/1024,
		m.NumGC,
		duration/time.Duration(atomic.LoadInt32(&stats.ProcessedCards)),
	)

	return err
}

func (pc *PriceCalculator) getCardStats(ctx context.Context, cardIDs []int64) (map[int64]CardStats, error) {
	var stats []CardStats

	// Create a more accurate query
	err := pc.db.BunDB().NewSelect().
		With("user_copies",
			pc.db.BunDB().NewSelect().
				ColumnExpr("card_id, user_id, COUNT(*) as copies").
				TableExpr("user_cards").
				GroupExpr("card_id, user_id"),
		).
		TableExpr("cards c").
		ColumnExpr("c.id as card_id").
		ColumnExpr("COALESCE(COUNT(uc.id), 0) as total_copies").
		ColumnExpr("COALESCE(COUNT(DISTINCT uc.user_id), 0) as unique_owners").
		ColumnExpr(`COALESCE(COUNT(DISTINCT CASE 
			WHEN u.last_daily > ? THEN uc.user_id 
			END), 0) as active_owners`, time.Now().Add(-pc.config.InactivityThreshold)).
		ColumnExpr(`COALESCE(COUNT(CASE 
			WHEN u.last_daily > ? THEN uc.id 
			END), 0) as active_copies`, time.Now().Add(-pc.config.InactivityThreshold)).
		ColumnExpr("COALESCE(MAX(uc_stats.copies), 0) as max_copies").
		ColumnExpr("COALESCE(AVG(uc_stats.copies)::float, 0) as avg_copies").
		Join("LEFT JOIN user_cards uc ON c.id = uc.card_id").
		Join("LEFT JOIN users u ON uc.user_id = u.discord_id").
		Join("LEFT JOIN user_copies uc_stats ON uc_stats.card_id = c.id").
		Where("c.id IN (?)", bun.In(cardIDs)).
		GroupExpr("c.id").
		Scan(ctx, &stats)

	if err != nil {
		return nil, fmt.Errorf("error fetching card stats: %w", err)
	}

	// Convert to map and add debug logging
	statsMap := make(map[int64]CardStats)
	for _, stat := range stats {
		log.Printf("[GoHYE] [%s] [DEBUG] [MARKET] Card %d stats: Total copies: %d, Unique owners: %d, Active owners: %d, Active copies: %d, Max copies per user: %d, Avg copies per user: %.2f",
			time.Now().Format("15:04:05"),
			stat.CardID,
			stat.TotalCopies,
			stat.UniqueOwners,
			stat.ActiveOwners,
			stat.ActiveCopies,
			stat.MaxCopiesPerUser,
			stat.AvgCopiesPerUser,
		)
		statsMap[stat.CardID] = stat
	}

	return statsMap, nil
}

func (pc *PriceCalculator) calculatePriceFactors(stats CardStats) PriceFactors {
	// 1. Scarcity Factor
	scarcityFactor := math.Max(0.5, 1.0-(float64(stats.ActiveCopies)*pc.config.ScarcityImpact))

	// 2. Distribution Factor
	distributionRatio := float64(stats.ActiveCopies) / float64(stats.ActiveOwners)
	distributionFactor := math.Max(0.5, 1.0-(distributionRatio-1.0)*pc.config.DistributionImpact)

	// 3. Hoarding Impact
	hoardingThreshold := float64(stats.ActiveCopies) * pc.config.HoardingThreshold
	hoardingFactor := 1.0
	if float64(stats.MaxCopiesPerUser) > hoardingThreshold {
		hoardingFactor = 1.0 + (float64(stats.MaxCopiesPerUser)/float64(stats.ActiveCopies))*pc.config.HoardingImpact
	}

	// 4. Activity Factor
	activityFactor := math.Max(0.5, float64(stats.ActiveOwners)/float64(stats.UniqueOwners))

	reason := fmt.Sprintf(
		"Price factors:\n"+
			"- Scarcity: %.2fx (%d active copies)\n"+
			"- Distribution: %.2fx (%d copies across %d owners)\n"+
			"- Hoarding: %.2fx (max %d copies per user)\n"+
			"- Activity: %.2fx (%d/%d active owners)",
		scarcityFactor, stats.ActiveCopies,
		distributionFactor, stats.ActiveCopies, stats.ActiveOwners,
		hoardingFactor, stats.MaxCopiesPerUser,
		activityFactor, stats.ActiveOwners, stats.UniqueOwners,
	)

	return PriceFactors{
		ScarcityFactor:     scarcityFactor,
		DistributionFactor: distributionFactor,
		HoardingFactor:     hoardingFactor,
		ActivityFactor:     activityFactor,
		Reason:             reason,
	}
}

func (pc *PriceCalculator) processBatch(ctx context.Context, cardIDs []int64, stats *ProcessingStats, batchNum int) error {
	batchStart := time.Now()

	if err := pc.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer pc.sem.Release(1)

	log.Printf("[GoHYE] [%s] [INFO] [MARKET] Starting batch %d with %d cards",
		time.Now().Format("15:04:05"),
		batchNum,
		len(cardIDs),
	)

	// Worker pool implementation
	g, ctx := errgroup.WithContext(ctx)
	workChan := make(chan int64, len(cardIDs))

	for i := 0; i < workerPoolSize; i++ {
		workerNum := i
		g.Go(func() error {
			log.Printf("[GoHYE] [%s] [DEBUG] [MARKET] Batch %d: Worker %d started",
				time.Now().Format("15:04:05"),
				batchNum,
				workerNum,
			)

			processed := 0
			for cardID := range workChan {
				start := time.Now()
				if err := pc.processCard(ctx, cardID); err != nil {
					atomic.AddInt32((*int32)(&stats.Errors), 1)
					log.Printf("[GoHYE] [%s] [ERROR] [MARKET] Error processing card %d: %v",
						time.Now().Format("15:04:05"),
						cardID,
						err,
					)
					return err
				}
				processed++
				atomic.AddInt32((*int32)(&stats.ProcessedCards), 1)

				log.Printf("[GoHYE] [%s] [DEBUG] [MARKET] Batch %d: Worker %d processed card %d in %v",
					time.Now().Format("15:04:05"),
					batchNum,
					workerNum,
					cardID,
					time.Since(start),
				)
			}
			return nil
		})
	}

	// Send work and wait for completion
	for _, cardID := range cardIDs {
		select {
		case workChan <- cardID:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	close(workChan)

	err := g.Wait()

	// Log batch completion
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("[GoHYE] [%s] [INFO] [MARKET] Batch %d completed:\n"+
		"- Time taken: %v\n"+
		"- Memory usage - Alloc: %v MiB, Sys: %v MiB\n"+
		"- Cards processed: %d",
		time.Now().Format("15:04:05"),
		batchNum,
		time.Since(batchStart),
		m.Alloc/1024/1024,
		m.Sys/1024/1024,
		len(cardIDs),
	)

	return err
}

func (pc *PriceCalculator) processCard(ctx context.Context, cardID int64) error {
	start := time.Now()

	// Retry mechanism for database operations
	var result struct {
		models.Card
		TotalCopies  int     `bun:"total_copies"`
		UniqueOwners int     `bun:"unique_owners"`
		ActiveOwners int     `bun:"active_owners"`
		ActiveCopies int     `bun:"active_copies"`
		MaxCopies    int     `bun:"max_copies"`
		AvgCopies    float64 `bun:"avg_copies"`
	}

	var err error
	for retry := 0; retry < MaxRetries; retry++ {
		err = pc.fetchCardStats(ctx, cardID, &result)
		if err == nil {
			break
		}
		if retry < MaxRetries-1 {
			time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to fetch card stats after %d retries: %w", MaxRetries, err)
	}

	// Validate the results
	stats := CardStats{
		CardID:           cardID,
		TotalCopies:      result.TotalCopies,
		UniqueOwners:     result.UniqueOwners,
		ActiveOwners:     result.ActiveOwners,
		ActiveCopies:     result.ActiveCopies,
		MaxCopiesPerUser: result.MaxCopies,
		AvgCopiesPerUser: result.AvgCopies,
	}

	if err := stats.Validate(); err != nil {
		return fmt.Errorf("card %d stats validation failed: %w", cardID, err)
	}

	// Get previous price with retry
	var prevHistory models.CardMarketHistory
	for retry := 0; retry < MaxRetries; retry++ {
		err = pc.db.BunDB().NewSelect().
			Model(&prevHistory).
			Where("card_id = ?", cardID).
			Order("timestamp DESC").
			Limit(1).
			Scan(ctx)
		if err == nil || err == sql.ErrNoRows {
			break
		}
		if retry < MaxRetries-1 {
			time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond)
		}
	}

	// Calculate price factors and validate
	factors := pc.calculatePriceFactors(stats)
	if err := pc.validatePriceFactors(factors); err != nil {
		return fmt.Errorf("invalid price factors for card %d: %w", cardID, err)
	}

	price := pc.calculateFinalPrice(result.Card, factors)
	price = pc.applyPriceLimits(price) // Ensure price is within bounds

	// Calculate price change with safety checks
	priceChangePercent := 0.0
	if prevHistory.Price > 0 {
		priceChangePercent = math.Round((float64(price-prevHistory.Price)/float64(prevHistory.Price)*100)*100) / 100
	}

	// Prepare history record with validation
	history := &models.CardMarketHistory{
		CardID:             cardID,
		Price:              price,
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
		PriceReason:        factors.Reason,
		Timestamp:          time.Now().UTC(),
		PriceChangePercent: priceChangePercent,
	}

	// Insert with retry
	for retry := 0; retry < MaxRetries; retry++ {
		_, err = pc.db.BunDB().NewInsert().
			Model(history).
			Exec(ctx)
		if err == nil {
			break
		}
		if retry < MaxRetries-1 {
			time.Sleep(time.Duration(retry+1) * 100 * time.Millisecond)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to insert market history after %d retries: %w", MaxRetries, err)
	}

	// Log successful processing
	log.Printf("[GoHYE] [%s] [INFO] [MARKET] Card %d processed successfully:\n"+
		"- Processing time: %v\n"+
		"- Price: %d (%.2f%% change)\n"+
		"- Active: %v (%d/%d owners)\n"+
		"- Copies: %d total, %d active\n"+
		"- Distribution: %.2f avg, %d max per user\n"+
		"- Factors: %.2fx scarcity, %.2fx distribution, %.2fx hoarding, %.2fx activity",
		time.Now().Format("15:04:05"),
		cardID,
		time.Since(start),
		price,
		priceChangePercent,
		history.IsActive,
		stats.ActiveOwners,
		stats.UniqueOwners,
		stats.TotalCopies,
		stats.ActiveCopies,
		stats.AvgCopiesPerUser,
		stats.MaxCopiesPerUser,
		factors.ScarcityFactor,
		factors.DistributionFactor,
		factors.HoardingFactor,
		factors.ActivityFactor,
	)

	return nil
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

func (pc *PriceCalculator) performInitialPricing(ctx context.Context) error {
	log.Printf("[GoHYE] [%s] [INFO] [MARKET] Starting initial card price initialization",
		time.Now().Format("15:04:05"))

	// Get all cards with their stats
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
		ColumnExpr("COUNT(uc.id) as total_copies").
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
			SELECT MAX(copies)
			FROM (
				SELECT COUNT(*) as copies
				FROM user_cards uc2
				WHERE uc2.card_id = c.id
				GROUP BY uc2.user_id
			) t
		), 0) as max_copies`).
		ColumnExpr(`COALESCE((
			SELECT AVG(copies)::float
			FROM (
				SELECT COUNT(*) as copies
				FROM user_cards uc2
				WHERE uc2.card_id = c.id
				GROUP BY uc2.user_id
			) t
		), 0) as avg_copies`).
		Join("LEFT JOIN user_cards uc ON c.id = uc.card_id").
		Join("LEFT JOIN users u ON uc.user_id = u.discord_id").
		GroupExpr("c.id").
		Scan(ctx, &cards)

	if err != nil {
		return fmt.Errorf("failed to fetch cards with stats: %w", err)
	}

	// Process cards in batches
	batchSize := 100
	for i := 0; i < len(cards); i += batchSize {
		end := i + batchSize
		if end > len(cards) {
			end = len(cards)
		}

		batch := cards[i:end]
		histories := make([]*models.CardMarketHistory, len(batch))

		for j, card := range batch {
			stats := CardStats{
				CardID:           card.ID,
				TotalCopies:      card.TotalCopies,
				UniqueOwners:     card.UniqueOwners,
				ActiveOwners:     card.ActiveOwners,
				ActiveCopies:     card.ActiveCopies,
				MaxCopiesPerUser: card.MaxCopies,
				AvgCopiesPerUser: card.AvgCopies,
			}

			factors := pc.calculatePriceFactors(stats)
			initialPrice := pc.calculateFinalPrice(card.Card, factors)

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
				PriceReason:        factors.Reason,
				Timestamp:          time.Now().UTC(),
				PriceChangePercent: 0,
			}

			log.Printf("[GoHYE] [%s] [INFO] [MARKET] Initialized card %d price: %d (Level: %d, Active Owners: %d)",
				time.Now().Format("15:04:05"),
				card.ID,
				initialPrice,
				card.Level,
				stats.ActiveOwners,
			)
		}

		_, err = pc.db.BunDB().NewInsert().
			Model(&histories).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to insert initial prices batch: %w", err)
		}
	}

	log.Printf("[GoHYE] [%s] [INFO] [MARKET] Completed initial price initialization for %d cards",
		time.Now().Format("15:04:05"),
		len(cards),
	)

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

func (pc *PriceCalculator) calculateFinalPrice(card models.Card, factors PriceFactors) int64 {
	basePrice := pc.calculateBasePrice(card.Level)

	// Apply all factors
	adjustedPrice := float64(basePrice) *
		factors.ScarcityFactor *
		factors.DistributionFactor *
		factors.HoardingFactor *
		factors.ActivityFactor *
		pc.calculateRarityModifier(card.Level)

	// Convert to int64 and apply limits
	return pc.applyPriceLimits(int64(adjustedPrice))
}
