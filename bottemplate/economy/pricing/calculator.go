package pricing

import (
	"fmt"
	"log"
	"math"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/utils"
)

// Use centralized economic constants from utils package

// PricingConfig holds all configuration for price calculation
type PricingConfig struct {
	BasePrice           int64   // Base price for level 1 cards
	LevelMultiplier     float64 // Price multiplier per level
	ScarcityWeight      float64 // Weight for scarcity impact
	ActivityWeight      float64 // Weight for activity impact
	MinPrice            int64   // Absolute minimum price
	MaxPrice            int64   // Absolute maximum price
	MinActiveOwners     int     // Minimum active owners for price calculation
	MinTotalCopies      int     // Minimum total copies for price calculation
	BaseMultiplier      float64 // Base price multiplier
	ScarcityImpact      float64 // Price reduction per copy
	DistributionImpact  float64 // Impact for distribution
	HoardingThreshold   float64 // Supply threshold for hoarding
	HoardingImpact      float64 // Price increase for hoarding
	ActivityImpact      float64 // Impact for activity
	OwnershipImpact     float64 // Impact per owner
	RarityMultiplier    float64 // Increase per rarity level
}

// CardStats represents the statistical data for a card
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

// PriceFactors represents the factors that influence card pricing
type PriceFactors struct {
	ScarcityFactor     float64
	DistributionFactor float64
	HoardingFactor     float64
	ActivityFactor     float64
	Reason             string
}

// Calculator handles pure price calculation logic
type Calculator struct {
	config PricingConfig
	logger *log.Logger
}

// NewCalculator creates a new price calculator with the given configuration
func NewCalculator(config PricingConfig) *Calculator {
	return &Calculator{
		config: config,
		logger: log.Default(),
	}
}

// CalculateBasePrice calculates the base price for a card based on its level
func (c *Calculator) CalculateBasePrice(card models.Card) int64 {
	// Base price calculation with level scaling
	basePrice := utils.InitialBasePrice * int64(math.Pow(utils.LevelMultiplier, float64(card.Level-1)))
	return max(basePrice, utils.MinPrice)
}

// CalculateOwnershipModifier calculates ownership modifier for scarcity
func (c *Calculator) CalculateOwnershipModifier(activeOwners int) float64 {
	if activeOwners < utils.MinimumActiveOwners {
		return 1.0
	}

	// Inverse logarithmic scaling for scarcity
	scarcityMod := 2.0 - math.Log10(float64(activeOwners)+1)/2.0
	return math.Max(1.0, scarcityMod)
}

// CalculateRarityModifier calculates rarity modifier based on card level
func (c *Calculator) CalculateRarityModifier(level int) float64 {
	// Exponential scaling for rarity
	return math.Pow(1.5, float64(level-1))
}

// CalculatePriceFactors calculates all the factors that influence card pricing
func (c *Calculator) CalculatePriceFactors(stats CardStats) PriceFactors {
	// Ensure minimum values to prevent division by zero
	safeActiveOwners := math.Max(1.0, float64(stats.ActiveOwners))
	safeUniqueOwners := math.Max(1.0, float64(stats.UniqueOwners))
	safeActiveCopies := math.Max(1.0, float64(stats.ActiveCopies))

	// 1. Scarcity Factor - prevent division by zero
	scarcityFactor := math.Max(0.5, 1.0-(safeActiveCopies*c.config.ScarcityImpact))

	// 2. Distribution Factor - prevent NaN
	distributionRatio := safeActiveCopies / safeActiveOwners
	distributionFactor := math.Max(0.5, 1.0-(math.Min(distributionRatio, 10.0)-1.0)*c.config.DistributionImpact)

	// 3. Hoarding Impact - prevent Inf
	hoardingFactor := 1.0
	if stats.MaxCopiesPerUser > 0 {
		hoardingThreshold := math.Max(1.0, safeActiveCopies*c.config.HoardingThreshold)
		if float64(stats.MaxCopiesPerUser) > hoardingThreshold {
			hoardingImpact := math.Min(
				(float64(stats.MaxCopiesPerUser)/safeActiveCopies)*c.config.HoardingImpact,
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

// CalculateFinalPrice calculates the final price using all factors and modifiers
func (c *Calculator) CalculateFinalPrice(card models.Card, factors PriceFactors) int64 {
	// Start with base price
	basePrice := float64(c.config.BasePrice)

	// Apply level multiplier safely
	levelMultiplier := math.Max(1.0, math.Pow(c.config.LevelMultiplier, float64(card.Level-1)))
	basePrice *= levelMultiplier

	// Apply rarity multiplier with safety bounds
	rarityMultiplier := 1.0 + (math.Max(0, float64(card.Level-1)) * c.config.RarityMultiplier)
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
		safeFactor := math.Max(0.1, math.Min(step.factor, 3.0))
		price *= safeFactor

		// Ensure price stays within reasonable bounds after each step
		price = math.Max(float64(c.config.MinPrice), math.Min(price, float64(c.config.MaxPrice)))

		// Per-step logging removed for performance - enable only for debugging specific cards
	}

	// Final bounds check
	finalPrice := int64(math.Max(float64(c.config.MinPrice),
		math.Min(float64(c.config.MaxPrice), price)))

	// Detailed per-card calculation logging removed for performance
	// Enable only for debugging specific pricing issues

	return finalPrice
}

// CalculateInitialPrice calculates initial price for cards during first pricing
func (c *Calculator) CalculateInitialPrice(card models.Card, stats CardStats) int64 {
	// Ensure we have valid stats to prevent division by zero
	if stats.TotalCopies == 0 {
		stats.TotalCopies = 1
	}
	if stats.UniqueOwners == 0 {
		stats.UniqueOwners = 1
	}

	// Start with a meaningful base price
	basePrice := float64(utils.InitialBasePrice) * math.Pow(utils.LevelMultiplier, float64(card.Level-1))

	// Scarcity value (fewer copies = higher price)
	scarcityMultiplier := math.Max(0.5, 2.0-(float64(stats.TotalCopies)/utils.ScarcityBaseValue))

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
	return c.ApplyPriceLimits(int64(math.Max(finalPrice, float64(utils.MinPrice))))
}

// ApplyPriceLimits ensures price stays within configured bounds
func (c *Calculator) ApplyPriceLimits(price int64) int64 {
	if price < utils.MinPrice {
		return utils.MinPrice
	}
	if price > utils.MaxPrice {
		return utils.MaxPrice
	}
	return price
}

// ValidatePriceFactors validates that price factors are within reasonable bounds
func (c *Calculator) ValidatePriceFactors(factors PriceFactors) error {
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

// ValidateCardStats validates that card statistics are consistent
func (c *Calculator) ValidateCardStats(stats CardStats) error {
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