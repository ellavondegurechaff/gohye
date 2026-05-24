package economy

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

var TradeCommand = discord.SlashCommandCreate{
	Name:        "trade",
	Description: "Trade cards with other users",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "your_card",
			Description: "The card you want to offer",
			Required:    true,
		},
		discord.ApplicationCommandOptionUser{
			Name:        "user",
			Description: "The user you want to trade with",
			Required:    true,
		},
		discord.ApplicationCommandOptionString{
			Name:        "their_card",
			Description: "The card you want from them",
			Required:    true,
		},
	},
}

var InboxCommand = discord.SlashCommandCreate{
	Name:        "inbox",
	Description: "View your trade offers",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionBool{
			Name:        "open_offers",
			Description: "Show only pending offers (default: true)",
			Required:    false,
		},
	},
}

type TradeHandler struct {
	bot          *bottemplate.Bot
	client       bot.Client
	tradeRepo    repositories.TradeRepository
	userCardRepo repositories.UserCardRepository
	cardRepo     repositories.CardRepository
	userRepo     repositories.UserRepository
}

func NewTradeHandler(bot *bottemplate.Bot, client bot.Client, tradeRepo repositories.TradeRepository, userCardRepo repositories.UserCardRepository, cardRepo repositories.CardRepository, userRepo repositories.UserRepository) *TradeHandler {
	return &TradeHandler{
		bot:          bot,
		client:       client,
		tradeRepo:    tradeRepo,
		userCardRepo: userCardRepo,
		cardRepo:     cardRepo,
		userRepo:     userRepo,
	}
}

func (h *TradeHandler) Register(r handler.Router) {
	r.Command("/trade", h.HandleTrade)
	r.Command("/inbox", h.HandleInbox)

	// Component handlers for trade interactions
	r.Component("/trade/accept/", h.HandleTradeAccept)
	r.Component("/trade/decline/", h.HandleTradeDecline)
	r.Component("/trade/cancel/", h.HandleTradeCancel)

	// Pagination components for inbox
	r.Component("/trade-inbox/", h.CreateInboxComponentHandler())
}

func (h *TradeHandler) HandleTrade(event *handler.CommandEvent) error {
    // Defer immediately to avoid 3s timeout (10062)
    if err := event.DeferCreateMessage(false); err != nil {
        return err
    }
    ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
    defer cancel()

	data := event.SlashCommandInteractionData()
	yourCardName := data.String("your_card")
	targetUser := data.User("user")
	theirCardName := data.String("their_card")

	offererID := event.User().ID.String()
	targetID := targetUser.ID.String()

    // Prevent self-trading
    if offererID == targetID {
        _, updErr := event.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ You cannot trade with yourself!")})
        return updErr
    }

	// Find offerer's card
    offererCard, err := h.getUserCardByName(ctx, offererID, yourCardName)
    if err != nil {
        _, updErr := event.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr(fmt.Sprintf("❌ You don't own a card matching '%s' or you don't have any copies available.", yourCardName))})
        return updErr
    }

	// Find target's card
    targetCard, err := h.getUserCardByName(ctx, targetID, theirCardName)
    if err != nil {
        _, updErr := event.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr(fmt.Sprintf("❌ %s doesn't own a card matching '%s' or they don't have any copies available.", targetUser.Username, theirCardName))})
        return updErr
    }

	// Check for existing pending trades between these users
    pendingTrades, err := h.tradeRepo.GetPendingTradesBetweenUsers(ctx, offererID, targetID)
    if err != nil {
        _, updErr := event.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Failed to check for existing trades.")})
        return updErr
    }

    if len(pendingTrades) > 0 {
        _, updErr := event.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr(fmt.Sprintf("❌ You already have a pending trade with %s. Please wait for them to respond or check your /inbox.", targetUser.Username))})
        return updErr
    }

	// Get card details for display
    offererCardDetails, err := h.cardRepo.GetByID(ctx, offererCard.CardID)
    if err != nil {
        _, updErr := event.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Failed to get your card details.")})
        return updErr
    }

    targetCardDetails, err := h.cardRepo.GetByID(ctx, targetCard.CardID)
    if err != nil {
        _, updErr := event.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Failed to get their card details.")})
        return updErr
    }

	// Generate unique trade ID
    tradeID, err := h.generateTradeID(ctx, offererCardDetails)
    if err != nil {
        _, updErr := event.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Failed to generate trade ID.")})
        return updErr
    }

	// Create trade offer
	trade := &models.Trade{
		TradeID:       tradeID,
		OffererID:     offererID,
		TargetID:      targetID,
		OffererCardID: offererCard.CardID,
		TargetCardID:  targetCard.CardID,
	}

    err = h.tradeRepo.Create(ctx, trade)
    if err != nil {
        _, updErr := event.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("❌ Failed to create trade offer.")})
        return updErr
    }

    // Create confirmation embed (for the offerer)
    embed := discord.NewEmbedBuilder().
        SetTitle("🔄 Trade Offer Created").
        SetDescription(fmt.Sprintf("Your trade offer has been sent to %s!", targetUser.Mention())).
        AddField("Your Offer", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(offererCardDetails.ColID, offererCardDetails.Level), offererCardDetails.Name), false).
        AddField("Requesting", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(targetCardDetails.ColID, targetCardDetails.Level), targetCardDetails.Name), false).
        AddField("Trade ID", tradeID, false).
        SetColor(config.BackgroundColor).
        SetFooter(fmt.Sprintf("%s can view this offer in their /inbox", targetUser.Username), "").
        Build()

    // Send DM to target user (best-effort)
    go h.sendTradeNotificationDM(targetID, trade, offererCardDetails, targetCardDetails, event.User().Username)

    // Respond to the offerer via the deferred interaction
    if _, err := event.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{embed}}); err != nil {
        return err
    }

    // Post an interactive offer in channel so the target can accept/decline
    offerEmbed := discord.NewEmbedBuilder().
        SetTitle("🔄 Trade Offer").
        SetDescription(fmt.Sprintf("%s wants to trade with %s", event.User().Mention(), targetUser.Mention())).
        AddField("Offerer Gives", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(offererCardDetails.ColID, offererCardDetails.Level), offererCardDetails.Name), false).
        AddField("Offerer Wants", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(targetCardDetails.ColID, targetCardDetails.Level), targetCardDetails.Name), false).
        AddField("Trade ID", trade.TradeID, false).
        SetColor(config.BackgroundColor).
        SetFooter("Only the target user can accept or decline this offer", "").
        Build()

    actionRow := discord.NewActionRow(
        discord.NewSuccessButton("Accept", fmt.Sprintf("/trade/accept/%d", trade.ID)),
        discord.NewDangerButton("Decline", fmt.Sprintf("/trade/decline/%d", trade.ID)),
    )

    _, _ = event.CreateFollowupMessage(discord.MessageCreate{Embeds: []discord.Embed{offerEmbed}, Components: []discord.ContainerComponent{actionRow}})

    return nil
}

func (h *TradeHandler) HandleInbox(event *handler.CommandEvent) error {
	ctx := context.Background()
	userID := event.User().ID.String()

	data := event.SlashCommandInteractionData()
	showOnlyPending := true // default value
	if openOffers, ok := data.OptBool("open_offers"); ok {
		showOnlyPending = openOffers
	}

	// Create data fetcher
	fetcher := &InboxDataFetcher{
		tradeRepo:    h.tradeRepo,
		cardRepo:     h.cardRepo,
		userRepo:     h.userRepo,
		onlyPending:  showOnlyPending,
	}

	// Create formatter
	formatter := &InboxFormatter{
		userID: userID,
	}

	// Create initial pagination params
	params := utils.PaginationParams{
		UserID: userID,
		Page:   0,
		Query:  "",
	}

	// Create initial embed and components with custom trade inbox handler
	embed, components, err := h.createInitialInboxEmbed(ctx, params, fetcher, formatter)
	if err != nil {
		if err.Error() == "no items found" {
			title := "📥 Trade Inbox"
			if showOnlyPending {
				title = "📥 Pending Trade Offers"
			}
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{
					discord.NewEmbedBuilder().
						SetTitle(title).
						SetDescription("No trade offers found.").
						SetColor(config.BackgroundColor).
						Build(),
				},
			})
		}
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Failed to get trades: %s", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: components,
	})
}

// Helper functions

func (h *TradeHandler) getUserCardByName(ctx context.Context, userID string, cardName string) (*models.UserCard, error) {
	// Try direct query first
	if card, err := h.cardRepo.GetByQuery(ctx, cardName); err == nil {
		if userCard, err := h.userCardRepo.GetUserCard(ctx, userID, card.ID); err == nil && userCard.Amount > 0 {
			return userCard, nil
		}
	}

	// Fallback to comprehensive search
	userCards, err := h.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user cards: %w", err)
	}

	// Filter for cards with amount > 0
	cardIDs := make([]int64, 0, len(userCards))
	userCardMap := make(map[int64]*models.UserCard)
	for _, uc := range userCards {
		if uc.Amount > 0 {
			cardIDs = append(cardIDs, uc.CardID)
			userCardMap[uc.CardID] = uc
		}
	}

	if len(cardIDs) == 0 {
		return nil, fmt.Errorf("no cards found")
	}

	// Get card details
	cards, err := h.cardRepo.GetByIDs(ctx, cardIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get card details: %w", err)
	}

	// Use weighted search
	searchFilters := utils.SearchFilters{
		Name:  cardName,
		Query: cardName,
	}
	matches := utils.WeightedSearch(cards, searchFilters)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no matching cards found")
	}

	return userCardMap[matches[0].ID], nil
}

func (h *TradeHandler) generateTradeID(ctx context.Context, card *models.Card) (string, error) {
	// Create base prefix similar to auction ID
	words := strings.Fields(card.Name)
	var prefix string
	if len(words) >= 2 {
		prefix = strings.ToUpper(string(words[0][0]) + string(words[1][0]))
	} else if len(words) == 1 {
		if len(words[0]) >= 2 {
			prefix = strings.ToUpper(words[0][:2])
		} else {
			prefix = strings.ToUpper(words[0] + "X")
		}
	}

	// Add trade indicator
	prefix = "T" + prefix

	// Generate random suffix
	for attempt := 0; attempt < 10; attempt++ {
		var randomBytes [4]byte
		if _, err := rand.Read(randomBytes[:]); err != nil {
			return "", fmt.Errorf("failed to generate random bytes: %w", err)
		}

		randomNum := binary.BigEndian.Uint32(randomBytes[:])
		suffix := fmt.Sprintf("%04d", randomNum%10000)
		tradeID := prefix + suffix

		// Check if ID exists
		exists, err := h.tradeRepo.TradeIDExists(ctx, tradeID)
		if err != nil {
			continue
		}
		if !exists {
			return tradeID, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique trade ID")
}

func (h *TradeHandler) sendTradeNotificationDM(targetID string, trade *models.Trade, offererCard, targetCard *models.Card, offererUsername string) {
	dmChannel, err := h.client.Rest().CreateDMChannel(snowflake.MustParse(targetID))
	if err != nil {
		return // Silently fail DM sending
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("🔄 New Trade Offer").
		SetDescription(fmt.Sprintf("**%s** wants to trade with you!", offererUsername)).
		AddField("They Offer", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(offererCard.ColID, offererCard.Level), offererCard.Name), false).
		AddField("They Want", fmt.Sprintf("%s %s", utils.GetPromoRarityPlainText(targetCard.ColID, targetCard.Level), targetCard.Name), false).
		AddField("Trade ID", trade.TradeID, false).
		SetColor(config.BackgroundColor).
		SetFooter("Use /inbox to view and respond to this trade offer", "").
		Build()

	_, _ = h.client.Rest().CreateMessage(dmChannel.ID(), discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
}

// createInitialInboxEmbed creates the initial inbox embed with custom components
func (h *TradeHandler) createInitialInboxEmbed(ctx context.Context, params utils.PaginationParams, fetcher *InboxDataFetcher, formatter *InboxFormatter) (discord.Embed, []discord.ContainerComponent, error) {
	// Fetch all data
	allItems, err := fetcher.FetchData(ctx, params)
	if err != nil {
		return discord.Embed{}, nil, err
	}

	if len(allItems) == 0 {
		return discord.Embed{}, nil, fmt.Errorf("no items found")
	}

	// Calculate pagination
	itemsPerPage := 5
	totalPages := (len(allItems) + itemsPerPage - 1) / itemsPerPage

	// Get first page items
	endIdx := min(itemsPerPage, len(allItems))
	pageItems := allItems[0:endIdx]

	// Create embed
	embed, err := formatter.FormatItems(pageItems, 0, totalPages, params)
	if err != nil {
		return discord.Embed{}, nil, err
	}

	// Create components with both pagination and trade action buttons
	components := h.createInboxComponents(pageItems, totalPages, params)

	return embed, components, nil
}

// Component handlers will be implemented in a separate file for better organization
