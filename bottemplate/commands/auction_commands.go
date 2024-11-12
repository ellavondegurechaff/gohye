package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

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
				discord.ApplicationCommandOptionInt{
					Name:        "auction_id",
					Description: "The ID of the auction",
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
	auctionID := int64(data.Int("auction_id"))
	amount := int64(data.Int("amount"))

	ctx := context.Background()
	err := h.manager.PlaceBid(ctx, auctionID, event.User().ID.String(), amount)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to place bid: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return event.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("Successfully placed bid of %d 💰", amount),
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
	description.WriteString("```md\n")
	description.WriteString("# Active Auctions\n\n")

	for _, auction := range auctions {
		card, err := h.cardRepo.GetByID(ctx, auction.CardID)
		if err != nil {
			continue
		}

		// Format time remaining
		timeLeft := time.Until(auction.EndTime).Round(time.Second)

		description.WriteString(fmt.Sprintf("## Auction #%d\n", auction.ID))
		description.WriteString(fmt.Sprintf("* Card: %s %s [%s]\n",
			strings.Repeat("⭐", card.Level),
			utils.FormatCardName(card.Name),
			strings.Trim(utils.FormatCollectionName(card.ColID), "[]")))
		description.WriteString(fmt.Sprintf("* Current Bid: %d 💰\n", auction.CurrentPrice))
		description.WriteString(fmt.Sprintf("* Seller: <@%s>\n", auction.SellerID))
		if auction.TopBidderID != "" {
			description.WriteString(fmt.Sprintf("* Top Bidder: <@%s>\n", auction.TopBidderID))
		}
		description.WriteString(fmt.Sprintf("* Time Left: %s\n\n", formatDuration(timeLeft)))
	}

	description.WriteString("```")

	embed := discord.NewEmbedBuilder().
		SetTitle("🏷️ Auction House").
		SetDescription(description.String()).
		SetColor(0x2B2D31).
		SetFooter(fmt.Sprintf("Total Active Auctions: %d", len(auctions)), "")

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed.Build()},
	})
}
