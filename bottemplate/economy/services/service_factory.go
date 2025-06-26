package services

import (
	"context"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/disgo/bot"
)

// ServiceFactory creates and configures economic services
type ServiceFactory struct {
	config EconomicServiceConfig
}

// NewServiceFactory creates a new service factory with default configuration
func NewServiceFactory() *ServiceFactory {
	return &ServiceFactory{
		config: DefaultEconomicServiceConfig(),
	}
}

// WithConfig sets custom configuration for the service factory
func (f *ServiceFactory) WithConfig(config EconomicServiceConfig) *ServiceFactory {
	f.config = config
	return f
}

// WithPricingConfig sets custom pricing configuration
func (f *ServiceFactory) WithPricingConfig(pricingConfig economy.PricingConfig) *ServiceFactory {
	f.config.PricingConfig = pricingConfig
	return f
}

// CreateEconomicService creates a fully configured economic service
func (f *ServiceFactory) CreateEconomicService(
	db *database.DB,
	client bot.Client,
	userRepo repositories.UserRepository,
	cardRepo repositories.CardRepository,
	userCardRepo repositories.UserCardRepository,
	auctionRepo repositories.AuctionRepository,
	effectRepo repositories.EffectRepository,
	economyStatsRepo repositories.EconomyStatsRepository,
) *EconomicService {

	return NewEconomicService(
		db,
		client,
		userRepo,
		cardRepo,
		userCardRepo,
		auctionRepo,
		effectRepo,
		economyStatsRepo,
		f.config,
	)
}

// CreateServiceAdapters creates all service adapters for an economic service
func (f *ServiceFactory) CreateServiceAdapters(service *EconomicService) *ServiceAdapters {
	return &ServiceAdapters{
		Forge:          NewForgeServiceAdapter(service),
		Vials:          NewVialServiceAdapter(service),
		Auction:        NewAuctionServiceAdapter(service),
		Claim:          NewClaimServiceAdapter(service),
		Effects:        NewEffectsServiceAdapter(service),
		Pricing:        NewPricingServiceAdapter(service),
		EconomyMonitor: NewEconomyMonitorAdapter(service),
	}
}

// ServiceAdapters holds all service adapters for easy access
type ServiceAdapters struct {
	Forge          *ForgeServiceAdapter
	Vials          *VialServiceAdapter
	Auction        *AuctionServiceAdapter
	Claim          *ClaimServiceAdapter
	Effects        *EffectsServiceAdapter
	Pricing        *PricingServiceAdapter
	EconomyMonitor *EconomyMonitorAdapter
}

// InitializeFullEconomicSystem creates and initializes the complete economic system
func (f *ServiceFactory) InitializeFullEconomicSystem(
	ctx context.Context,
	db *database.DB,
	client bot.Client,
	userRepo repositories.UserRepository,
	cardRepo repositories.CardRepository,
	userCardRepo repositories.UserCardRepository,
	auctionRepo repositories.AuctionRepository,
	effectRepo repositories.EffectRepository,
	economyStatsRepo repositories.EconomyStatsRepository,
) (*EconomicService, *ServiceAdapters, error) {

	// Create the economic service
	service := f.CreateEconomicService(
		db, client, userRepo, cardRepo, userCardRepo,
		auctionRepo, effectRepo, economyStatsRepo,
	)

	// Initialize card prices
	if err := service.InitializePrices(ctx); err != nil {
		return nil, nil, err
	}

	// Start the service
	if err := service.Start(ctx); err != nil {
		return nil, nil, err
	}

	// Create service adapters
	adapters := f.CreateServiceAdapters(service)

	return service, adapters, nil
}

// DefaultEconomicServiceConfig returns a default configuration for the economic service
func DefaultEconomicServiceConfig() EconomicServiceConfig {
	return EconomicServiceConfig{
		DefaultTransactionTimeout: 30 * time.Second,
		MaxRetries:                3,
		ClaimCooldownPeriod:       5 * time.Second,
		MonitoringInterval:        15 * time.Minute,
		PricingConfig: economy.PricingConfig{
			BasePrice:           1000,
			LevelMultiplier:     1.5,
			ScarcityWeight:      0.8,
			ActivityWeight:      0.5,
			MinPrice:            100,
			MaxPrice:            1000000,
			MinActiveOwners:     3,
			MinTotalCopies:      1,
			BaseMultiplier:      1000,
			ScarcityImpact:      0.01,
			DistributionImpact:  0.05,
			HoardingThreshold:   0.2,
			HoardingImpact:      0.1,
			ActivityImpact:      0.05,
			OwnershipImpact:     0.01,
			RarityMultiplier:    0.5,
			PriceUpdateInterval: 1 * time.Hour,
			InactivityThreshold: 7 * 24 * time.Hour,
			CacheExpiration:     15 * time.Minute,
		},
	}
}

// ProductionEconomicServiceConfig returns a production-optimized configuration
func ProductionEconomicServiceConfig() EconomicServiceConfig {
	config := DefaultEconomicServiceConfig()
	
	// Production optimizations
	config.DefaultTransactionTimeout = 60 * time.Second
	config.MaxRetries = 5
	config.MonitoringInterval = 5 * time.Minute
	config.PricingConfig.PriceUpdateInterval = 6 * time.Hour
	config.PricingConfig.CacheExpiration = 30 * time.Minute
	
	return config
}

// DevelopmentEconomicServiceConfig returns a development-optimized configuration
func DevelopmentEconomicServiceConfig() EconomicServiceConfig {
	config := DefaultEconomicServiceConfig()
	
	// Development optimizations
	config.DefaultTransactionTimeout = 10 * time.Second
	config.ClaimCooldownPeriod = 1 * time.Second
	config.MonitoringInterval = 1 * time.Minute
	config.PricingConfig.PriceUpdateInterval = 10 * time.Minute
	config.PricingConfig.CacheExpiration = 5 * time.Minute
	
	return config
}