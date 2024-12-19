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
}

func (h *AuctionHandler) HandleCreate(event *handler.CommandEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data := event.SlashCommandInteractionData()
	cardID := int64(data.Int("card_id"))
	startPrice := int64(data.Int("start_price"))
	duration := time.Duration(data.Int("duration")) * time.Second

	if err := event.DeferCreateMessage(false); err != nil {
		return fmt.Errorf("failed to defer message: %w", err)
	}

	// Get card details first for the error message
	card, err := h.cardRepo.GetByID(ctx, cardID)
	if err != nil {
		_, err = event.CreateFollowupMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				discord.NewEmbedBuilder().
					SetTitle("❌ Error").
					SetDescription("Card not found in the database").
					SetColor(0xFF0000).
					Build(),
			},
			Flags: discord.MessageFlagEphemeral,
		})
		return err
	}

	// Check card ownership
	userCard, err := h.manager.UserCardRepo.GetByUserIDAndCardID(ctx, event.User().ID.String(), cardID)
	if err != nil || userCard == nil || userCard.Amount <= 0 {
		_, err = event.CreateFollowupMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				discord.NewEmbedBuilder().
					SetTitle("❌ Card Not Owned").
					SetDescription(fmt.Sprintf("You don't own the card: **%s** (ID: %d)\nTry checking your inventory to see which cards you own",
						card.Name, cardID)).
					SetColor(0xFF0000).
					Build(),
			},
			Flags: discord.MessageFlagEphemeral,
		})
		return err
	}

	// Create auction
	auction, err := h.manager.CreateAuction(ctx, cardID, event.User().ID.String(), startPrice, duration)
	if err != nil {
		_, err = event.CreateFollowupMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				discord.NewEmbedBuilder().
					SetTitle("❌ Auction Creation Failed").
					SetDescription(fmt.Sprintf("Failed to create auction: %s", err)).
					SetColor(0xFF0000).
					Build(),
			},
			Flags: discord.MessageFlagEphemeral,
		})
		return err
	}

	// Success message
	_, err = event.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{
			discord.NewEmbedBuilder().
				SetTitle("✅ Auction Created").
				SetDescription(fmt.Sprintf("Successfully created auction #%s for **%s**", auction.AuctionID, card.Name)).
				SetColor(0x00FF00).
				AddField("Start Price", fmt.Sprintf("%d 💰", startPrice), true).
				AddField("Duration", fmt.Sprintf("%d hours", int(duration.Hours())), true).
				Build(),
		},
		Flags: discord.MessageFlagEphemeral,
	})
	return err
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
		timeStr := formatDuration(timeLeft)

		// Format card name by capitalizing each word
		words := strings.Split(strings.ReplaceAll(card.Name, "_", " "), " ")
		for i, word := range words {
			words[i] = strings.Title(strings.ToLower(word))
		}
		cardName := strings.Join(words, " ")

		// Format auction entry with enhanced colors and new layout
		description.WriteString(fmt.Sprintf("\u001b[36m[%s]\u001b[0m \u001b[33m%s\u001b[0m \u001b[32m[%d]\u001b[0m %s \u001b[97m%s\u001b[0m \u001b[94m[%s]\u001b[0m\n",
			timeStr,                         // Cyan for time
			auction.AuctionID,               // Gold for auction ID
			auction.CurrentPrice,            // Green for price
			strings.Repeat("★", card.Level), // Stars (no color)
			cardName,                        // Bright white for name
			strings.ToUpper(card.ColID)))    // Light blue for collection
	}
	description.WriteString("```")

	embed := discord.NewEmbedBuilder().
		SetTitle("🏛️ Auction House").
		SetDescription(description.String()).
		SetColor(0x2b2d31).
		SetFooter(fmt.Sprintf("Page 1/%d", (len(auctions)+9)/10), "")

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
