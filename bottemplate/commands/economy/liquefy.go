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
			// Check liquefy eligibility
			if !utils.IsCardLiquefyEligible(card, userCard) {
				return e.CreateMessage(discord.MessageCreate{
					Content: "❌ This card cannot be liquefied (may be locked, favorite with 1 copy, level 4+, or from restricted collection)",
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
			// Check liquefy eligibility
			if !utils.IsCardLiquefyEligible(card, userCard) {
				return e.CreateMessage(discord.MessageCreate{
					Content: "❌ This card cannot be liquefied (may be locked, favorite with 1 copy, level 4+, or from restricted collection)",
					Flags:   discord.MessageFlagEphemeral,
				})
			}
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
		// Check liquefy eligibility for weighted search result
		if !utils.IsCardLiquefyEligible(card, userCard) {
			return e.CreateMessage(discord.MessageCreate{
				Content: "❌ This card cannot be liquefied (may be locked, favorite with 1 copy, level 4+, or from restricted collection)",
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
			utils.GetPromoRarityPlainText(card.ColID, card.Level),
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
				vials))

		// Track effect progress for Holy Grail
		if h.bot.EffectManager != nil {
			go h.bot.EffectManager.UpdateEffectProgress(context.Background(), e.User().ID.String(), "holygrail", 1)
		}

		return e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed.Build()},
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

// findCardByName finds a card by name using enhanced search
func (h *LiquefyHandler) findCardByName(ctx context.Context, query, userID string) (*models.Card, error) {
	// Handle empty query
	if query == "" {
		return nil, fmt.Errorf("please provide a card name")
	}

	// Try direct query first (optimized approach)
	card, err := h.bot.CardRepository.GetByQuery(ctx, query)
	if err == nil {
		// Check if user owns this card
		userCard, err := h.bot.CardRepository.GetUserCard(ctx, userID, card.ID)
		if err == nil && userCard.Amount > 0 {
			return card, nil
		}
	}

	// Fallback to comprehensive search within user's cards
	userCards, err := h.bot.CardRepository.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user cards: %v", err)
	}

	// Get card IDs and create lookup map
	cardIDs := make([]int64, 0, len(userCards))
	userCardMap := make(map[int64]*models.UserCard)
	for _, uc := range userCards {
		cardIDs = append(cardIDs, uc.CardID)
		userCardMap[uc.CardID] = uc
	}

	if len(cardIDs) == 0 {
		return nil, fmt.Errorf("you don't have any cards available")
	}

	// Get card details
	cards, err := h.bot.CardRepository.GetByIDs(ctx, cardIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch card details: %v", err)
	}

	// Filter cards using centralized liquefy eligibility logic
	var liquefyEligibleCards []*models.Card
	eligibleUserCardMap := make(map[int64]*models.UserCard)
	for _, card := range cards {
		userCard := userCardMap[card.ID]
		if utils.IsCardLiquefyEligible(card, userCard) {
			liquefyEligibleCards = append(liquefyEligibleCards, card)
			eligibleUserCardMap[card.ID] = userCard
		}
	}

	if len(liquefyEligibleCards) == 0 {
		return nil, fmt.Errorf("no cards available for liquefying (cards may be locked, favorites, level 4+, or from restricted collections)")
	}

	// Use enhanced search filters on eligible cards
	filters := utils.ParseSearchQuery(query)
	searchResults := utils.WeightedSearchWithMulti(liquefyEligibleCards, filters, eligibleUserCardMap)

	if len(searchResults) == 0 {
		return nil, fmt.Errorf("no cards found matching '%s'", query)
	}

	// Return the best match
	return searchResults[0], nil
}
