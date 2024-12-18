package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/forge"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const ForgeCustomIDPrefix = "/forge/"

var Forge = discord.SlashCommandCreate{
	Name:        "forge",
	Description: "✨ Forge two cards into a new one",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "card_query_1",
			Description: "First card to forge (ID or name)",
			Required:    true,
		},
		discord.ApplicationCommandOptionString{
			Name:        "card_query_2",
			Description: "Second card to forge (ID or name)",
			Required:    false,
		},
	},
}

type ForgeHandler struct {
	bot *bottemplate.Bot
}

func NewForgeHandler(b *bottemplate.Bot) *ForgeHandler {
	return &ForgeHandler{
		bot: b,
	}
}

func (h *ForgeHandler) HandleForge(e *handler.CommandEvent) error {
	fm := forge.NewForgeManager(h.bot.DB, h.bot.PriceCalculator)
	query1 := strings.ReplaceAll(strings.TrimSpace(e.SlashCommandInteractionData().String("card_query_1")), " ", "_")
	query2 := strings.ReplaceAll(strings.TrimSpace(e.SlashCommandInteractionData().String("card_query_2")), " ", "_")
	ctx := context.Background()
	userID := strconv.FormatInt(int64(e.User().ID), 10)

	// Find first card
	card1, err := h.findCard(ctx, query1, userID)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Error finding first card: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Find second card
	card2, err := h.findCard(ctx, query2, userID)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Error finding second card: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Validate cards can be forged
	if card1.Level != card2.Level {
		return e.CreateMessage(discord.MessageCreate{
			Content: "❌ Cards must be of the same level to forge",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if card1.ID == card2.ID {
		return e.CreateMessage(discord.MessageCreate{
			Content: "❌ You must use two different cards to forge",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Calculate forge cost
	cost, err := fm.CalculateForgeCost(ctx, card1, card2)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Error calculating forge cost: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return h.showForgeConfirmation(e, card1, card2, cost)
}

func (h *ForgeHandler) showForgeConfirmation(e *handler.CommandEvent, card1, card2 *models.Card, cost int64) error {
	embed := discord.NewEmbedBuilder().
		SetTitle("⚔️ Confirm Forging").
		SetColor(0x2b2d31).
		SetDescription(fmt.Sprintf("```md\n"+
			"## Cards to Forge\n"+
			"1. %s (%s) %s\n"+
			"2. %s (%s) %s\n"+
			"\n## Details\n"+
			"* Level: %s\n"+
			"* Cost: %d 💰\n"+
			"* Result: Random %s card%s\n"+
			"```\n⚠️ Warning: This action cannot be undone!",
			card1.Name, card1.ColID, strings.Repeat("⭐", card1.Level),
			card2.Name, card2.ColID, strings.Repeat("⭐", card2.Level),
			strings.Repeat("⭐", card1.Level),
			cost,
			strings.Repeat("⭐", card1.Level),
			getSameCollectionBonus(card1, card2))).
		SetTimestamp(time.Now()).
		Build()

	actionRow := discord.NewActionRow(
		discord.NewSuccessButton(
			"Confirm",
			fmt.Sprintf("/forge/confirm/%d/%d", card1.ID, card2.ID)),
		discord.NewDangerButton(
			"Cancel",
			fmt.Sprintf("/forge/cancel/%d/%d", card1.ID, card2.ID)),
	)

	return e.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: []discord.ContainerComponent{actionRow},
		Flags:      discord.MessageFlagEphemeral,
	})
}

func (h *ForgeHandler) HandleComponent(e *handler.ComponentEvent) error {
	fm := forge.NewForgeManager(h.bot.DB, h.bot.PriceCalculator)
	userID := int64(e.User().ID)
	ctx := context.Background()

	parts := strings.Split(e.Data.CustomID(), "/")
	if len(parts) != 5 {
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid interaction"),
			Components: &[]discord.ContainerComponent{},
		})
	}

	card1ID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid card ID"),
			Components: &[]discord.ContainerComponent{},
		})
	}

	card2ID, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid card ID"),
			Components: &[]discord.ContainerComponent{},
		})
	}

	switch parts[2] {
	case "confirm":
		newCard, err := fm.ForgeCards(ctx, userID, card1ID, card2ID)
		if err != nil {
			return e.UpdateMessage(discord.MessageUpdate{
				Content:    utils.Ptr(fmt.Sprintf("❌ Failed to forge cards: %s", err.Error())),
				Components: &[]discord.ContainerComponent{},
			})
		}

		embed := discord.NewEmbedBuilder().
			SetTitle("⚔️ Forge Successful").
			SetColor(0x57F287).
			SetDescription(fmt.Sprintf("```md\n"+
				"## Result\n"+
				"* New Card: %s\n"+
				"* Collection: %s\n"+
				"* Level: %s\n"+
				"```",
				newCard.Name,
				newCard.ColID,
				strings.Repeat("⭐", newCard.Level))).
			SetTimestamp(time.Now()).
			Build()

		return e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &[]discord.ContainerComponent{},
		})

	case "cancel":
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Forge cancelled."),
			Components: &[]discord.ContainerComponent{},
		})

	default:
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid action"),
			Components: &[]discord.ContainerComponent{},
		})
	}
}

func (h *ForgeHandler) findCard(ctx context.Context, query, userID string) (*models.Card, error) {
	// Try parsing as ID first
	if cardID, err := strconv.ParseInt(query, 10, 64); err == nil {
		card, err := h.bot.CardRepository.GetByID(ctx, cardID)
		if err == nil {
			userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, cardID)
			if err != nil {
				return nil, fmt.Errorf("error checking card ownership: %v", err)
			}
			if userCard == nil || userCard.Amount <= 0 {
				return nil, fmt.Errorf("you don't own this card")
			}
			return card, nil
		}
	}

	// Try exact match
	card, err := h.bot.CardRepository.GetByQuery(ctx, query)
	if err == nil && card != nil {
		userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, card.ID)
		if err != nil {
			return nil, fmt.Errorf("error checking card ownership: %v", err)
		}
		if userCard == nil || userCard.Amount <= 0 {
			return nil, fmt.Errorf("you don't own this card")
		}
		return card, nil
	}

	// Try fuzzy search
	cards, err := h.bot.CardRepository.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to search for cards")
	}

	searchResults := utils.WeightedSearch(cards, query, utils.SearchModePartial)
	if len(searchResults) == 0 {
		return nil, fmt.Errorf("no cards found matching '%s'", query)
	}

	userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, searchResults[0].ID)
	if err != nil {
		return nil, fmt.Errorf("error checking ownership: %v", err)
	}

	if userCard == nil || userCard.Amount <= 0 {
		return nil, fmt.Errorf("you don't own this card")
	}

	return searchResults[0], nil
}

func getSameCollectionBonus(card1, card2 *models.Card) string {
	if card1.ColID == card2.ColID {
		return fmt.Sprintf(" from %s collection", card1.ColID)
	}
	return ""
}
