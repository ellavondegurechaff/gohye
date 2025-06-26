package utils

import (
	"fmt"
	"math"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
)

// CardValidation provides common card validation functions
type CardValidation struct{}

// NewCardValidation creates a new card validation helper
func NewCardValidation() *CardValidation {
	return &CardValidation{}
}

// ValidateCardLevel checks if a card level is within valid range
func (cv *CardValidation) ValidateCardLevel(level int) error {
	if level < MinCardLevel || level > MaxCardLevel {
		return fmt.Errorf("invalid card level: %d (must be between %d and %d)", level, MinCardLevel, MaxCardLevel)
	}
	return nil
}

// ValidateCardForLiquefying checks if a card can be liquefied
func (cv *CardValidation) ValidateCardForLiquefying(card *models.Card) error {
	if card.Level > MaxLiquefyCardLevel {
		return fmt.Errorf("cannot liquefy cards above %d stars (card is %d stars)", MaxLiquefyCardLevel, card.Level)
	}
	return nil
}

// ValidatePrice ensures a price is within acceptable bounds
func (cv *CardValidation) ValidatePrice(price int64) error {
	if price < MinPrice {
		return fmt.Errorf("price %d is below minimum price %d", price, MinPrice)
	}
	if price > MaxPrice {
		return fmt.Errorf("price %d exceeds maximum price %d", price, MaxPrice)
	}
	return nil
}

// CalculateBasePrice calculates the base price for a card based on its level
func (cv *CardValidation) CalculateBasePrice(card models.Card) int64 {
	if err := cv.ValidateCardLevel(card.Level); err != nil {
		return MinPrice
	}
	
	basePrice := InitialBasePrice * int64(math.Pow(LevelMultiplier, float64(card.Level-1)))
	return max(basePrice, MinPrice)
}

// CalculateVialRate returns the vial conversion rate for a card level
func (cv *CardValidation) CalculateVialRate(level int) (float64, error) {
	switch level {
	case 1:
		return VialRate1Star, nil
	case 2:
		return VialRate2Star, nil
	case 3:
		return VialRate3Star, nil
	default:
		return 0, fmt.Errorf("invalid card level for vial calculation: %d", level)
	}
}

// CalculateVialYield calculates how many vials a card would yield
func (cv *CardValidation) CalculateVialYield(card *models.Card, price int64) (int64, error) {
	if err := cv.ValidateCardForLiquefying(card); err != nil {
		return 0, err
	}

	vialRate, err := cv.CalculateVialRate(card.Level)
	if err != nil {
		return 0, err
	}

	vials := int64(math.Floor(float64(price) * vialRate))
	return vials, nil
}

// CalculateForgeCost calculates the cost to forge two cards
func (cv *CardValidation) CalculateForgeCost(price1, price2 int64) int64 {
	avgPrice := (price1 + price2) / 2
	forgeCost := int64(math.Max(float64(avgPrice)*ForgeCostMultiplier, float64(MinForgeCost)))
	return forgeCost
}

// ApplyPriceLimits ensures a price is within the acceptable range
func (cv *CardValidation) ApplyPriceLimits(price int64) int64 {
	if price < MinPrice {
		return MinPrice
	}
	if price > MaxPrice {
		return MaxPrice
	}
	return price
}

// ValidateBidAmount checks if a bid amount is valid for an auction
func (cv *CardValidation) ValidateBidAmount(currentPrice, bidAmount int64) error {
	minValidBid := currentPrice + MinBidIncrement
	if bidAmount < minValidBid {
		return fmt.Errorf("bid must be at least %d (current price + minimum increment)", minValidBid)
	}
	return nil
}

// CalculateRarityModifier returns the rarity modifier for a card level
func (cv *CardValidation) CalculateRarityModifier(level int) float64 {
	if err := cv.ValidateCardLevel(level); err != nil {
		return 1.0
	}
	return math.Pow(1.5, float64(level-1))
}

// CalculateOwnershipModifier calculates the ownership modifier based on active owners
func (cv *CardValidation) CalculateOwnershipModifier(activeOwners int) float64 {
	if activeOwners < MinimumActiveOwners {
		return 1.0
	}
	
	// Inverse logarithmic scaling for scarcity
	scarcityMod := 2.0 - math.Log10(float64(activeOwners)+1)/2.0
	return math.Max(1.0, scarcityMod)
}

// max is a helper function to find the maximum of two int64 values
func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}