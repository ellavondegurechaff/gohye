package services

import (
	"context"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/economy/forge"
	"github.com/disgoorg/bot-template/bottemplate/economy/vials"
)

// ForgeServiceAdapter adapts the economic service to provide forge functionality
type ForgeServiceAdapter struct {
	service *EconomicService
}

// NewForgeServiceAdapter creates a forge service adapter
func NewForgeServiceAdapter(service *EconomicService) *ForgeServiceAdapter {
	return &ForgeServiceAdapter{service: service}
}

// CalculateForgeCost calculates the cost to forge two cards
func (a *ForgeServiceAdapter) CalculateForgeCost(ctx context.Context, card1, card2 *models.Card) (int64, error) {
	fm := a.service.forgeManagerPool.Get().(*forge.ForgeManager)
	defer a.service.forgeManagerPool.Put(fm)

	return fm.CalculateForgeCost(ctx, card1, card2)
}

// ForgeCards forges two cards into a new one
func (a *ForgeServiceAdapter) ForgeCards(ctx context.Context, userID int64, card1ID, card2ID int64) (*models.Card, error) {
	fm := a.service.forgeManagerPool.Get().(*forge.ForgeManager)
	defer a.service.forgeManagerPool.Put(fm)

	return fm.ForgeCards(ctx, userID, card1ID, card2ID)
}

// VialServiceAdapter adapts the economic service to provide vial functionality
type VialServiceAdapter struct {
	service *EconomicService
}

// NewVialServiceAdapter creates a vial service adapter
func NewVialServiceAdapter(service *EconomicService) *VialServiceAdapter {
	return &VialServiceAdapter{service: service}
}

// GetVials returns the current vial balance for a user
func (a *VialServiceAdapter) GetVials(ctx context.Context, userID int64) (int64, error) {
	vm := a.service.vialManagerPool.Get().(*vials.VialManager)
	defer a.service.vialManagerPool.Put(vm)

	return vm.GetVials(ctx, userID)
}

// AddVials adds vials to a user's balance
func (a *VialServiceAdapter) AddVials(ctx context.Context, userID int64, amount int64) error {
	vm := a.service.vialManagerPool.Get().(*vials.VialManager)
	defer a.service.vialManagerPool.Put(vm)

	return vm.AddVials(ctx, userID, amount)
}

// CalculateVialYield calculates how many vials a card would yield
func (a *VialServiceAdapter) CalculateVialYield(ctx context.Context, card *models.Card) (int64, error) {
	vm := a.service.vialManagerPool.Get().(*vials.VialManager)
	defer a.service.vialManagerPool.Put(vm)

	return vm.CalculateVialYield(ctx, card)
}

// LiquefyCard converts a card into vials
func (a *VialServiceAdapter) LiquefyCard(ctx context.Context, userID int64, cardNameOrID interface{}) (int64, error) {
	vm := a.service.vialManagerPool.Get().(*vials.VialManager)
	defer a.service.vialManagerPool.Put(vm)

	return vm.LiquefyCard(ctx, userID, cardNameOrID)
}

// AuctionServiceAdapter adapts the economic service to provide auction functionality
type AuctionServiceAdapter struct {
	service *EconomicService
}

// NewAuctionServiceAdapter creates an auction service adapter
func NewAuctionServiceAdapter(service *EconomicService) *AuctionServiceAdapter {
	return &AuctionServiceAdapter{service: service}
}

// CreateAuction creates a new auction
func (a *AuctionServiceAdapter) CreateAuction(ctx context.Context, cardID int64, sellerID string, startPrice int64, duration time.Duration) (*models.Auction, error) {
	return a.service.auctionManager.CreateAuction(ctx, cardID, sellerID, startPrice, duration)
}

// PlaceBid places a bid on an auction
func (a *AuctionServiceAdapter) PlaceBid(ctx context.Context, auctionID int64, bidderID string, amount int64) error {
	return a.service.auctionManager.PlaceBid(ctx, auctionID, bidderID, amount)
}

// CancelAuction cancels an auction
func (a *AuctionServiceAdapter) CancelAuction(ctx context.Context, auctionID int64, requesterID string) error {
	return a.service.auctionManager.CancelAuction(ctx, auctionID, requesterID)
}

// GetActiveAuctions returns all active auctions
func (a *AuctionServiceAdapter) GetActiveAuctions(ctx context.Context) ([]*models.Auction, error) {
	return a.service.auctionManager.GetActiveAuctions(ctx)
}

// ClaimServiceAdapter adapts the economic service to provide claim functionality
type ClaimServiceAdapter struct {
	service *EconomicService
}

// NewClaimServiceAdapter creates a claim service adapter
func NewClaimServiceAdapter(service *EconomicService) *ClaimServiceAdapter {
	return &ClaimServiceAdapter{service: service}
}

// CanClaim checks if a user can claim
func (a *ClaimServiceAdapter) CanClaim(userID string) (bool, time.Duration) {
	return a.service.claimManager.CanClaim(userID)
}

// HasActiveSession checks if user has an active claim session
func (a *ClaimServiceAdapter) HasActiveSession(userID string) bool {
	return a.service.claimManager.HasActiveSession(userID)
}

// LockClaim locks a claim for a user
func (a *ClaimServiceAdapter) LockClaim(userID string) bool {
	return a.service.claimManager.LockClaim(userID)
}

// ReleaseClaim releases a claim for a user
func (a *ClaimServiceAdapter) ReleaseClaim(userID string) {
	a.service.claimManager.ReleaseClaim(userID)
}

// SetClaimCooldown sets the claim cooldown for a user
func (a *ClaimServiceAdapter) SetClaimCooldown(userID string) {
	a.service.claimManager.SetClaimCooldown(userID)
}

// EffectsServiceAdapter adapts the economic service to provide effects functionality
type EffectsServiceAdapter struct {
	service *EconomicService
}

// NewEffectsServiceAdapter creates an effects service adapter
func NewEffectsServiceAdapter(service *EconomicService) *EffectsServiceAdapter {
	return &EffectsServiceAdapter{service: service}
}

// PurchaseEffect purchases an effect item
func (a *EffectsServiceAdapter) PurchaseEffect(ctx context.Context, userID string, effectID string) error {
	return a.service.effectsManager.PurchaseEffect(ctx, userID, effectID)
}

// ActivateEffect activates an effect from inventory
func (a *EffectsServiceAdapter) ActivateEffect(ctx context.Context, userID string, effectID string) error {
	return a.service.effectsManager.ActivateEffect(ctx, userID, effectID)
}

// ListEffectItems returns all available effect items
func (a *EffectsServiceAdapter) ListEffectItems(ctx context.Context) ([]*models.EffectItem, error) {
	return a.service.effectsManager.ListEffectItems(ctx)
}

// GetEffectItem returns a specific effect item by ID
func (a *EffectsServiceAdapter) GetEffectItem(ctx context.Context, effectID string) (*models.EffectItem, error) {
	return a.service.effectsManager.GetEffectItem(ctx, effectID)
}

// ListUserEffects returns all effects in user's inventory with their details
func (a *EffectsServiceAdapter) ListUserEffects(ctx context.Context, userID string) ([]*models.EffectItem, error) {
	return a.service.effectsManager.ListUserEffects(ctx, userID)
}

// PricingServiceAdapter adapts the economic service to provide pricing functionality
type PricingServiceAdapter struct {
	service *EconomicService
}

// NewPricingServiceAdapter creates a pricing service adapter
func NewPricingServiceAdapter(service *EconomicService) *PricingServiceAdapter {
	return &PricingServiceAdapter{service: service}
}

// CalculateCardPrice calculates the price for a card
func (a *PricingServiceAdapter) CalculateCardPrice(ctx context.Context, cardID int64) (int64, error) {
	return a.service.priceCalculator.CalculateCardPrice(ctx, cardID)
}

// UpdateCardPrice updates the price for a card
func (a *PricingServiceAdapter) UpdateCardPrice(ctx context.Context, cardID int64) error {
	return a.service.priceCalculator.UpdateCardPrice(ctx, cardID)
}

// GetPriceHistory returns price history for a card
func (a *PricingServiceAdapter) GetPriceHistory(ctx context.Context, cardID int64, days int) ([]economy.PricePoint, error) {
	return a.service.priceCalculator.GetPriceHistory(ctx, cardID, days)
}

// GetLatestPrice returns the latest price for a card
func (a *PricingServiceAdapter) GetLatestPrice(ctx context.Context, cardID int64) (int64, error) {
	return a.service.priceCalculator.GetLatestPrice(ctx, cardID)
}

// UpdateAllPrices updates prices for all cards
func (a *PricingServiceAdapter) UpdateAllPrices(ctx context.Context) error {
	return a.service.priceCalculator.UpdateAllPrices(ctx)
}

// GetMarketStats returns market statistics for a card
func (a *PricingServiceAdapter) GetMarketStats(ctx context.Context, cardID int64, currentPrice int64) (*economy.MarketStats, error) {
	return a.service.priceCalculator.GetMarketStats(ctx, cardID, currentPrice)
}

// EconomyMonitorAdapter adapts the economic service to provide monitoring functionality
type EconomyMonitorAdapter struct {
	service *EconomicService
}

// NewEconomyMonitorAdapter creates an economy monitor adapter
func NewEconomyMonitorAdapter(service *EconomicService) *EconomyMonitorAdapter {
	return &EconomyMonitorAdapter{service: service}
}

// CollectStats gathers current economic statistics
func (a *EconomyMonitorAdapter) CollectStats(ctx context.Context) (*models.EconomyStats, error) {
	return a.service.economyMonitor.CollectStats(ctx)
}

// RunMonitoringCycle executes a single monitoring cycle
func (a *EconomyMonitorAdapter) RunMonitoringCycle(ctx context.Context) error {
	return a.service.economyMonitor.RunMonitoringCycle(ctx)
}

// Start starts the monitoring service
func (a *EconomyMonitorAdapter) Start(ctx context.Context) {
	a.service.economyMonitor.Start(ctx)
}