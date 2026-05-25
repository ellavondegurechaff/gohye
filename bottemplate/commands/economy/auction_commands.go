package economy

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
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
					Description: "Auction duration in hours (min 1 hour, max 24 hours)",
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
	bot      *bottemplate.Bot
	manager  *auction.Manager
	client   bot.Client
	cardRepo repositories.CardRepository
}

func NewAuctionHandler(bot *bottemplate.Bot, manager *auction.Manager, client bot.Client, cardRepo repositories.CardRepository) *AuctionHandler {
	return &AuctionHandler{
		bot:      bot,
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

	// Register auction list pagination components
	r.Component("/auction-list/", h.CreateAuctionListComponentHandler())
}

func (h *AuctionHandler) HandleCreate(event *handler.CommandEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
	defer cancel()

	data := event.SlashCommandInteractionData()
	cardName := data.String("card_name")
	startPrice := int64(data.Int("start_price"))
	duration := time.Duration(data.Int("duration")) * time.Hour

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

	if !utils.IsCardAuctionEligible(card, userCard) {
		return event.CreateMessage(discord.MessageCreate{
			Content: "❌ This card cannot be auctioned (legendary, locked, or restricted collection)",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Create confirmation embed
	embed := discord.NewEmbedBuilder().
		SetTitle("🏛️ Confirm Auction Creation").
		SetDescription(fmt.Sprintf("Please confirm that you want to create an auction for **%s**", utils.FormatCardName(card.Name))).
		AddField("Card", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(card.ColID, card.Level), utils.FormatCardName(card.Name)), false).
		AddField("Start Price", fmt.Sprintf("%d 💰", startPrice), true).
		AddField("Duration", formatDuration(duration), true).
		AddField("Collection", strings.ToUpper(card.ColID), true).
		SetColor(config.BackgroundColor).
		SetFooter("This auction will be visible to all users", "").
		Build()

	// Create confirmation buttons (restrict to command user)
	ownerID := event.User().ID.String()
	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewSuccessButton(
				"Confirm",
				fmt.Sprintf("/auction/confirm/%s/%d/%d/%d", ownerID, card.ID, startPrice, int64(duration.Seconds())),
			),
			discord.NewDangerButton(
				"Cancel",
				fmt.Sprintf("/auction/cancel/%s", ownerID),
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
	auctionIDStr := strings.ToUpper(data.String("auction_id"))
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

	// Track quest progress for auction bid
	if h.bot.QuestTracker != nil {
		go h.bot.QuestTracker.TrackAuctionBid(context.Background(), event.User().ID.String())
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
	userID := event.User().ID.String()

	// Create data fetcher
	fetcher := &AuctionListDataFetcher{
		manager:  h.manager,
		cardRepo: h.cardRepo,
	}

	// Create formatter
	formatter := &AuctionListFormatter{}

	// Create validator
	validator := &AuctionListValidator{}

	// Create factory configuration
	factoryConfig := utils.PaginationFactoryConfig{
		ItemsPerPage: 10,
		Prefix:       "auction-list",
		Parser:       utils.NewRegularParser("auction-list"),
		Fetcher:      fetcher,
		Formatter:    formatter,
		Validator:    validator,
	}

	// Create factory
	factory := utils.NewPaginationFactory(factoryConfig)

	// Create initial pagination params
	params := utils.PaginationParams{
		UserID: userID,
		Page:   0,
		Query:  "",
	}

	// Create initial embed and components
	embed, components, err := factory.CreateInitialPaginationEmbed(ctx, params)
	if err != nil {
		if err.Error() == "no items found" {
			return event.CreateMessage(discord.MessageCreate{
				Content: "No active auctions found.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("Failed to get auctions: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: components,
	})
}

func (h *AuctionHandler) HandleCancel(event *handler.ComponentEvent) error {
	// Validate only the command user can cancel
	parts := strings.Split(event.Data.CustomID(), "/")
	if len(parts) >= 4 {
		ownerID := parts[3]
		if ownerID != event.User().ID.String() {
			return utils.EH.CreateEphemeralError(event, "Only the command user can cancel this action.")
		}
	}
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

// CreateAuctionListComponentHandler creates component handler for auction list pagination
func (h *AuctionHandler) CreateAuctionListComponentHandler() handler.ComponentHandler {
	// Create data fetcher
	fetcher := &AuctionListDataFetcher{
		manager:  h.manager,
		cardRepo: h.cardRepo,
	}

	// Create formatter
	formatter := &AuctionListFormatter{}

	// Create validator
	validator := &AuctionListValidator{}

	// Create factory configuration
	factoryConfig := utils.PaginationFactoryConfig{
		ItemsPerPage: 10,
		Prefix:       "auction-list",
		Parser:       utils.NewRegularParser("auction-list"),
		Fetcher:      fetcher,
		Formatter:    formatter,
		Validator:    validator,
	}

	// Create factory and return handler
	factory := utils.NewPaginationFactory(factoryConfig)
	return factory.CreateHandler()
}

// AuctionListItem represents an auction item for pagination
type AuctionListItem struct {
	Auction *models.Auction
	Card    *models.Card
}

// AuctionListDataFetcher implements DataFetcher for auction list
type AuctionListDataFetcher struct {
	manager  *auction.Manager
	cardRepo repositories.CardRepository
}

func (f *AuctionListDataFetcher) FetchData(ctx context.Context, params utils.PaginationParams) ([]interface{}, error) {
	auctions, err := f.manager.GetActiveAuctions(ctx)
	if err != nil {
		return nil, err
	}

	var items []interface{}
	for _, auc := range auctions {
		card, err := f.cardRepo.GetByID(ctx, auc.CardID)
		if err != nil {
			continue
		}

		items = append(items, AuctionListItem{
			Auction: auc,
			Card:    card,
		})
	}

	return items, nil
}

// AuctionListFormatter implements ItemFormatter for auction list
type AuctionListFormatter struct{}

func (f *AuctionListFormatter) FormatItems(allItems []interface{}, page, totalPages int, params utils.PaginationParams) (discord.Embed, error) {
	// Items are already page-scoped by PaginationFactory
	pageItems := allItems

	var description strings.Builder
	description.WriteString("```ansi\n")

	for _, item := range pageItems {
		auctionItem := item.(AuctionListItem)
		auction := auctionItem.Auction
		card := auctionItem.Card

		timeLeft := time.Until(auction.EndTime).Round(time.Second)
		timeStr := formatDuration(timeLeft)

		// Format card display name (no underscores)
		cardName := utils.FormatCardName(card.Name)

		// Format auction entry with enhanced colors and show current price
		priceDisplay := fmt.Sprintf("%d 💰", auction.CurrentPrice)
		bidStatus := "No bids"
		if auction.BidCount > 0 {
			bidStatus = fmt.Sprintf("%d bid(s)", auction.BidCount)
		}

		// Show current price and bid status
		description.WriteString(fmt.Sprintf("\u001b[36m[%s]\u001b[0m \u001b[33m%s\u001b[0m \u001b[32m[%s]\u001b[0m \u001b[91m[%s]\u001b[0m %s \u001b[97m%s\u001b[0m \u001b[94m[%s]\u001b[0m\n",
			timeStr,           // Cyan for time
			auction.AuctionID, // Gold for auction ID
			bidStatus,         // Green for bid status
			priceDisplay,      // Red for current price
			utils.GetPromoRarityPlainText(card.ColID, card.Level), // Stars or promo emoji (no color)
			cardName,                     // Bright white for name
			strings.ToUpper(card.ColID))) // Light blue for collection
	}
	description.WriteString("```")

	return discord.Embed{
		Title:       fmt.Sprintf("🏛️ Auction House - Page %d/%d", page+1, totalPages),
		Description: description.String(),
		Color:       config.BackgroundColor,
		Footer: &discord.EmbedFooter{
			Text: "Use /auction bid to place bids",
		},
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (f *AuctionListFormatter) FormatCopy(items []interface{}, params utils.PaginationParams) string {
	var result []string
	for _, item := range items {
		auctionItem := item.(AuctionListItem)
		auction := auctionItem.Auction
		card := auctionItem.Card
		result = append(result, fmt.Sprintf("%s: %s - %d 💰", auction.AuctionID, utils.FormatCardName(card.Name), auction.CurrentPrice))
	}
	return strings.Join(result, "\n")
}

// AuctionListValidator implements UserValidator for auction list
type AuctionListValidator struct{}

func (v *AuctionListValidator) ValidateUser(eventUserID string, params utils.PaginationParams) bool {
	return eventUserID == params.UserID
}
