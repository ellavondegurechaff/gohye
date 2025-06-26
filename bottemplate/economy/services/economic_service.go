package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/economy/auction"
	"github.com/disgoorg/bot-template/bottemplate/economy/claim"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/bot-template/bottemplate/economy/forge"
	"github.com/disgoorg/bot-template/bottemplate/economy/vials"
	"github.com/disgoorg/disgo/bot"
	"github.com/uptrace/bun"
)

// EconomicService coordinates all economic operations and managers
type EconomicService struct {
	// Core dependencies
	db     *database.DB
	client bot.Client

	// Repositories
	userRepo         repositories.UserRepository
	cardRepo         repositories.CardRepository
	userCardRepo     repositories.UserCardRepository
	auctionRepo      repositories.AuctionRepository
	effectRepo       repositories.EffectRepository
	economyStatsRepo repositories.EconomyStatsRepository

	// Managers
	auctionManager  *auction.Manager
	claimManager    *claim.Manager
	effectsManager  *effects.Manager
	priceCalculator *economy.PriceCalculator
	economyMonitor  *economy.EconomyMonitor

	// Per-operation managers (created on demand)
	forgeManagerPool sync.Pool
	vialManagerPool  sync.Pool

	// Configuration
	config EconomicServiceConfig

	// Coordination
	mu                sync.RWMutex
	activeOperations  map[string]*OperationContext
	operationCounter  int64
}

// EconomicServiceConfig holds configuration for the economic service
type EconomicServiceConfig struct {
	// Transaction settings
	DefaultTransactionTimeout time.Duration
	MaxRetries                int

	// Pricing settings
	PricingConfig economy.PricingConfig

	// Claim settings
	ClaimCooldownPeriod time.Duration

	// Monitoring settings
	MonitoringInterval time.Duration
}

// OperationContext tracks individual economic operations
type OperationContext struct {
	ID          string
	Type        OperationType
	UserID      string
	StartTime   time.Time
	Timeout     time.Duration
	Transaction *bun.Tx
	mu          sync.Mutex
}

// OperationType defines types of economic operations
type OperationType string

const (
	OperationTypeAuction OperationType = "auction"
	OperationTypeForge   OperationType = "forge"
	OperationTypeVials   OperationType = "vials"
	OperationTypeEffect  OperationType = "effect"
	OperationTypeClaim   OperationType = "claim"
)

// TransactionResult represents the result of an economic transaction
type TransactionResult struct {
	Success   bool
	Message   string
	Data      interface{}
	Error     error
	Timestamp time.Time
}

// NewEconomicService creates a new coordinated economic service
func NewEconomicService(
	db *database.DB,
	client bot.Client,
	userRepo repositories.UserRepository,
	cardRepo repositories.CardRepository,
	userCardRepo repositories.UserCardRepository,
	auctionRepo repositories.AuctionRepository,
	effectRepo repositories.EffectRepository,
	economyStatsRepo repositories.EconomyStatsRepository,
	config EconomicServiceConfig,
) *EconomicService {

	service := &EconomicService{
		db:               db,
		client:           client,
		userRepo:         userRepo,
		cardRepo:         cardRepo,
		userCardRepo:     userCardRepo,
		auctionRepo:      auctionRepo,
		effectRepo:       effectRepo,
		economyStatsRepo: economyStatsRepo,
		config:           config,
		activeOperations: make(map[string]*OperationContext),
	}

	// Initialize managers with service coordination
	service.initializeManagers()

	// Initialize object pools for per-operation managers
	service.initializePools()

	return service
}

// initializeManagers sets up all economic managers
func (s *EconomicService) initializeManagers() {
	// Initialize price calculator
	s.priceCalculator = economy.NewPriceCalculator(
		s.db,
		s.config.PricingConfig,
		s.economyStatsRepo,
	)

	// Initialize auction manager
	s.auctionManager = auction.NewManager(
		s.auctionRepo,
		s.userCardRepo,
		s.cardRepo,
		s.client,
	)

	// Initialize claim manager
	s.claimManager = claim.NewManager(s.config.ClaimCooldownPeriod)

	// Initialize effects manager
	s.effectsManager = effects.NewManager(s.effectRepo, s.userRepo)

	// Initialize economy monitor
	s.economyMonitor = economy.NewEconomyMonitor(
		s.economyStatsRepo,
		s.priceCalculator,
		s.userRepo,
	)
}

// initializePools sets up object pools for managers created per-operation
func (s *EconomicService) initializePools() {
	s.forgeManagerPool = sync.Pool{
		New: func() interface{} {
			return forge.NewForgeManager(s.db, s.priceCalculator)
		},
	}

	s.vialManagerPool = sync.Pool{
		New: func() interface{} {
			return vials.NewVialManager(s.db, s.priceCalculator)
		},
	}
}

// GetAuctionManager returns the auction manager (backward compatibility)
func (s *EconomicService) GetAuctionManager() *auction.Manager {
	return s.auctionManager
}

// GetClaimManager returns the claim manager (backward compatibility)
func (s *EconomicService) GetClaimManager() *claim.Manager {
	return s.claimManager
}

// GetEffectsManager returns the effects manager (backward compatibility)
func (s *EconomicService) GetEffectsManager() *effects.Manager {
	return s.effectsManager
}

// GetPriceCalculator returns the price calculator (backward compatibility)
func (s *EconomicService) GetPriceCalculator() *economy.PriceCalculator {
	return s.priceCalculator
}

// GetEconomyMonitor returns the economy monitor (backward compatibility)
func (s *EconomicService) GetEconomyMonitor() *economy.EconomyMonitor {
	return s.economyMonitor
}

// ExecuteForgeOperation executes a forge operation with coordination
func (s *EconomicService) ExecuteForgeOperation(ctx context.Context, userID string, card1ID, card2ID int64) (*TransactionResult, error) {
	opCtx, err := s.startOperation(ctx, OperationTypeForge, userID)
	if err != nil {
		return nil, err
	}
	defer s.endOperation(opCtx.ID)

	// Get forge manager from pool
	fm := s.forgeManagerPool.Get().(*forge.ForgeManager)
	defer s.forgeManagerPool.Put(fm)

	// Execute forge operation
	forgedCard, err := fm.ForgeCards(ctx, int64(parseUserID(userID)), card1ID, card2ID)
	if err != nil {
		return &TransactionResult{
			Success:   false,
			Error:     err,
			Timestamp: time.Now(),
		}, err
	}

	return &TransactionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully forged card: %s", forgedCard.Name),
		Data:      forgedCard,
		Timestamp: time.Now(),
	}, nil
}

// ExecuteVialOperation executes a vial operation with coordination
func (s *EconomicService) ExecuteVialOperation(ctx context.Context, userID string, cardNameOrID interface{}) (*TransactionResult, error) {
	opCtx, err := s.startOperation(ctx, OperationTypeVials, userID)
	if err != nil {
		return nil, err
	}
	defer s.endOperation(opCtx.ID)

	// Get vial manager from pool
	vm := s.vialManagerPool.Get().(*vials.VialManager)
	defer s.vialManagerPool.Put(vm)

	// Execute vial operation
	vialsObtained, err := vm.LiquefyCard(ctx, int64(parseUserID(userID)), cardNameOrID)
	if err != nil {
		return &TransactionResult{
			Success:   false,
			Error:     err,
			Timestamp: time.Now(),
		}, err
	}

	return &TransactionResult{
		Success:   true,
		Message:   fmt.Sprintf("Successfully liquefied card for %d vials", vialsObtained),
		Data:      vialsObtained,
		Timestamp: time.Now(),
	}, nil
}

// ValidateUserBalance checks if user has sufficient balance for an operation
func (s *EconomicService) ValidateUserBalance(ctx context.Context, userID string, requiredAmount int64) error {
	user, err := s.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Balance < requiredAmount {
		return fmt.Errorf("insufficient balance: have %d, need %d", user.Balance, requiredAmount)
	}

	return nil
}

// ValidateCardOwnership checks if user owns a specific card
func (s *EconomicService) ValidateCardOwnership(ctx context.Context, userID string, cardID int64) error {
	userCard, err := s.userCardRepo.GetByUserIDAndCardID(ctx, userID, cardID)
	if err != nil {
		return fmt.Errorf("failed to check card ownership: %w", err)
	}

	if userCard == nil || userCard.Amount <= 0 {
		return fmt.Errorf("user does not own this card")
	}

	return nil
}

// GetEconomicHealth returns current economic health metrics
func (s *EconomicService) GetEconomicHealth(ctx context.Context) (*models.EconomyStats, error) {
	return s.economyStatsRepo.GetLatest(ctx)
}

// startOperation begins a new coordinated operation
func (s *EconomicService) startOperation(ctx context.Context, opType OperationType, userID string) (*OperationContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing operations for this user
	for _, op := range s.activeOperations {
		if op.UserID == userID && time.Since(op.StartTime) < s.config.DefaultTransactionTimeout {
			return nil, fmt.Errorf("user has active %s operation", op.Type)
		}
	}

	// Create new operation context
	opID := fmt.Sprintf("%s_%s_%d", opType, userID, time.Now().UnixNano())
	opCtx := &OperationContext{
		ID:        opID,
		Type:      opType,
		UserID:    userID,
		StartTime: time.Now(),
		Timeout:   s.config.DefaultTransactionTimeout,
	}

	s.activeOperations[opID] = opCtx
	s.operationCounter++

	slog.Debug("Started economic operation",
		slog.String("operation_id", opID),
		slog.String("type", string(opType)),
		slog.String("user_id", userID))

	return opCtx, nil
}

// endOperation completes an operation and cleans up
func (s *EconomicService) endOperation(operationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if opCtx, exists := s.activeOperations[operationID]; exists {
		if opCtx.Transaction != nil {
			// Ensure transaction is closed
			opCtx.Transaction.Rollback()
		}

		slog.Debug("Ended economic operation",
			slog.String("operation_id", operationID),
			slog.String("type", string(opCtx.Type)),
			slog.Duration("duration", time.Since(opCtx.StartTime)))

		delete(s.activeOperations, operationID)
	}
}

// GetActiveOperations returns currently active operations (for monitoring)
func (s *EconomicService) GetActiveOperations() map[string]*OperationContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*OperationContext)
	for k, v := range s.activeOperations {
		result[k] = v
	}
	return result
}

// Start initializes background processes
func (s *EconomicService) Start(ctx context.Context) error {
	// Start claim manager cleanup
	s.claimManager.StartCleanupRoutine(ctx)

	// Start price calculator updates
	s.priceCalculator.StartPriceUpdateJob(ctx)

	// Start economy monitoring
	s.economyMonitor.Start(ctx)

	// Start operation cleanup
	go s.cleanupExpiredOperations(ctx)

	slog.Info("Economic service started successfully")
	return nil
}

// cleanupExpiredOperations removes expired operations
func (s *EconomicService) cleanupExpiredOperations(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			var expired []string

			for opID, opCtx := range s.activeOperations {
				if now.Sub(opCtx.StartTime) > opCtx.Timeout {
					expired = append(expired, opID)
				}
			}

			for _, opID := range expired {
				delete(s.activeOperations, opID)
			}

			if len(expired) > 0 {
				slog.Debug("Cleaned up expired operations", slog.Int("count", len(expired)))
			}
			s.mu.Unlock()
		}
	}
}

// Shutdown gracefully shuts down the economic service
func (s *EconomicService) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close any active transactions
	for _, opCtx := range s.activeOperations {
		if opCtx.Transaction != nil {
			opCtx.Transaction.Rollback()
		}
	}

	// Shutdown auction manager
	if s.auctionManager != nil {
		s.auctionManager.Shutdown()
	}

	slog.Info("Economic service shut down successfully")
	return nil
}

// Helper function to parse user ID
func parseUserID(userID string) int {
	// This is a simplified version - in real implementation you'd need proper parsing
	// For now, we'll assume the userID string can be converted appropriately
	return 0 // Placeholder - actual implementation would parse the Discord ID
}

// InitializePrices initializes card prices (for startup)
func (s *EconomicService) InitializePrices(ctx context.Context) error {
	return s.priceCalculator.InitializeCardPrices(ctx)
}

// CalculateCardPrice calculates price for a specific card
func (s *EconomicService) CalculateCardPrice(ctx context.Context, cardID int64) (int64, error) {
	return s.priceCalculator.CalculateCardPrice(ctx, cardID)
}

// GetLatestPrice gets the latest price for a card
func (s *EconomicService) GetLatestPrice(ctx context.Context, cardID int64) (int64, error) {
	return s.priceCalculator.GetLatestPrice(ctx, cardID)
}