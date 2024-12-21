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
				discord.ApplicationCommandOptionString{
					Name:        "card_name",
					Description: "The name of the card to auction",
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
					Description: "Auction duration in seconds (min 10 seconds, max 24 hours)",
					Required:    true,
					MinValue:    intPtr(10),
					MaxValue:    intPtr(86400),
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

	// Component patterns must start with /
	r.Component("/auction/confirm", h.HandleConfirmation)
	r.Component("/auction/cancel", h.HandleCancel)
}

func (h *AuctionHandler) HandleCreate(event *handler.CommandEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data := event.SlashCommandInteractionData()
	cardName := data.String("card_name")
	startPrice := int64(data.Int("start_price"))
	duration := time.Duration(data.Int("duration")) * time.Second

	// Get user's matching card using the weighted search
	userCard, err := h.manager.GetUserCardByName(ctx, event.User().ID.String(), cardName)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Get card details
	card, err := h.cardRepo.GetByID(ctx, userCard.CardID)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Content: "Failed to get card details",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Create confirmation embed
	embed := discord.NewEmbedBuilder().
		SetTitle("🏛️ Confirm Auction Creation").
		SetDescription(fmt.Sprintf("Please confirm that you want to create an auction for **%s**", card.Name)).
		AddField("Card", fmt.Sprintf("%s %s", strings.Repeat("★", card.Level), card.Name), false).
		AddField("Start Price", fmt.Sprintf("%d 💰", startPrice), true).
		AddField("Duration", formatDuration(duration), true).
		AddField("Collection", strings.ToUpper(card.ColID), true).
		SetColor(0x2b2d31).
		SetFooter("This auction will be visible to all users", "").
		Build()

	// Create confirmation buttons
	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewSuccessButton(
				"Confirm",
				fmt.Sprintf("/auction/confirm/%d/%d/%d", card.ID, startPrice, int64(duration.Seconds())),
			),
			discord.NewDangerButton(
				"Cancel",
				"/auction/cancel",
			),
		),
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: components,
		Flags:      discord.MessageFlagEphemeral,
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
		// Don't reveal the actual bid amount in the error message
		if strings.Contains(err.Error(), "bid must be at least") {
			return event.CreateMessage(discord.MessageCreate{
				Content: "Your bid was too low. Try a higher amount!",
				Flags:   discord.MessageFlagEphemeral,
			})
		}
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
		timeStr := formatDuration(timeLeft)

		// Format card name by capitalizing each word
		words := strings.Split(strings.ReplaceAll(card.Name, "_", " "), " ")
		for i, word := range words {
			words[i] = strings.Title(strings.ToLower(word))
		}
		cardName := strings.Join(words, " ")

		// Format auction entry with enhanced colors but hide current price
		bidStatus := "No bids"
		if auction.BidCount > 0 {
			bidStatus = fmt.Sprintf("%d bid(s)", auction.BidCount)
		}

		// Modified format to hide current price
		description.WriteString(fmt.Sprintf("\u001b[36m[%s]\u001b[0m \u001b[33m%s\u001b[0m \u001b[32m[%s]\u001b[0m %s \u001b[97m%s\u001b[0m \u001b[94m[%s]\u001b[0m\n",
			timeStr,                         // Cyan for time
			auction.AuctionID,               // Gold for auction ID
			bidStatus,                       // Green for bid status instead of price
			strings.Repeat("★", card.Level), // Stars (no color)
			cardName,                        // Bright white for name
			strings.ToUpper(card.ColID)))    // Light blue for collection
	}
	description.WriteString("```")

	embed := discord.NewEmbedBuilder().
		SetTitle("🏛️ Auction House").
		SetDescription(description.String()).
		SetColor(0x2b2d31).
		SetFooter(fmt.Sprintf("Page 1/%d • Bid to reveal current price", (len(auctions)+9)/10), "")

	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewPrimaryButton("◀ Previous", "auction:prev_page"),
			discord.NewPrimaryButton("Next ▶", "auction:next_page"),
		),
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed.Build()},
		Components: components,
	})
}

func (h *AuctionHandler) HandleCancel(event *handler.ComponentEvent) error {
	return event.UpdateMessage(discord.MessageUpdate{
		Embeds: &[]discord.Embed{
			discord.NewEmbedBuilder().
				SetTitle("❌ Auction Cancelled").
				SetDescription("The auction creation was cancelled.").
				SetColor(0xFF0000).
				Build(),
		},
		Components: &[]discord.ContainerComponent{},
	})
}
