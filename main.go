package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/commands"
	"github.com/disgoorg/bot-template/bottemplate/components"
	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/economy/auction"
	"github.com/disgoorg/bot-template/bottemplate/economy/claim"
	"github.com/disgoorg/bot-template/bottemplate/handlers"
	"github.com/disgoorg/bot-template/bottemplate/logger"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/handler"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	// Initialize custom logger
	customHandler := logger.NewHandler()
	slog.SetDefault(slog.New(customHandler))

	slog.Info("Starting GoHYE Discord Bot",
		slog.String("version", version),
		slog.String("commit", commit))

	shouldSyncCommands := flag.Bool("sync-commands", false, "Whether to sync commands to discord")
	shouldCalculatePrices := flag.Bool("calculate-prices", false, "Whether to calculate prices on startup")
	path := flag.String("config", "config.toml", "path to config")
	flag.Parse()

	cfg, err := bottemplate.LoadConfig(*path)
	if err != nil {
		slog.Error("Failed to load configuration", slog.Any("error", err))
		os.Exit(-1)
	}
	slog.Info("Configuration loaded successfully")

	slog.Info("Initializing database connection...")
	dbStartTime := time.Now()

	// Create context with longer timeout for database connection and initial setup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Convert bottemplate.DBConfig to database.DBConfig
	dbConfig := database.DBConfig{
		Host:     cfg.DB.Host,
		Port:     cfg.DB.Port,
		User:     cfg.DB.User,
		Password: cfg.DB.Password,
		Database: cfg.DB.Database,
		PoolSize: cfg.DB.PoolSize,
	}

	db, err := database.New(ctx, dbConfig)
	if err != nil {
		slog.Error("Database connection failed",
			slog.String("error", err.Error()),
			slog.Duration("attempted_for", time.Since(dbStartTime)))
		os.Exit(-1)
	}

	slog.Info("Database connected successfully",
		slog.String("database", cfg.DB.Database),
		slog.Duration("took", time.Since(dbStartTime)))

	// Add automatic schema initialization
	slog.Info("Initializing database schema...")
	if err := db.InitializeSchema(ctx); err != nil {
		slog.Error("Failed to initialize database schema",
			slog.String("error", err.Error()),
			slog.Duration("attempted_for", time.Since(dbStartTime)))
		os.Exit(-1)
	}
	slog.Info("Database schema initialized successfully")

	defer db.Close()

	b := bottemplate.New(*cfg, version, commit)
	b.DB = db

	// Initialize Spaces service
	spacesService := services.NewSpacesService(
		cfg.Spaces.Key,
		cfg.Spaces.Secret,
		cfg.Spaces.Region,
		cfg.Spaces.Bucket,
		cfg.Spaces.CardRoot, // Add this parameter
	)
	b.SpacesService = spacesService

	//Initialize Auction Manager
	auctionRepo := repositories.NewAuctionRepository(b.DB.BunDB()) // Create the repository
	b.AuctionManager = auction.NewManager(auctionRepo)             // Create the manager
	b.AuctionManager.SetClient(b.Client)

	// Initialize repositories
	b.UserRepository = repositories.NewUserRepository(b.DB.BunDB())
	b.UserCardRepository = repositories.NewUserCardRepository(b.DB.BunDB())
	b.CardRepository = repositories.NewCardRepository(b.DB.BunDB(), spacesService)
	b.ClaimRepository = repositories.NewClaimRepository(b.DB.BunDB())

	// Update the price calculator initialization with better configured values
	priceCalc := economy.NewPriceCalculator(db, economy.PricingConfig{
		BasePrice:       1000, // Base price for level 1 cards
		LevelMultiplier: 1.5,  // 50% increase per level
		ScarcityWeight:  0.8,  // Weight for scarcity impact

		ActivityWeight:      0.5,     // Weight for activity impact
		MinPrice:            100,     // Absolute minimum price
		MaxPrice:            1000000, // Absolute maximum price
		MinActiveOwners:     3,       // Minimum active owners for price calculation
		MinTotalCopies:      1,       // Minimum total copies for price calculation
		BaseMultiplier:      1000,    // Base price multiplier
		ScarcityImpact:      0.01,    // 1% price reduction per copy
		DistributionImpact:  0.05,    // 5% impact for distribution
		HoardingThreshold:   0.2,     // 20% of supply triggers hoarding
		HoardingImpact:      0.1,     // 10% price increase for hoarding
		ActivityImpact:      0.05,    // 5% impact for activity
		OwnershipImpact:     0.01,    // 1% impact per owner
		RarityMultiplier:    0.5,     // 50% increase per rarity level
		PriceUpdateInterval: 1 * time.Hour,
		InactivityThreshold: 7 * 24 * time.Hour, // 7 days for inactivity
		CacheExpiration:     15 * time.Minute,
	})

	// Initialize prices if needed with a longer timeout
	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	if err := priceCalc.InitializeCardPrices(initCtx); err != nil {
		initCancel()
		slog.Error("Failed to initialize card prices",
			slog.String("error", err.Error()))
		os.Exit(-1)
	}
	initCancel()

	// Schedule price updates with a separate context
	updateCtx, updateCancel := context.WithCancel(context.Background())
	defer updateCancel()

	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Create a new context for each update
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
				if err := priceCalc.UpdateAllPrices(ctx); err != nil {
					slog.Error("Failed to update prices",
						slog.String("error", err.Error()))
				}
				cancel()
			case <-updateCtx.Done():
				return
			}
		}
	}()

	// Force initial price calculation for all cards
	if *shouldCalculatePrices {
		slog.Info("Performing initial price calculation for all cards...")
		if err := priceCalc.UpdateAllPrices(ctx); err != nil {
			slog.Error("Failed to calculate initial prices",
				slog.String("error", err.Error()))
			os.Exit(-1)
		}
	}

	slog.Info("Card market system initialized successfully",
		slog.String("component", "price_calculator"),
		slog.String("status", "success"))

	// Add price calculator to bot
	b.PriceCalculator = priceCalc

	// Initialize ClaimManager
	b.ClaimManager = claim.NewManager(time.Second * 5) // 5 second cooldown between claims
	b.ClaimManager.StartCleanupRoutine(context.Background())

	h := handler.New()

	// Group related command handlers
	// System commands
	h.Command("/version", commands.VersionHandler(b))
	h.Command("/test", handlers.WrapWithLogging("test", commands.TestHandler))
	h.Autocomplete("/test", commands.TestAutocompleteHandler)
	h.Component("/test-button", components.TestComponent)

	// Database/Admin commands
	h.Command("/dbtest", handlers.WrapWithLogging("dbtest", commands.DBTestHandler(b)))
	h.Command("/deletecard", handlers.WrapWithLogging("deletecard", commands.DeleteCardHandler(b)))
	h.Command("/init", handlers.WrapWithLogging("init", commands.InitHandler(b)))

	// Card-related commands
	h.Command("/summon", handlers.WrapWithLogging("summon", commands.SummonHandler(b)))
	h.Command("/searchcards", handlers.WrapWithLogging("searchcards", commands.SearchCardsHandler(b)))
	h.Command("/cards", handlers.WrapWithLogging("cards", commands.CardsHandler(b)))
	h.Command("/price-stats", handlers.WrapWithLogging("price-stats", commands.PriceStatsHandler(b)))
	h.Component("/details/", handlers.WrapComponentWithLogging("price-details", commands.PriceDetailsHandler(b)))
	h.Component("/claim/", handlers.WrapComponentWithLogging("claim", commands.ClaimButtonHandler(b)))
	h.Command("/metrics", handlers.WrapWithLogging("metrics", commands.MetricsHandler(b)))
	h.Command("/claim", handlers.WrapWithLogging("claim", commands.ClaimHandler(b)))
	h.Command("/fixduplicates", handlers.WrapWithLogging("fixduplicates", commands.FixDuplicatesHandler(b)))
	h.Command("/levelup", handlers.WrapWithLogging("levelup", commands.LevelUpHandler(b)))
	h.Command("/analyze-economy", handlers.WrapWithLogging("analyze-economy", commands.AnalyzeEconomyHandler(b)))
	h.Command("/manage-images", handlers.WrapWithLogging("manage-images", commands.ManageImagesHandler(b)))
	//User-Related Commands
	h.Command("/balance", handlers.WrapWithLogging("balance", commands.BalanceHandler(b)))
	// Auction-related commands
	auctionHandler := commands.NewAuctionHandler(b.AuctionManager, b.Client, b.CardRepository)
	auctionHandler.Register(h)

	if err = b.SetupBot(h, bot.NewListenerFunc(b.OnReady), handlers.MessageHandler(b)); err != nil {
		slog.Error("Failed to setup bot",
			slog.String("type", "sys"),
			slog.Any("error", err),
			slog.String("error_details", fmt.Sprintf("%+v", err)),
			slog.String("component", "bot_setup"),
			slog.String("status", "failed"),
		)
		os.Exit(-1)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		b.Client.Close(ctx)
	}()

	if *shouldSyncCommands {
		slog.Info("Syncing commands",
			slog.String("type", "sys"),
			slog.Any("guild_ids", cfg.Bot.DevGuilds),
		)
		if err = handler.SyncCommands(b.Client, commands.Commands, cfg.Bot.DevGuilds); err != nil {
			slog.Error("Failed to sync commands",
				slog.String("type", "sys"),
				slog.Any("error", err),
				slog.String("error_details", fmt.Sprintf("%+v", err)),
				slog.String("component", "command_sync"),
				slog.String("status", "failed"),
			)
		}
	}

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = b.Client.OpenGateway(ctx); err != nil {
		slog.Error("Failed to open gateway",
			slog.String("type", "sys"),
			slog.Any("error", err),
			slog.String("error_details", fmt.Sprintf("%+v", err)),
			slog.String("component", "gateway"),
			slog.String("status", "failed"),
		)
		os.Exit(-1)
	}

	slog.Info("Bot is running. Press CTRL-C to exit.")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	<-s
	slog.Info("Shutting down bot...")
}
