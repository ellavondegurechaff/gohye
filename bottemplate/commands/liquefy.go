package commands

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/vials"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// Add this constant at the top of the file
const LiquefyCustomIDPrefix = "/liquefy/"

// Add to commands list
var Liquefy = discord.SlashCommandCreate{
	Name:        "liquefy",
	Description: "Convert a card into vials",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "query",
			Description: "Card ID or name to liquefy",
			Required:    true,
		},
	},
}

type LiquefyHandler struct {
	bot *bottemplate.Bot
}

func NewLiquefyHandler(b *bottemplate.Bot) *LiquefyHandler {
	return &LiquefyHandler{
		bot: b,
	}
}

func (h *LiquefyHandler) HandleLiquefy(e *handler.CommandEvent) error {
	vm := vials.NewVialManager(h.bot.DB, h.bot.PriceCalculator)
	query := strings.ReplaceAll(strings.TrimSpace(e.SlashCommandInteractionData().String("query")), " ", "_")
	ctx := context.Background()
	userID := strconv.FormatInt(int64(e.User().ID), 10)

	log.Printf("[DEBUG] Searching for card with query: %s", query)

	// Try parsing as ID first
	if cardID, err := strconv.ParseInt(query, 10, 64); err == nil {
		card, err := h.bot.CardRepository.GetByID(ctx, cardID)
		if err == nil {
			userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, cardID)
			if err != nil {
				log.Printf("[ERROR] Error checking card ownership: %v", err)
				return e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Error checking card ownership: %v", err),
					Flags:   discord.MessageFlagEphemeral,
				})
			}
			if userCard == nil || userCard.Amount <= 0 {
				return e.CreateMessage(discord.MessageCreate{
					Content: "❌ You don't own this card",
					Flags:   discord.MessageFlagEphemeral,
				})
			}
			return h.showLiquefyConfirmation(e, card, vm)
		}
	}

	// For name search, try exact match first
	card, err := h.bot.CardRepository.GetByQuery(ctx, query)
	if err == nil && card != nil {
		log.Printf("[DEBUG] Found exact match: %+v", card)
		userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, card.ID)
		if err != nil {
			log.Printf("[ERROR] Error checking card ownership: %v", err)
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Error checking card ownership: %v", err),
				Flags:   discord.MessageFlagEphemeral,
			})
		}
		if userCard == nil || userCard.Amount <= 0 {
			log.Printf("[DEBUG] User doesn't own card: %+v", userCard)
			return e.CreateMessage(discord.MessageCreate{
				Content: "❌ You don't own this card",
				Flags:   discord.MessageFlagEphemeral,
			})
		}
		return h.showLiquefyConfirmation(e, card, vm)
	}

	// If exact match fails, try fuzzy search
	cards, err := h.bot.CardRepository.GetAll(ctx)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "Failed to search for cards",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	filters := utils.ParseSearchQuery(query)
	filters.SortBy = utils.SortByLevel // Prioritize higher level cards
	filters.SortDesc = true            // Descending order

	searchResults := utils.WeightedSearch(cards, filters)
	if len(searchResults) == 0 {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ No cards found matching '%s'", query),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	// Debug logging
	log.Printf("[DEBUG] Found card match: %+v", searchResults[0])

	userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, searchResults[0].ID)
	if err != nil {
		log.Printf("[ERROR] Error checking ownership: %v", err)
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Error checking card ownership: %v", err),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	if userCard == nil || userCard.Amount <= 0 {
		log.Printf("[DEBUG] User card data: %+v", userCard)
		return e.CreateMessage(discord.MessageCreate{
			Content: "❌ You don't own this card",
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	return h.showLiquefyConfirmation(e, searchResults[0], vm)
}

func (h *LiquefyHandler) showLiquefyConfirmation(e *handler.CommandEvent, card *models.Card, vm *vials.VialManager) error {
	vials, err := vm.CalculateVialYield(context.Background(), card)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "❌ Failed to calculate vial yield: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("🍷 Confirm Liquefication").
		SetColor(0x2b2d31).
		SetDescription(fmt.Sprintf("```md\n## Card Details\n* Name: %s\n* Collection: %s\n* Level: %s\n* Vial Yield: %d 🍷\n```\n⚠️ Warning: This action cannot be undone!",
			card.Name,
			card.ColID,
			strings.Repeat("⭐", card.Level),
			vials)).
		SetTimestamp(time.Now()).
		Build()

	actionRow := discord.NewActionRow(
		discord.NewSuccessButton(
			"Confirm",
			fmt.Sprintf("/liquefy/confirm/%d", card.ID)),
		discord.NewDangerButton(
			"Cancel",
			fmt.Sprintf("/liquefy/cancel/%d", card.ID)),
	)

	return e.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: []discord.ContainerComponent{actionRow},
		Flags:      discord.MessageFlagEphemeral,
	})
}

func (h *LiquefyHandler) HandleComponent(e *handler.ComponentEvent) error {
	vm := vials.NewVialManager(h.bot.DB, h.bot.PriceCalculator)
	userID := int64(e.User().ID)
	ctx := context.Background()

	parts := strings.Split(e.Data.CustomID(), "/")
	if len(parts) != 4 {
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("��� Invalid interaction"),
			Components: &[]discord.ContainerComponent{},
		})
	}

	cardID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid card ID"),
			Components: &[]discord.ContainerComponent{},
		})
	}

	// Check if user owns the card first
	userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, strconv.FormatInt(userID, 10), cardID)
	if err != nil {
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Failed to liquefy card: card not found in your inventory"),
			Components: &[]discord.ContainerComponent{},
		})
	}

	if userCard == nil || userCard.Amount <= 0 {
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Failed to liquefy card: you don't own this card"),
			Components: &[]discord.ContainerComponent{},
		})
	}

	switch parts[2] {
	case "confirm":
		card, err := h.bot.CardRepository.GetByID(context.Background(), cardID)
		if err != nil {
			return e.UpdateMessage(discord.MessageUpdate{
				Content:    utils.Ptr("❌ Card not found"),
				Components: &[]discord.ContainerComponent{},
			})
		}

		vials, err := vm.LiquefyCard(context.Background(), userID, cardID)
		if err != nil {
			return e.UpdateMessage(discord.MessageUpdate{
				Content:    utils.Ptr(fmt.Sprintf("❌ Failed to liquefy card: %s", err.Error())),
				Components: &[]discord.ContainerComponent{},
			})
		}

		embed := discord.NewEmbedBuilder().
			SetTitle("🍷 Card Successfully Liquefied").
			SetColor(0x57F287).
			SetDescription(fmt.Sprintf("```md\n## Result\n* Card: %s\n* Collection: %s\n* Vials Received: %d 🍷\n```",
				card.Name,
				card.ColID,
				vials)).
			SetTimestamp(time.Now()).
			Build()

		return e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &[]discord.ContainerComponent{},
		})

	case "cancel":
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Liquefication cancelled."),
			Components: &[]discord.ContainerComponent{},
		})

	default:
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid action"),
			Components: &[]discord.ContainerComponent{},
		})
	}
}
