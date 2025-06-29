package economy

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/vials"
	"github.com/disgoorg/bot-template/bottemplate/services"
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
	bot                   *bottemplate.Bot
	cardOperationsService *services.CardOperationsService
}

func NewLiquefyHandler(b *bottemplate.Bot) *LiquefyHandler {
	return &LiquefyHandler{
		bot:                   b,
		cardOperationsService: services.NewCardOperationsService(b.CardRepository, b.UserCardRepository),
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
		if err == nil && userCard != nil && userCard.Amount > 0 {
			return h.showLiquefyConfirmation(e, card, vm)
		}
	}

	// If exact match fails, try weighted search (like forge command)
	card, err = h.findCardByName(ctx, query, userID)
	if err == nil && card != nil {
		log.Printf("[DEBUG] Found weighted match: %+v", card)
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

	// If no card found or error occurred
	log.Printf("[DEBUG] No card found for query: %s, error: %v", query, err)
	return e.CreateMessage(discord.MessageCreate{
		Content: fmt.Sprintf("❌ Card '%s' not found or you don't own it", query),
		Flags:   discord.MessageFlagEphemeral,
	})
}

func (h *LiquefyHandler) showLiquefyConfirmation(e *handler.CommandEvent, card *models.Card, vm *vials.VialManager) error {
	// Calculate vial yield with effect bonuses
	userID := strconv.FormatInt(int64(e.User().ID), 10)
	vials, err := vm.CalculateVialYieldWithEffects(context.Background(), card, userID, h.bot.EffectIntegrator)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: "❌ Failed to calculate vial yield: " + err.Error(),
			Flags:   discord.MessageFlagEphemeral,
		})
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("🍷 Confirm Liquefication").
		SetColor(config.BackgroundColor).
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

// findCardByName finds a card by name using weighted search (similar to forge command pattern)
func (h *LiquefyHandler) findCardByName(ctx context.Context, query, userID string) (*models.Card, error) {
	// Handle empty query
	if query == "" {
		return nil, fmt.Errorf("please provide a card name")
	}

	// Get all cards from repository (same pattern as forge command)
	cards, err := h.bot.CardRepository.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to search for cards: %v", err)
	}

	// Parse search query and perform weighted search (same as forge command)
	filters := utils.ParseSearchQuery(query)
	filters.SortBy = utils.SortByLevel
	filters.SortDesc = true

	// Use CardOperationsService for consistent search behavior
	searchResults := h.cardOperationsService.SearchCardsInCollection(ctx, cards, filters)
	
	if len(searchResults) == 0 {
		return nil, fmt.Errorf("no cards found matching '%s'", query)
	}

	// Find first card that the user owns
	for _, card := range searchResults {
		// Check if user owns this card
		userCard, err := h.bot.UserCardRepository.GetUserCard(ctx, userID, card.ID)
		if err != nil {
			// User doesn't own this card, continue to next
			continue
		}

		// Skip cards with zero amount
		if userCard.Amount <= 0 {
			continue
		}

		// Found a valid card
		return card, nil
	}

	return nil, fmt.Errorf("you don't own any cards matching '%s'", query)
}
