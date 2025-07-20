package utils

import "time"

// Pricing Constants
const (
	// Price boundaries
	MinPrice = 500     // Minimum price floor
	MaxPrice = 1000000 // Maximum price ceiling

	// Base pricing
	InitialBasePrice  = 1000 // Base price for level 1 cards
	LevelMultiplier   = 1.5  // Price multiplier per level
	ScarcityBaseValue = 100  // Base value for scarcity calculation
	ActivityBaseValue = 50   // Base value for activity calculation

	// Ownership thresholds
	MinimumActiveOwners = 1 // Minimum active owners for price calculation
	MinimumTotalCopies  = 1 // Minimum total copies for price calculation
)

// Forge Constants
const (
	ForgeCostMultiplier = 0.15 // 15% of combined card value
	MinForgeCost        = 1000 // Minimum forge cost
)

// Vial Constants
const (
	VialRate1Star = 0.05 // 5% of card price
	VialRate2Star = 0.08 // 8% of card price
	VialRate3Star = 0.12 // 12% of card price
)

// Auction Constants
const (
	MinBidIncrement = 100              // Minimum bid increment
	MaxAuctionTime  = 24 * time.Hour   // Maximum auction duration
	MinAuctionTime  = 10 * time.Second // Minimum auction duration
	AntiSnipeTime   = 10 * time.Second // Anti-snipe extension time
	AuctionIDLength = 6                // Length of auction ID
	MaxRetries      = 5                // Maximum retries for operations
)

// Card Level Validation
const (
	MinCardLevel        = 1 // Minimum card level
	MaxCardLevel        = 5 // Maximum card level
	MaxLiquefyCardLevel = 3 // Maximum card level that can be liquefied
)

// Transaction Constants
const (
	DefaultTxTimeout        = 30 * time.Second       // Default transaction timeout
	CleanupInterval         = 15 * time.Second       // Cleanup ticker interval
	NotificationDelay       = 100 * time.Millisecond // Delay between notifications
	UserInactivityThreshold = 30 * 24 * time.Hour    // 30 days user inactivity threshold
)
