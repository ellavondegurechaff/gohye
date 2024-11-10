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

	h := handler.New()
	h.Command("/test", handlers.WrapWithLogging("test", commands.TestHandler))
	h.Autocomplete("/test", commands.TestAutocompleteHandler)
	h.Command("/version", commands.VersionHandler(b))
	h.Component("/test-button", components.TestComponent)
	h.Command("/dbtest", handlers.WrapWithLogging("dbtest", commands.DBTestHandler(b)))
	h.Command("/deletecard", handlers.WrapWithLogging("deletecard", commands.DeleteCardHandler(b)))
	h.Command("/summon", handlers.WrapWithLogging("summon", commands.SummonHandler(b)))
	h.Command("/searchcards", handlers.WrapWithLogging("searchcards", commands.SearchCardsHandler(b)))
	h.Command("/init", handlers.WrapWithLogging("init", commands.InitHandler(b)))
	h.Command("/cards", handlers.WrapWithLogging("cards", commands.CardsHandler(b)))

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
