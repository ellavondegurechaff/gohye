package pricing

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/uptrace/bun"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	batchSize            = 50
	numWorkers           = 3
	queryTimeout         = 60 * time.Second
	maxConcurrentBatches = 5
	workerPoolSize       = 10
	MaxRetries           = 3
)

// ProcessingStats tracks statistics during price updates
type ProcessingStats struct {
	StartTime      time.Time
	CardCount      int
	BatchCount     int
	ProcessedCards int32 // Using int32 for atomic operations
	Errors         int32 // Using int32 for atomic operations
}

// PriceScheduler handles background price update orchestration and batch processing
type PriceScheduler struct {
	calculator     *Calculator
	analyzer       *MarketAnalyzer
	store          *PriceStore
	sem            *semaphore.Weighted
	updateInterval time.Duration
	logger         *log.Logger
}

// NewPriceScheduler creates a new price scheduler
func NewPriceScheduler(calculator *Calculator, analyzer *MarketAnalyzer, store *PriceStore, updateInterval time.Duration) *PriceScheduler {
	return &PriceScheduler{
		calculator:     calculator,
		analyzer:       analyzer,
		store:          store,
		sem:            semaphore.NewWeighted(maxConcurrentBatches),
		updateInterval: updateInterval,
		logger:         log.Default(),
	}
}

// StartPriceUpdateJob starts the background price update job
func (ps *PriceScheduler) StartPriceUpdateJob(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(ps.updateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := ps.UpdateAllPrices(ctx); err != nil {
					log.Printf("Error updating prices: %v", err)
				}
			}
		}
	}()
}

// Add this to prevent repeated runs
var isUpdating atomic.Bool

// UpdateAllPrices updates prices for all active cards in batches
func (ps *PriceScheduler) UpdateAllPrices(ctx context.Context) error {
	// Add mutex to prevent concurrent updates
	var updateMutex sync.Mutex
	updateMutex.Lock()
	defer updateMutex.Unlock()

	// Get active cards with timeout
	cardIDs, err := ps.analyzer.GetActiveCards(ctx)
	if err != nil {
		return err
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
			if err := ps.processBatch(ctx, cardIDs[i:end], stats, batchNum); err != nil {
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

// processBatch processes a batch of cards for price updates
func (ps *PriceScheduler) processBatch(ctx context.Context, cardIDs []int64, procStats *ProcessingStats, batchNum int) error {
	batchStart := time.Now()

	// Create a shorter timeout for batch processing
	batchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := ps.sem.Acquire(batchCtx, 1); err != nil {
		return err
	}
	defer ps.sem.Release(1)

	// Get card stats with optimized batch size
	cardStats, err := ps.analyzer.GetCardStats(batchCtx, cardIDs)
	if err != nil {
		return err
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

			factors := ps.calculator.CalculatePriceFactors(stats)
			price := ps.calculator.CalculateFinalPrice(models.Card{ID: cardID}, factors)

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
		return err
	}

	// Batch insert histories
	if len(histories) > 0 {
		err = ps.store.StoreHistories(batchCtx, histories)
		if err != nil {
			atomic.AddInt32(&procStats.Errors, 1)
			return err
		}
	}

	// Log batch completion less frequently to reduce noise
	if batchNum%10 == 0 || batchNum == 1 {
		log.Printf("[%s] [INFO] [MARKET] Batch %d completed in %v",
			time.Now().Format("15:04:05"),
			batchNum,
			time.Since(batchStart))
	}

	return nil
}

// BatchCalculateCardPrices calculates prices for multiple cards efficiently
func (ps *PriceScheduler) BatchCalculateCardPrices(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	prices := make(map[int64]int64)
	oldPrices := make(map[int64]int64)

	// Get old prices
	for _, cardID := range cardIDs {
		oldPrice, _ := ps.store.GetLastPrice(ctx, cardID)
		oldPrices[cardID] = oldPrice
	}

	// Get card details in bulk
	var cards []models.Card
	err := ps.store.GetDB().BunDB().NewSelect().
		Model(&cards).
		Where("id IN (?)", bun.In(cardIDs)).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Get market stats in bulk
	stats, err := ps.analyzer.GetCardStats(ctx, cardIDs)
	if err != nil {
		return nil, err
	}

	for _, card := range cards {
		cardStats := stats[card.ID]

		// Pass the entire card object to calculateBasePrice
		basePrice := ps.calculator.CalculateBasePrice(card)

		// Enhanced market modifiers
		marketMod := 1.0
		if cardStats.TotalCopies > 0 {
			// Simple scarcity and activity modifiers for batch calculation
			scarcityMod := max(0.5, 2.0-float64(cardStats.TotalCopies)/1000.0)

			activityMod := 1.0
			if cardStats.ActiveOwners > 0 {
				activityMod = 1.0 + float64(cardStats.ActiveOwners)/100.0
			}

			marketMod = scarcityMod * activityMod
		}

		finalPrice := int64(float64(basePrice) * marketMod)
		finalPrice = max(utils.MinPrice, min(utils.MaxPrice, finalPrice))

		prices[card.ID] = finalPrice

		// Per-card logging removed for performance - use batch summary instead
	}

	// Update market stats for batch
	for cardID, price := range prices {
		if err := ps.store.UpdateMarketStats(ctx, cardID, price, oldPrices[cardID]); err != nil {
			ps.logger.Printf("Warning: Failed to update market stats for card %d: %v", cardID, err)
		}
	}

	return prices, nil
}

// CalculateCardPricesBatch calculates prices for multiple cards using full factor analysis
func (ps *PriceScheduler) CalculateCardPricesBatch(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	prices := make(map[int64]int64)

	// Get card stats for the entire batch
	stats, err := ps.analyzer.GetCardStats(ctx, cardIDs)
	if err != nil {
		return nil, err
	}

	// Get all cards in one query
	var cards []models.Card
	err = ps.store.GetDB().BunDB().NewSelect().
		Model(&cards).
		Where("id IN (?)", bun.In(cardIDs)).
		Scan(ctx)
	if err != nil {
		return nil, err
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
		factors := ps.calculator.CalculatePriceFactors(cardStats)

		// Calculate final price with validation
		finalPrice := ps.calculator.CalculateFinalPrice(card, factors)
		if finalPrice <= 0 {
			finalPrice = utils.MinPrice
		}

		prices[card.ID] = finalPrice
	}

	return prices, nil
}
