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
	"github.com/disgoorg/bot-template/bottemplate/commands/admin"
	"github.com/disgoorg/bot-template/bottemplate/commands/cards"
	economyCommands "github.com/disgoorg/bot-template/bottemplate/commands/economy"
	"github.com/disgoorg/bot-template/bottemplate/commands/social"
	"github.com/disgoorg/bot-template/bottemplate/commands/system"
	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/economy/auction"
	"github.com/disgoorg/bot-template/bottemplate/economy/claim"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	effectsHandlers "github.com/disgoorg/bot-template/bottemplate/economy/effects/handlers"
	"github.com/disgoorg/bot-template/bottemplate/handlers"
	"github.com/disgoorg/bot-template/bottemplate/logger"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/handler"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	// Initialize custom logger with service name
	customHandler := logger.NewHandler("GoHYE")
	slog.SetDefault(slog.New(customHandler))

	slog.Info("Starting Discord Bot",
		slog.String("version", version),
		slog.String("commit", commit),
		slog.String("type", "sys"))

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

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

	// Initialize database schema
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
		cfg.Spaces.CardRoot,
	)
	b.SpacesService = spacesService

	// Initialize repositories first
	b.UserRepository = repositories.NewUserRepository(b.DB.BunDB())
	b.UserCardRepository = repositories.NewUserCardRepository(b.DB.BunDB())
	b.CardRepository = repositories.NewCardRepository(b.DB.BunDB())
	b.ClaimRepository = repositories.NewClaimRepository(b.DB.BunDB())
	b.CollectionRepository = repositories.NewCollectionRepository(b.DB.BunDB())
	b.EconomyStatsRepository = repositories.NewEconomyStatsRepository(b.DB.BunDB())
	b.WishlistRepository = repositories.NewWishlistRepository(b.DB.BunDB())
	b.ItemRepository = repositories.NewItemRepository(b.DB.BunDB())

	// Initialize collection cache for promo filtering
	collections, err := b.CollectionRepository.GetAll(ctx)
	if err != nil {
		slog.Error("Failed to load collections for cache",
			slog.String("error", err.Error()))
		os.Exit(-1)
	}
	utils.InitializeCollectionInfo(collections)
	slog.Info("Collection cache initialized successfully",
		slog.Int("collections_loaded", len(collections)))

	// Initialize Collection Service
	b.CollectionService = services.NewCollectionService(
		b.CollectionRepository,
		b.CardRepository,
		b.UserCardRepository,
	)

	// Initialize Completion Checker Service
	b.CompletionChecker = services.NewCompletionCheckerService(
		b.Client,
		b.CollectionService,
		b.UserRepository,
		b.CardRepository,
		b.UserCardRepository,
		b.CollectionRepository,
	)

	// Then initialize Auction Manager with all required dependencies
	// auctionRepo := repositories.NewAuctionRepository(b.DB.BunDB())
	// auctionManager := auction.NewManager(
	// 	auctionRepo,
	// 	b.UserCardRepository,
	// 	b.Client,
	// )

	priceCalc := economy.NewPriceCalculator(
		db,
		economy.PricingConfig{
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
		b.EconomyStatsRepository,
	)

	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	if err := priceCalc.InitializeCardPrices(initCtx); err != nil {
		initCancel()
		slog.Error("Failed to initialize card prices",
			slog.String("error", err.Error()))
		os.Exit(-1)
	}
	initCancel()

	// Start price update process using background process manager
	b.BackgroundProcessManager.StartProcess("price-updater", "Updates card prices every 6 hours", func(ctx context.Context) {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				updateCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
				if err := priceCalc.UpdateAllPrices(updateCtx); err != nil {
					slog.Error("Failed to update prices",
						slog.String("error", err.Error()))
				}
				cancel()
			case <-ctx.Done():
				return
			}
		}
	})

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

	b.PriceCalculator = priceCalc

	b.ClaimManager = claim.NewManager(time.Second * 5)
	
	// Start claim cleanup process using background process manager
	b.BackgroundProcessManager.StartProcess("claim-cleanup", "Cleans up expired claim sessions", func(ctx context.Context) {
		b.ClaimManager.StartCleanupRoutine(ctx)
	})

	h := handler.New()

	// System commands
	h.Command("/version", system.VersionHandler(b))

	// Database/Admin commands
	h.Command("/dbtest", handlers.WrapWithLogging("dbtest", admin.DBTestHandler(b)))
	h.Command("/deletecard", handlers.WrapWithLogging("deletecard", admin.DeleteCardHandler(b)))
	h.Command("/init", handlers.WrapWithLogging("init", admin.InitHandler(b)))
	h.Command("/gift", handlers.WrapWithLogging("gift", admin.GiftHandler(b)))
	h.Command("/reset-daily", handlers.WrapWithLogging("reset-daily", admin.ResetDailyHandler(b)))

	// Card-related commands
	h.Command("/summon", handlers.WrapWithLogging("summon", cards.SummonHandler(b)))
	h.Command("/searchcards", handlers.WrapWithLogging("searchcards", cards.SearchCardsHandler(b)))
	h.Command("/cards", handlers.WrapWithLogging("cards", cards.CardsHandler(b)))
	h.Command("/price-stats", handlers.WrapWithLogging("price-stats", economyCommands.PriceStatsHandler(b)))
	h.Component("/details/", handlers.WrapComponentWithLogging("price-details", economyCommands.PriceDetailsHandler(b)))
	// h.Component("/claim/", handlers.WrapComponentWithLogging("claim", cards.ClaimButtonHandler(b)))
	h.Command("/metrics", handlers.WrapWithLogging("metrics", system.MetricsHandler(b)))
	h.Command("/claim", handlers.WrapWithLogging("claim", cards.NewClaimHandler(b).HandleCommand))
	h.Command("/fixduplicates", handlers.WrapWithLogging("fixduplicates", admin.FixDuplicatesHandler(b)))
	h.Command("/levelup", handlers.WrapWithLogging("levelup", cards.LevelUpHandler(b)))
	h.Command("/analyze-economy", handlers.WrapWithLogging("analyze-economy", admin.AnalyzeEconomyHandler(b)))
	h.Command("/manage-images", handlers.WrapWithLogging("manage-images", admin.ManageImagesHandler(b)))
	h.Autocomplete("/manage-images", admin.ManageImagesAutocomplete(b))

	// User-Related Commands
	h.Command("/balance", handlers.WrapWithLogging("balance", economyCommands.BalanceHandler(b)))
	h.Command("/daily", handlers.WrapWithLogging("daily", economyCommands.DailyHandler(b)))
	h.Command("/wish", handlers.WrapWithLogging("wish", social.WishHandler(b)))
	h.Command("/has", handlers.WrapWithLogging("has", social.HasHandler(b)))
	h.Command("/miss", handlers.WrapWithLogging("miss", social.MissHandler(b)))
	h.Command("/diff", handlers.WrapWithLogging("diff", social.DiffHandler(b)))

	// Vial Related Commands
	h.Command("/liquefy", handlers.WrapWithLogging("liquefy", economyCommands.NewLiquefyHandler(b).HandleLiquefy))
	h.Component("/liquefy/", handlers.WrapComponentWithLogging("liquefy", economyCommands.NewLiquefyHandler(b).HandleComponent))

	// Forge Related Commands
	h.Command("/forge", handlers.WrapWithLogging("forge", cards.NewForgeHandler(b).HandleForge))
	h.Component("/forge/", handlers.WrapComponentWithLogging("forge", cards.NewForgeHandler(b).HandleComponent))

	// Work Related Commands
	workHandler := economyCommands.NewWorkHandler(b)
	h.Command("/work", handlers.WrapWithLogging("work", workHandler.HandleWork))
	h.Component("/work/", handlers.WrapComponentWithLogging("work", workHandler.HandleComponent))

	// Fuse Command
	fuseHandler := economyCommands.NewFuseHandler(b)
	h.Command("/fuse", handlers.WrapWithLogging("fuse", fuseHandler.Handle))
	h.Component("/fuse/", handlers.WrapComponentWithLogging("fuse", fuseHandler.HandleComponent))

	// Initialize modern effect system
	effectManager := effects.NewManager(
		repositories.NewEffectRepository(b.DB.BunDB()),
		b.UserRepository,
		b.UserCardRepository,
		b.CardRepository,
		b.CollectionRepository,
		b.DB,
	)
	b.EffectManager = effectManager

	// Initialize modern effect integrator
	b.EffectIntegrator = effects.NewGameIntegrator(effectManager)

	// Register all effect handlers inline to avoid import cycle
	deps := effectManager.GetDependencies()
	registry := effectManager.GetRegistry()

	// Register passive effects
	passiveEffects := []effects.EffectHandler{
		effectsHandlers.NewTohrugiftHandler(deps),
		effectsHandlers.NewCakedayHandler(deps),
		effectsHandlers.NewHolygrailHandler(deps),
		effectsHandlers.NewSkyfriendHandler(deps),
		effectsHandlers.NewCherryblossHandler(deps),
		effectsHandlers.NewRulerjeanneHandler(deps),
		effectsHandlers.NewSpellcardHandler(deps),
		effectsHandlers.NewWalpurgisnightHandler(deps),
	}

	for _, effect := range passiveEffects {
		if err := effectManager.RegisterEffect(effect); err != nil {
			slog.Error("Failed to register passive effect", slog.String("effect", effect.GetMetadata().ID), slog.Any("error", err))
			os.Exit(-1)
		}
	}

	// Register active effects
	activeEffects := []effects.EffectHandler{
		effectsHandlers.NewClaimRecallHandler(deps),
		effectsHandlers.NewSpaceUnityHandler(deps),
		effectsHandlers.NewJudgeDayHandler(deps, registry),
		effectsHandlers.NewEnayanoHandler(deps),
		effectsHandlers.NewPbocchiHandler(deps),
	}

	for _, effect := range activeEffects {
		if err := effectManager.RegisterEffect(effect); err != nil {
			slog.Error("Failed to register active effect", slog.String("effect", effect.GetMetadata().ID), slog.Any("error", err))
			os.Exit(-1)
		}
	}

	slog.Info("All effects registered successfully",
		slog.Int("total_effects", len(passiveEffects)+len(activeEffects)),
		slog.Int("passive_effects", len(passiveEffects)),
		slog.Int("active_effects", len(activeEffects)))

	slog.Info("Modern effect system initialized successfully",
		slog.String("system", "modern_v2"),
		slog.String("component", "effect_manager"))

	// Shop commands
	shopHandler := economyCommands.NewShopHandler(b, effectManager)
	h.Command("/shop", handlers.WrapWithLogging("shop", shopHandler.Handle))
	h.Component("/shop_category", handlers.WrapComponentWithLogging("shop_category", shopHandler.HandleComponent))
	h.Component("/shop_item", handlers.WrapComponentWithLogging("shop_item", shopHandler.HandleComponent))
	h.Component("/shop_buy/", handlers.WrapComponentWithLogging("shop_buy", shopHandler.HandleComponent))

	// Inventory commands
	inventoryHandler := system.NewInventoryHandler(b, effectManager)
	h.Command("/inventory", handlers.WrapWithLogging("inventory", inventoryHandler.Handle))
	h.Component("/inventory_category", handlers.WrapComponentWithLogging("inventory_category", inventoryHandler.HandleComponent))
	h.Component("/inventory_item", handlers.WrapComponentWithLogging("inventory_item", inventoryHandler.HandleComponent))

	// Use Effect commands
	useEffectHandler := system.NewUseEffectHandler(b, effectManager)
	h.Command("/use-effect", handlers.WrapWithLogging("use-effect", useEffectHandler.Handle))

	// Craft Effect commands
	craftEffectHandler := system.NewCraftEffectHandler(b, effectManager)
	h.Command("/craft-effect", handlers.WrapWithLogging("craft-effect", craftEffectHandler.Handle))

	// Help commands
	helpHandler := system.NewHelpHandler(b)
	h.Command("/help", handlers.WrapWithLogging("help", helpHandler.Handle))
	h.Component("/help_category", handlers.WrapComponentWithLogging("help", helpHandler.HandleComponent))
	h.Component("/help_back", handlers.WrapComponentWithLogging("help", helpHandler.HandleComponent))

	// Profile command
	h.Command("/profile", handlers.WrapWithLogging("profile", system.ProfileHandler(b)))

	// Claim commands
	claimHandler := cards.NewClaimHandler(b)
	h.Command("/claim", handlers.WrapWithLogging("claim", claimHandler.HandleCommand))
	h.Component("/claim/next/", handlers.WrapComponentWithLogging("claim", claimHandler.HandleComponent))
	h.Component("/claim/prev/", handlers.WrapComponentWithLogging("claim", claimHandler.HandleComponent))
	h.Component("/claim/favorite/", handlers.WrapComponentWithLogging("claim", claimHandler.HandleComponent))

	// Add this with the other component handlers
	h.Component("/cards/", handlers.WrapComponentWithLogging("cards", cards.CardsComponentHandler(b)))
	h.Component("/miss/", handlers.WrapComponentWithLogging("miss", social.MissComponentHandler(b)))
	h.Component("/diff/", handlers.WrapComponentWithLogging("diff", social.DiffComponentHandler(b)))

	// Limited Commands
	h.Command("/limitedcards", handlers.WrapWithLogging("limitedcards", cards.LimitedCardsHandler(b)))
	h.Command("/limitedstats", handlers.WrapWithLogging("limitedstats", cards.LimitedStatsHandler(b)))

	// Collection Commands
	h.Command("/collection-list", handlers.WrapWithLogging("collection-list", cards.CollectionListHandler(b)))
	h.Command("/collection-info", handlers.WrapWithLogging("collection-info", cards.CollectionInfoHandler(b)))
	h.Command("/collection-progress", handlers.WrapWithLogging("collection-progress", cards.CollectionProgressHandler(b)))

	// Add this line with the other component handlers
	h.Component("/limitedstats/", handlers.WrapComponentWithLogging("limitedstats", cards.LimitedStatsComponentHandler(b)))
	h.Component("/limitedcards/", handlers.WrapComponentWithLogging("limitedcards", cards.LimitedCardsComponentHandler(b)))
	h.Component("/collection-list/", handlers.WrapComponentWithLogging("collection-list", cards.CollectionListComponentHandler(b)))

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

	// Initialize auction manager with the now-initialized client
	auctionManager := auction.NewManager(
		repositories.NewAuctionRepository(db.BunDB()),
		repositories.NewUserCardRepository(db.BunDB()),
		b.CardRepository,
		b.Client,
	)

	// Store the auction manager in the bot instance
	b.AuctionManager = auctionManager

	// Initialize auction handler with the manager
	auctionHandler := economyCommands.NewAuctionHandler(auctionManager, b.Client, b.CardRepository)
	auctionHandler.Register(h)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		
		// Gracefully shutdown the bot and all background processes
		if err := b.Shutdown(ctx); err != nil {
			slog.Error("Error during bot shutdown", slog.Any("error", err))
		}
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
	
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Wait for shutdown signal
	sig := <-sigChan
	slog.Info("Received shutdown signal, initiating graceful shutdown...",
		slog.String("signal", sig.String()))
}
