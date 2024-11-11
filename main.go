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

	// Create context with timeout for database connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Initialize repositories
	b.CardRepository = repositories.NewCardRepository(b.DB.BunDB(), spacesService)
	b.UserCardRepository = repositories.NewUserCardRepository(b.DB.BunDB())

	// Update the price calculator initialization with better configured values
	priceCalc := economy.NewPriceCalculator(db, economy.PricingConfig{
		BaseMultiplier:      1000,    // Base price multiplier for card levels
		ScarcityImpact:      0.01,    // 1% price reduction per copy
		DistributionImpact:  0.05,    // 5% impact for distribution
		HoardingThreshold:   0.2,     // 20% of supply triggers hoarding
		HoardingImpact:      0.1,     // 10% price increase for hoarding
		ActivityImpact:      0.05,    // 5% impact for activity
		OwnershipImpact:     0.01,    // 1% impact per owner
		RarityMultiplier:    0.5,     // 50% increase per rarity level
		MinimumPrice:        100,     // Minimum card price
		MaximumPrice:        1000000, // Maximum card price
		PriceUpdateInterval: 1 * time.Hour,
		InactivityThreshold: 7 * 24 * time.Hour, // 7 days for inactivity
		CacheExpiration:     15 * time.Minute,
	})

	// Create context with timeout for initialization
	initCtx, initCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer initCancel()

	// Initialize the market history table
	slog.Info("Initializing card market system...")
	if err := priceCalc.InitializeCardPrices(initCtx); err != nil {
		slog.Error("Failed to initialize card market system",
			slog.String("error", err.Error()),
			slog.String("component", "price_calculator"),
			slog.String("status", "failed"))
		os.Exit(-1)
	}

	// Get active cards and perform initial price calculation
	activeCards, err := priceCalc.GetActiveCards(initCtx)
	if err != nil {
		slog.Error("Failed to get active cards",
			slog.String("error", err.Error()),
			slog.String("component", "price_calculator"),
			slog.String("status", "failed"))
		os.Exit(-1)
	}

	if *shouldCalculatePrices {
		slog.Info("Starting initial price calculation",
			slog.Int("active_cards", len(activeCards)),
			slog.String("component", "price_calculator"))

		// Perform initial price update for active cards
		if err := priceCalc.UpdateAllPrices(initCtx); err != nil {
			slog.Error("Failed to perform initial price calculation",
				slog.String("error", err.Error()),
				slog.String("component", "price_calculator"),
				slog.String("status", "failed"))
			os.Exit(-1)
		}
	} else {
		slog.Info("Skipping initial price calculation (use --calculate-prices to enable)")
	}

	// Start the background price update job
	priceCalc.StartPriceUpdateJob(context.Background())

	slog.Info("Card market system initialized successfully",
		slog.String("component", "price_calculator"),
		slog.String("status", "success"))

	// Add price calculator to bot
	b.PriceCalculator = priceCalc

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
