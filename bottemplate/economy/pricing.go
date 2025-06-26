package economy

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy/pricing"
	"github.com/disgoorg/bot-template/bottemplate/economy/utils"
)

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

// PriceCalculator coordinates all pricing components
type PriceCalculator struct {
	calculator *pricing.Calculator
	analyzer   *pricing.MarketAnalyzer
	store      *pricing.PriceStore
	scheduler  *pricing.PriceScheduler
	config     PricingConfig
	logger     *log.Logger
}

// Expose types from pricing package for backward compatibility
type PricePoint = pricing.PricePoint
type MarketStats = pricing.MarketStats
type CardStats = pricing.CardStats
type PriceFactors = pricing.PriceFactors

// Constants for backward compatibility
const (
	MinimumActiveOwners = utils.MinimumActiveOwners
	MinimumTotalCopies  = utils.MinimumTotalCopies
	MinPrice            = utils.MinPrice
	MaxPrice            = utils.MaxPrice
)

// NewPriceCalculator creates a new coordinated price calculator
func NewPriceCalculator(db *database.DB, config PricingConfig, statsRepo repositories.EconomyStatsRepository) *PriceCalculator {
	// Convert to internal pricing config
	pricingConfig := pricing.PricingConfig{
		BasePrice:          config.BasePrice,
		LevelMultiplier:    config.LevelMultiplier,
		ScarcityWeight:     config.ScarcityWeight,
		ActivityWeight:     config.ActivityWeight,
		MinPrice:           config.MinPrice,
		MaxPrice:           config.MaxPrice,
		MinActiveOwners:    config.MinActiveOwners,
		MinTotalCopies:     config.MinTotalCopies,
		BaseMultiplier:     config.BaseMultiplier,
		ScarcityImpact:     config.ScarcityImpact,
		DistributionImpact: config.DistributionImpact,
		HoardingThreshold:  config.HoardingThreshold,
		HoardingImpact:     config.HoardingImpact,
		ActivityImpact:     config.ActivityImpact,
		OwnershipImpact:    config.OwnershipImpact,
		RarityMultiplier:   config.RarityMultiplier,
	}

	// Create components
	calculator := pricing.NewCalculator(pricingConfig)
	analyzer := pricing.NewMarketAnalyzer(db, pricingConfig, config.InactivityThreshold)
	store := pricing.NewPriceStore(db, pricingConfig, statsRepo, config.CacheExpiration)
	scheduler := pricing.NewPriceScheduler(calculator, analyzer, store, config.PriceUpdateInterval)

	return &PriceCalculator{
		calculator: calculator,
		analyzer:   analyzer,
		store:      store,
		scheduler:  scheduler,
		config:     config,
		logger:     log.Default(),
	}
}

// CalculateCardPrice calculates the price for a single card
func (pc *PriceCalculator) CalculateCardPrice(ctx context.Context, cardID int64) (int64, error) {
	// Get old price for comparison
	oldPrice, err := pc.GetLastPrice(ctx, cardID)
	if err != nil {
		pc.logger.Printf("Warning: Could not get old price: %v", err)
	}

	// Get card details
	var card models.Card
	err = pc.store.GetDB().BunDB().NewSelect().
		Model(&card).
		Where("id = ?", cardID).
		Scan(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get card: %w", err)
	}

	log.Printf("[%s] [DEBUG] [MARKET] Found card %d with level %d",
		time.Now().Format("15:04:05"), cardID, card.Level)

	// Get active owners count
	activeOwners, err := pc.analyzer.GetActiveOwnersCount(ctx, cardID)
	if err != nil {
		return 0, err
	}

	log.Printf("[%s] [DEBUG] [MARKET] Card %d has %d active owners",
		time.Now().Format("15:04:05"), cardID, activeOwners)

	// Calculate price components
	basePrice := pc.calculator.CalculateBasePrice(card)
	ownershipModifier := pc.calculator.CalculateOwnershipModifier(activeOwners)
	rarityModifier := pc.calculator.CalculateRarityModifier(card.Level)

	// Calculate final price
	finalPrice := int64(float64(basePrice) * ownershipModifier * rarityModifier)
	finalPrice = pc.calculator.ApplyPriceLimits(finalPrice)

	log.Printf("[%s] [DEBUG] [MARKET] Card %d price calculation: base=%d, ownership=%.2f, rarity=%.2f, final=%d",
		time.Now().Format("15:04:05"), cardID, basePrice, ownershipModifier, rarityModifier, finalPrice)

	// Update market stats
	if err := pc.store.UpdateMarketStats(ctx, cardID, finalPrice, oldPrice); err != nil {
		pc.logger.Printf("Warning: Failed to update market stats: %v", err)
	}

	return finalPrice, nil
}

// GetLastPrice retrieves the most recent price for a card
func (pc *PriceCalculator) GetLastPrice(ctx context.Context, cardID int64) (int64, error) {
	return pc.store.GetLastPrice(ctx, cardID)
}

// GetLatestPrice retrieves the latest price for a card
func (pc *PriceCalculator) GetLatestPrice(ctx context.Context, cardID int64) (int64, error) {
	return pc.store.GetLatestPrice(ctx, cardID, pc.calculator, pc.analyzer)
}

// UpdateCardPrice updates the price for a card
func (pc *PriceCalculator) UpdateCardPrice(ctx context.Context, cardID int64) error {
	var card models.Card
	err := pc.store.GetDB().BunDB().NewSelect().
		Model(&card).
		Where("id = ?", cardID).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to get card: %w", err)
	}

	activeOwners, err := pc.analyzer.GetActiveOwnersCount(ctx, cardID)
	if err != nil {
		return fmt.Errorf("failed to get active owners count: %w", err)
	}

	price, err := pc.CalculateCardPrice(ctx, cardID)
	if err != nil {
		return err
	}

	return pc.store.UpdateCardPrice(ctx, cardID, price, activeOwners)
}

// GetPriceHistory retrieves price history for a card
func (pc *PriceCalculator) GetPriceHistory(ctx context.Context, cardID int64, days int) ([]PricePoint, error) {
	return pc.store.GetPriceHistory(ctx, cardID, days, pc.calculator, pc.analyzer)
}

// StartPriceUpdateJob starts the background price update job
func (pc *PriceCalculator) StartPriceUpdateJob(ctx context.Context) {
	pc.scheduler.StartPriceUpdateJob(ctx)
}

// UpdateAllPrices updates prices for all active cards
func (pc *PriceCalculator) UpdateAllPrices(ctx context.Context) error {
	return pc.scheduler.UpdateAllPrices(ctx)
}

// GetActiveCards returns all cards that have active owners
func (pc *PriceCalculator) GetActiveCards(ctx context.Context) ([]int64, error) {
	return pc.analyzer.GetActiveCards(ctx)
}

// GetCardStats retrieves comprehensive statistics for multiple cards
func (pc *PriceCalculator) GetCardStats(ctx context.Context, cardIDs []int64) (map[int64]CardStats, error) {
	return pc.analyzer.GetCardStats(ctx, cardIDs)
}

// CalculatePriceFactors calculates price factors for given card stats
func (pc *PriceCalculator) CalculatePriceFactors(stats CardStats) PriceFactors {
	return pc.calculator.CalculatePriceFactors(stats)
}

// BatchCalculateCardPrices calculates prices for multiple cards efficiently
func (pc *PriceCalculator) BatchCalculateCardPrices(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	return pc.scheduler.BatchCalculateCardPrices(ctx, cardIDs)
}

// CalculateCardPricesBatch calculates prices for multiple cards using full factor analysis
func (pc *PriceCalculator) CalculateCardPricesBatch(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	return pc.scheduler.CalculateCardPricesBatch(ctx, cardIDs)
}

// GetBatchActiveOwnersCount returns active owner counts for multiple cards
func (pc *PriceCalculator) getBatchActiveOwnersCount(ctx context.Context, cardIDs []int64) (map[int64]int, error) {
	return pc.analyzer.GetBatchActiveOwnersCount(ctx, cardIDs)
}

// GetDB returns the database instance for backward compatibility
func (pc *PriceCalculator) GetDB() *database.DB {
	return pc.store.GetDB()
}

// InitializeCardPrices initializes the price system
func (pc *PriceCalculator) InitializeCardPrices(ctx context.Context) error {
	return pc.store.InitializeCardPrices(ctx, pc.calculator, pc.analyzer)
}

// GetMarketStats retrieves market statistics for a card
func (pc *PriceCalculator) GetMarketStats(ctx context.Context, cardID int64, currentPrice int64) (*MarketStats, error) {
	return pc.analyzer.GetMarketStats(ctx, cardID, currentPrice)
}

// ValidateCardPrice validates a price against historical data
func (pc *PriceCalculator) ValidateCardPrice(ctx context.Context, cardID int64, price int64) error {
	return pc.store.ValidateCardPrice(ctx, cardID, price)
}