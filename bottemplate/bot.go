package bottemplate

import (
	"context"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/economy/auction"
	"github.com/disgoorg/bot-template/bottemplate/services"
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
		Cfg:       cfg,
		Paginator: paginator.New(),
		Version:   version,
		Commit:    commit,
	}
}

type Bot struct {
	Cfg                Config
	Client             bot.Client
	Paginator          *paginator.Manager
	Version            string
	Commit             string
	DB                 *database.DB
	CardRepository     repositories.CardRepository
	UserCardRepository repositories.UserCardRepository
	SpacesService      *services.SpacesService
	PriceCalculator    *economy.PriceCalculator
	AuctionManager     *auction.Manager
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
		gateway.WithListeningActivity("rewrite myself :)"),
		gateway.WithOnlineStatus(discord.OnlineStatusOnline)); err != nil {
		slog.Error("Failed to set presence", slog.Any("error", err))
	}
}
