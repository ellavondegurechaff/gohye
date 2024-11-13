package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy/auction"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var AuctionCommand = discord.SlashCommandCreate{
	Name:        "auction",
	Description: "Auction related commands",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommand{
			Name:        "create",
			Description: "Create a new auction",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionInt{
					Name:        "card_id",
					Description: "The ID of the card to auction",
					Required:    true,
				},
				discord.ApplicationCommandOptionInt{
					Name:        "start_price",
					Description: "Starting price for the auction",
					Required:    true,
					MinValue:    intPtr(100),
				},
				discord.ApplicationCommandOptionInt{
					Name:        "duration",
					Description: "Auction duration in hours (1-24)",
					Required:    true,
					MinValue:    intPtr(1),
					MaxValue:    intPtr(24),
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "bid",
			Description: "Place a bid on an auction",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "auction_id",
					Description: "The ID of the auction (e.g. 123 or ABC4)",
					Required:    true,
				},
				discord.ApplicationCommandOptionInt{
					Name:        "amount",
					Description: "Bid amount",
					Required:    true,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "list",
			Description: "List all active auctions",
		},
	},
}

type AuctionHandler struct {
	manager  *auction.Manager
	client   bot.Client
	cardRepo repositories.CardRepository
}

func NewAuctionHandler(manager *auction.Manager, client bot.Client, cardRepo repositories.CardRepository) *AuctionHandler {
	return &AuctionHandler{
		manager:  manager,
		client:   client,
		cardRepo: cardRepo,
	}
}

func (h *AuctionHandler) Register(r handler.Router) {
	r.Route("/auction", func(r handler.Router) {
		r.Command("/create", h.HandleCreate)
		r.Command("/bid", h.HandleBid)
		r.Command("/list", h.HandleList)
	})
}

func (h *AuctionHandler) HandleCreate(event *handler.CommandEvent) error {
	data := event.SlashCommandInteractionData()
	cardID := int64(data.Int("card_id"))
	startPrice := int64(data.Int("start_price"))
	duration := time.Duration(data.Int("duration")) * time.Hour

	ctx := context.Background()
	auction, err := h.manager.CreateAuction(ctx, cardID, event.User().ID.String(), startPrice, duration)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to create auction: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return event.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Successfully created auction #%d", auction.ID),
		Flags:   discord.MessageFlagEphemeral,
	})
}

func (h *AuctionHandler) HandleBid(event *handler.CommandEvent) error {
	data := event.SlashCommandInteractionData()
	auctionIDStr := data.String("auction_id")
	amount := int64(data.Int("amount"))

	ctx := context.Background()

	var auction *models.Auction
	var err error

	// Try to get auction by alphanumeric ID first
	auction, err = h.manager.GetAuctionByAuctionID(ctx, auctionIDStr)
	if err != nil {
		// If not found, try parsing as numeric ID

		numericID, parseErr := strconv.ParseInt(auctionIDStr, 10, 64)
		if parseErr == nil {
			auction, err = h.manager.GetByID(ctx, numericID)
		}

		if err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("Failed to find auction: %s", err),
				Flags:   discord.MessageFlagEphemeral,
			})
		}
	}

	err = h.manager.PlaceBid(ctx, auction.ID, event.User().ID.String(), amount)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to place bid: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return event.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Successfully placed bid of %d 💰 on auction %s", amount, auction.AuctionID),
		Flags:   discord.MessageFlagEphemeral,
	})
}

func intPtr(v int) *int {
	return &v
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (h *AuctionHandler) HandleList(event *handler.CommandEvent) error {
	ctx := context.Background()
	auctions, err := h.manager.GetActiveAuctions(ctx)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to get auctions: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if len(auctions) == 0 {
		return event.CreateMessage(discord.MessageCreate{
			Content: "No active auctions found.",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	var description strings.Builder
	description.WriteString("```ansi\n")

	for _, auction := range auctions {
		card, err := h.cardRepo.GetByID(ctx, auction.CardID)
		if err != nil {
			continue
		}

		timeLeft := time.Until(auction.EndTime).Round(time.Second)
		hours := int(timeLeft.Hours())
		minutes := int(timeLeft.Minutes()) % 60

		// Format time remaining
		var timeStr string
		if hours > 0 {
			timeStr = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			timeStr = fmt.Sprintf("%dm", minutes)
		}

		// Format auction entry using card formatter
		formattedName := utils.FormatCardName(card.Name)
		formattedCollection := utils.FormatCollectionName(card.ColID)
		stars := utils.GetStarsDisplay(card.Level)

		description.WriteString(fmt.Sprintf("### %s\n", auction.AuctionID))
		description.WriteString(fmt.Sprintf("> \x1b[32m%s\x1b[0m [%s] %s\n",
			formattedName,
			stars,
			formattedCollection))
		description.WriteString(fmt.Sprintf("> 💰 Current Bid: %d\n", auction.CurrentPrice))
		description.WriteString(fmt.Sprintf("> ⏳ Ends in: %s\n", timeStr))
		description.WriteString("\n")
	}

	description.WriteString("-------------------\n")
	description.WriteString("```")

	embed := discord.NewEmbedBuilder().
		SetTitle("Auction House").
		SetDescription(description.String()).
		SetColor(0x2b2d31).
		SetFooter(fmt.Sprintf("Total Active Auctions: %d", len(auctions)), "")

	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewPrimaryButton("◀ Previous", "auction:prev_page"),
			discord.NewPrimaryButton("Next ▶", "auction:next_page"),
			discord.NewSecondaryButton("🔄 Refresh", "auction:refresh"),
		),
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed.Build()},
		Components: components,
	})
}
