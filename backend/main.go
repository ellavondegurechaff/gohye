package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	economyutils "github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/disgoorg/bot-template/bottemplate/logger"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/backend/config"
	"github.com/disgoorg/bot-template/backend/handlers"
	"github.com/disgoorg/bot-template/backend/middleware"
	webmodels "github.com/disgoorg/bot-template/backend/models"
	webservices "github.com/disgoorg/bot-template/backend/services"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	// Parse command line flags
	configPath := "../config.toml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Initialize logger first
	customHandler := logger.NewHandler("GoHYE-Backend")
	slog.SetDefault(slog.New(customHandler))

	slog.Info("Starting GoHYE Backend API",
		slog.String("version", version),
		slog.String("commit", commit),
		slog.String("type", "backend"))

	// Load configuration
	cfg, err := bottemplate.LoadConfig(configPath)
	if err != nil {
		slog.Error("Failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Create web app configuration
	webCfg := config.NewWebAppConfig(cfg, true) // debug mode for development

	// Initialize database
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	slog.Info("Connecting to database...")
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
		slog.Error("Failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("Database connected successfully")

	// Initialize repositories
	repos := webmodels.NewRepositories(
		repositories.NewUserRepository(db.BunDB()),
		repositories.NewCardRepository(db.BunDB()),
		repositories.NewCollectionRepository(db.BunDB()),
		repositories.NewUserCardRepository(db.BunDB()),
		repositories.NewClaimRepository(db.BunDB()),
		repositories.NewAuctionRepository(db.BunDB()),
		repositories.NewEffectRepository(db.BunDB()),
		repositories.NewWishlistRepository(db.BunDB()),
		repositories.NewEconomyStatsRepository(db.BunDB()),
	)

	// Initialize services
	spacesService := services.NewSpacesService(
		cfg.Spaces.Key,
		cfg.Spaces.Secret,
		cfg.Spaces.Region,
		cfg.Spaces.Bucket,
		cfg.Spaces.CardRoot,
	)

	// Initialize transaction manager
	txManager := economyutils.NewEconomicTransactionManager(db.BunDB())

	// Initialize web services
	cardMgmtService := webservices.NewCardManagementService(repos, spacesService)
	syncMgrService := webservices.NewSyncManagerService(repos, spacesService)
	collectionImportService := webservices.NewCollectionImportService(repos.Card, repos.Collection, spacesService, txManager)
	oauthService := webservices.NewOAuthService(webCfg)
	sessionService := webservices.NewSessionService(webCfg)

	// Initialize Fiber as API-only backend
	app := fiber.New(fiber.Config{
		AppName:      "GoHYE Backend API",
		ServerHeader: "GoHYE-Backend",
		ErrorHandler: middleware.CustomErrorHandler,
	})

	// Global middleware
	app.Use(recover.New())
	app.Use(middleware.SecurityHeaders())
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // Fast compression for better performance
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000,http://localhost:8080",
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With,Cookie",
		AllowCredentials: true,
	}))
	app.Use(middleware.LoggingMiddleware())

	// Create web app instance
	webApp := &handlers.WebApp{
		Config:                    webCfg,
		DB:                        db,
		Repos:                     repos,
		SpacesService:             spacesService,
		CardMgmtService:           cardMgmtService,
		SyncMgrService:            syncMgrService,
		CollectionImportService:   collectionImportService,
		OAuthService:              oauthService,
		SessionService:            sessionService,
		Version:                   version,
		Commit:                    commit,
	}

	// Setup routes
	setupRoutes(app, webApp)

	// Start server
	address := fmt.Sprintf("%s:%d", cfg.Web.Host, cfg.Web.Port)
	slog.Info("Starting backend server", slog.String("address", address))

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := app.Listen(address); err != nil {
			slog.Error("Failed to start server", slog.String("error", err.Error()))
		}
	}()

	<-c
	slog.Info("Shutting down backend server...")

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		slog.Error("Server shutdown error", slog.String("error", err.Error()))
	}

	// Close database connection
	db.Close()

	slog.Info("Backend server shutdown complete")
}

// setupRoutes configures all application routes
func setupRoutes(app *fiber.App, webApp *handlers.WebApp) {
	// Health check endpoint
	app.Get("/health", handlers.HealthCheck(webApp))

	// Authentication routes
	auth := app.Group("/auth")
	auth.Get("/discord", handlers.DiscordOAuth(webApp))
	auth.Get("/callback", handlers.OAuthCallback(webApp))
	auth.Post("/logout", handlers.Logout(webApp))

	// Public routes (minimal - mainly for legacy support)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "GoHYE Backend API",
			"version": webApp.Version,
			"status":  "running",
		})
	})

	// Protected admin routes
	admin := app.Group("/admin")
	admin.Use(middleware.AuthRequired(webApp))
	admin.Use(middleware.AdminRequired(webApp))

	// Dashboard redirect for legacy
	admin.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("http://localhost:3000/dashboard")
	})

	// Card management routes (API)
	cards := admin.Group("/cards")
	cards.Get("/:id", handlers.CardsDetail(webApp))
	cards.Post("/", handlers.CardsCreate(webApp))
	cards.Put("/:id", handlers.CardsUpdate(webApp))
	cards.Delete("/:id", handlers.CardsDelete(webApp))
	cards.Post("/bulk", handlers.CardsBulkOperation(webApp))

	// Collection management routes (API)
	collections := admin.Group("/collections")
	collections.Get("/import", handlers.CollectionsImportPage(webApp))
	collections.Get("/:id", handlers.CollectionsDetail(webApp))
	collections.Post("/", handlers.CollectionsCreate(webApp))
	collections.Post("/import", handlers.CollectionsImport(webApp))
	collections.Put("/:id", handlers.CollectionsUpdate(webApp))
	collections.Delete("/:id", handlers.CollectionsDelete(webApp))

	// Sync management routes (API)
	sync := admin.Group("/sync")
	sync.Get("/status", handlers.SyncStatus(webApp))
	sync.Post("/fix", handlers.SyncFix(webApp))
	sync.Post("/cleanup", handlers.SyncCleanup(webApp))

	// User management routes (API)
	users := admin.Group("/users")
	users.Get("/:id", handlers.UsersDetail(webApp))

	// API routes for Next.js frontend
	api := admin.Group("/api")
	api.Get("/cards", handlers.CardsAPI(webApp))
	api.Get("/collections", handlers.CollectionsAPI(webApp))
	api.Get("/collections/:id/cards", handlers.CollectionCardsAPI(webApp))
	api.Post("/upload", handlers.UploadAPI(webApp))
	api.Get("/progress/:id", handlers.ProgressAPI(webApp))
	api.Get("/dashboard/stats", handlers.DashboardStatsAPI(webApp))
	api.Get("/activity", handlers.ActivityAPI(webApp))

	// Session validation endpoint for Next.js frontend
	app.Get("/api/auth/validate", handlers.ValidateSession(webApp))

	// Global error handler for unhandled routes
	app.Use(func(c *fiber.Ctx) error {
		// If we get here, no route matched
		slog.Warn("No route matched for request",
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.String("ip", c.IP()),
		)
		return c.Status(404).JSON(fiber.Map{
			"error": "Not Found",
			"message": "The requested endpoint does not exist",
		})
	})
}