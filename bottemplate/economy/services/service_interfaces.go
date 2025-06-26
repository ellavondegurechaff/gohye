package services

import (
	"context"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/economy/auction"
	"github.com/disgoorg/bot-template/bottemplate/economy/claim"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
)

// EconomicServiceInterface defines the contract for economic service operations
type EconomicServiceInterface interface {
	// Manager access (for backward compatibility)
	GetAuctionManager() *auction.Manager
	GetClaimManager() *claim.Manager
	GetEffectsManager() *effects.Manager
	GetPriceCalculator() *economy.PriceCalculator
	GetEconomyMonitor() *economy.EconomyMonitor

	// Coordinated operations
	ExecuteForgeOperation(ctx context.Context, userID string, card1ID, card2ID int64) (*TransactionResult, error)
	ExecuteVialOperation(ctx context.Context, userID string, cardNameOrID interface{}) (*TransactionResult, error)

	// Validation services
	ValidateUserBalance(ctx context.Context, userID string, requiredAmount int64) error
	ValidateCardOwnership(ctx context.Context, userID string, cardID int64) error

	// Economic health
	GetEconomicHealth(ctx context.Context) (*models.EconomyStats, error)

	// Pricing services
	CalculateCardPrice(ctx context.Context, cardID int64) (int64, error)
	GetLatestPrice(ctx context.Context, cardID int64) (int64, error)
	InitializePrices(ctx context.Context) error

	// Lifecycle management
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error

	// Monitoring
	GetActiveOperations() map[string]*OperationContext
}

// ForgeServiceInterface defines forge-specific operations
type ForgeServiceInterface interface {
	CalculateForgeCost(ctx context.Context, card1, card2 *models.Card) (int64, error)
	ForgeCards(ctx context.Context, userID int64, card1ID, card2ID int64) (*models.Card, error)
}

// VialServiceInterface defines vial-specific operations
type VialServiceInterface interface {
	GetVials(ctx context.Context, userID int64) (int64, error)
	AddVials(ctx context.Context, userID int64, amount int64) error
	CalculateVialYield(ctx context.Context, card *models.Card) (int64, error)
	LiquefyCard(ctx context.Context, userID int64, cardNameOrID interface{}) (int64, error)
}

// AuctionServiceInterface defines auction-specific operations
type AuctionServiceInterface interface {
	CreateAuction(ctx context.Context, cardID int64, sellerID string, startPrice int64, duration time.Duration) (*models.Auction, error)
	PlaceBid(ctx context.Context, auctionID int64, bidderID string, amount int64) error
	CancelAuction(ctx context.Context, auctionID int64, requesterID string) error
	GetActiveAuctions(ctx context.Context) ([]*models.Auction, error)
}

// ClaimServiceInterface defines claim-specific operations
type ClaimServiceInterface interface {
	CanClaim(userID string) (bool, time.Duration)
	HasActiveSession(userID string) bool
	LockClaim(userID string) bool
	ReleaseClaim(userID string)
	SetClaimCooldown(userID string)
}

// EffectsServiceInterface defines effects-specific operations
type EffectsServiceInterface interface {
	PurchaseEffect(ctx context.Context, userID string, effectID string) error
	ActivateEffect(ctx context.Context, userID string, effectID string) error
	ListEffectItems(ctx context.Context) ([]*models.EffectItem, error)
	GetEffectItem(ctx context.Context, effectID string) (*models.EffectItem, error)
	ListUserEffects(ctx context.Context, userID string) ([]*models.EffectItem, error)
}

// PricingServiceInterface defines pricing-specific operations
type PricingServiceInterface interface {
	CalculateCardPrice(ctx context.Context, cardID int64) (int64, error)
	UpdateCardPrice(ctx context.Context, cardID int64) error
	GetPriceHistory(ctx context.Context, cardID int64, days int) ([]economy.PricePoint, error)
	GetLatestPrice(ctx context.Context, cardID int64) (int64, error)
	UpdateAllPrices(ctx context.Context) error
	GetMarketStats(ctx context.Context, cardID int64, currentPrice int64) (*economy.MarketStats, error)
}

// EconomyMonitorInterface defines monitoring operations
type EconomyMonitorInterface interface {
	CollectStats(ctx context.Context) (*models.EconomyStats, error)
	RunMonitoringCycle(ctx context.Context) error
	Start(ctx context.Context)
}