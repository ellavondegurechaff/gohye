package bottemplate

import (
	"context"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/economy/auction"
	"github.com/disgoorg/bot-template/bottemplate/economy/claim"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/paginator"
)

func New(cfg Config, version string, commit string) *Bot {
	return &Bot{
		Cfg:                      cfg,
		Paginator:                paginator.New(),
		Version:                  version,
		Commit:                   commit,
		StartTime:                time.Now(),
		BackgroundProcessManager: utils.NewBackgroundProcessManager(),
	}
}

type Bot struct {
	Cfg       Config
	Client    bot.Client
	Paginator *paginator.Manager

	// Modern effect system
	EffectManager    *effects.Manager
	EffectIntegrator *effects.GameIntegrator

	Version                  string
	Commit                   string
	DB                       *database.DB
	UserRepository           repositories.UserRepository
	CardRepository           repositories.CardRepository
	CollectionRepository     repositories.CollectionRepository
	UserCardRepository       repositories.UserCardRepository
	SpacesService            *services.SpacesService
	PriceCalculator          *economy.PriceCalculator
	AuctionManager           *auction.Manager
	ClaimManager             *claim.Manager
	ClaimRepository          repositories.ClaimRepository
	EconomyStatsRepository   repositories.EconomyStatsRepository
	StartTime                time.Time
	WishlistRepository       repositories.WishlistRepository
	BackgroundProcessManager *utils.BackgroundProcessManager
	CollectionService        *services.CollectionService
	CompletionChecker        *services.CompletionCheckerService
	ItemRepository           repositories.ItemRepository
	QuestRepository          repositories.QuestRepository
	QuestService             *services.QuestService
	QuestTracker             *services.QuestTracker
}

// GetQuestTracker returns the quest tracker instance
func (b *Bot) GetQuestTracker() *services.QuestTracker {
	return b.QuestTracker
}

func (b *Bot) SetupBot(listeners ...bot.EventListener) error {
	client, err := disgo.New(b.Cfg.Bot.Token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuilds, gateway.IntentGuildMessages, gateway.IntentMessageContent)),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagGuilds)),
		bot.WithEventListeners(b.Paginator),
		bot.WithEventListeners(listeners...),
	)
	if err != nil {
		return err
	}

	b.Client = client
	return nil
}

func (b *Bot) OnReady(_ *events.Ready) {
	slog.Info("GoHYE Bot is now ready",
		slog.String("version", b.Version),
		slog.String("commit", b.Commit))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := b.Client.SetPresence(ctx,
		gateway.WithListeningActivity("Give me will power"),
		gateway.WithOnlineStatus(discord.OnlineStatusOnline)); err != nil {
		slog.Error("Failed to set presence", slog.Any("error", err))
	}
}

// Shutdown gracefully shuts down the bot and all background processes
func (b *Bot) Shutdown(ctx context.Context) error {
	slog.Info("Initiating bot shutdown...")

	// Stop all background processes first
	if err := b.BackgroundProcessManager.Shutdown(10 * time.Second); err != nil {
		slog.Error("Failed to shutdown background processes", slog.Any("error", err))
	}

	// Shutdown auction manager if it exists
	if b.AuctionManager != nil {
		b.AuctionManager.Shutdown()
	}

	// Close the Discord client
	if b.Client != nil {
		b.Client.Close(ctx)
		slog.Info("Discord client closed")
	}

	// Close database connection
	if b.DB != nil {
		b.DB.Close()
	}

	slog.Info("Bot shutdown completed")
	return nil
}
