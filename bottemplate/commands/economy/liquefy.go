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
	if err := e.DeferCreateMessage(true); err != nil {
		return err
	}

	vm := vials.NewVialManager(h.bot.DB, h.bot.PriceCalculator)
	query := strings.ReplaceAll(strings.TrimSpace(e.SlashCommandInteractionData().String("query")), " ", "_")
	ctx := context.Background()
	userID := strconv.FormatInt(int64(e.User().ID), 10)

	log.Printf("[DEBUG] Searching for card with query: %s", query)

	card, _, err := h.findOwnedLiquefyCard(ctx, query, userID)
	if err != nil {
		log.Printf("[DEBUG] No owned liquefy card found for query: %s, error: %v", query, err)
		return updateLiquefyCommandContent(e, fmt.Sprintf("❌ %s", err.Error()))
	}

	log.Printf("[DEBUG] Found owned liquefy match: %+v", card)
	return h.showLiquefyConfirmation(e, card, vm)
}

func (h *LiquefyHandler) showLiquefyConfirmation(e *handler.CommandEvent, card *models.Card, vm *vials.VialManager) error {
	// Calculate vial yield with effect bonuses
	userID := strconv.FormatInt(int64(e.User().ID), 10)
	vials, err := vm.CalculateVialYieldWithEffects(context.Background(), card, userID, h.bot.EffectIntegrator)
	if err != nil {
		return updateLiquefyCommandContent(e, "❌ Failed to calculate vial yield: "+err.Error())
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("🍷 Confirm Liquefication").
		SetColor(config.BackgroundColor).
		SetDescription(fmt.Sprintf("```md\n## Card Details\n* Name: %s\n* Collection: %s\n* Level: %s\n* Vial Yield: %d 🍷\n```\n⚠️ Warning: This action cannot be undone!",
			utils.FormatCardName(card.Name),
			card.ColID,
			utils.GetPromoRarityPlainText(card.ColID, card.Level),
			vials)).
		SetTimestamp(time.Now()).
		Build()

	ownerID := e.User().ID.String()
	actionRow := discord.NewActionRow(
		discord.NewSuccessButton(
			"Confirm",
			fmt.Sprintf("/liquefy/confirm/%s/%d", ownerID, card.ID)),
		discord.NewDangerButton(
			"Cancel",
			fmt.Sprintf("/liquefy/cancel/%s/%d", ownerID, card.ID)),
	)

	_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{actionRow},
	})
	return err
}

func (h *LiquefyHandler) HandleComponent(e *handler.ComponentEvent) error {
	if err := e.DeferUpdateMessage(); err != nil {
		return err
	}

	vm := vials.NewVialManager(h.bot.DB, h.bot.PriceCalculator)
	userID := int64(e.User().ID)
	ctx := context.Background()

	parts := strings.Split(e.Data.CustomID(), "/")
	if len(parts) != 5 {
		_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid interaction"),
			Components: &[]discord.ContainerComponent{},
		})
		return err
	}

	if parts[3] != e.User().ID.String() {
		return utils.EH.CreateEphemeralError(e, "Only the command user can use these buttons.")
	}

	cardID, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid card ID"),
			Components: &[]discord.ContainerComponent{},
		})
		return err
	}

	// Check if user owns the card first
	userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, strconv.FormatInt(userID, 10), cardID)
	if err != nil {
		_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Failed to liquefy card: card not found in your inventory"),
			Components: &[]discord.ContainerComponent{},
		})
		return err
	}

	if userCard == nil || userCard.Amount <= 0 {
		_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Failed to liquefy card: you don't own this card"),
			Components: &[]discord.ContainerComponent{},
		})
		return err
	}

	switch parts[2] {
	case "confirm":
		card, err := h.bot.CardRepository.GetByID(context.Background(), cardID)
		if err != nil {
			_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
				Content:    utils.Ptr("❌ Card not found"),
				Components: &[]discord.ContainerComponent{},
			})
			return err
		}
		if !utils.IsCardLiquefyEligible(card, userCard) {
			_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
				Content:    utils.Ptr("❌ Failed to liquefy card: this card is locked, protected, too high rarity, or from a restricted collection"),
				Components: &[]discord.ContainerComponent{},
			})
			return err
		}

		vials, err := vm.LiquefyCardWithEffects(context.Background(), userID, cardID, h.bot.EffectIntegrator)
		if err != nil {
			_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
				Content:    utils.Ptr(fmt.Sprintf("❌ Failed to liquefy card: %s", err.Error())),
				Components: &[]discord.ContainerComponent{},
			})
			return err
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

		_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed.Build()},
			Components: &[]discord.ContainerComponent{},
		})
		return err

	case "cancel":
		_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Liquefication cancelled."),
			Components: &[]discord.ContainerComponent{},
		})
		return err

	default:
		_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid action"),
			Components: &[]discord.ContainerComponent{},
		})
		return err
	}
}

func updateLiquefyCommandContent(e *handler.CommandEvent, content string) error {
	_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
		Content:    utils.Ptr(content),
		Embeds:     &[]discord.Embed{},
		Components: &[]discord.ContainerComponent{},
	})
	return err
}

// findOwnedLiquefyCard searches only the user's owned inventory, so the card
// shown in confirmation is guaranteed to be the same card ID later removed.
func (h *LiquefyHandler) findOwnedLiquefyCard(ctx context.Context, query, userID string) (*models.Card, *models.UserCard, error) {
	if query == "" {
		return nil, nil, fmt.Errorf("please provide a card name")
	}

	userCards, err := h.bot.UserCardRepository.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch user cards: %v", err)
	}
	if len(userCards) == 0 {
		return nil, nil, fmt.Errorf("you don't have any cards available")
	}

	userCardMap := make(map[int64]*models.UserCard)
	cardIDs := make([]int64, 0, len(userCards))
	for _, uc := range userCards {
		if uc == nil || uc.Amount <= 0 {
			continue
		}
		if existing, ok := userCardMap[uc.CardID]; ok {
			existing.Amount += uc.Amount
			existing.Favorite = existing.Favorite || uc.Favorite
			existing.Locked = existing.Locked || uc.Locked
			continue
		}
		copyUC := *uc
		userCardMap[uc.CardID] = &copyUC
		cardIDs = append(cardIDs, uc.CardID)
	}

	cards, err := h.bot.CardRepository.GetByIDs(ctx, cardIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch card details: %v", err)
	}

	cardByID := make(map[int64]*models.Card, len(cards))
	for _, card := range cards {
		cardByID[card.ID] = card
	}

	if cardID, err := strconv.ParseInt(query, 10, 64); err == nil && cardID > 0 {
		card := cardByID[cardID]
		userCard := userCardMap[cardID]
		if card == nil || userCard == nil {
			return nil, nil, fmt.Errorf("card '%s' not found in your inventory", query)
		}
		if !utils.IsCardLiquefyEligible(card, userCard) {
			return nil, nil, fmt.Errorf("this card cannot be liquefied (may be locked, favorite with 1 copy, level 4+, or from restricted collection)")
		}
		return card, userCard, nil
	}

	normalizedQuery := normalizeLiquefySearchTerm(query)
	var ownedCards []*models.Card
	for _, card := range cards {
		userCard := userCardMap[card.ID]
		if userCard == nil {
			continue
		}
		ownedCards = append(ownedCards, card)
		if normalizeLiquefySearchTerm(card.Name) == normalizedQuery {
			if !utils.IsCardLiquefyEligible(card, userCard) {
				return nil, nil, fmt.Errorf("this card cannot be liquefied (may be locked, favorite with 1 copy, level 4+, or from restricted collection)")
			}
			return card, userCard, nil
		}
	}

	filters := utils.ParseSearchQuery(query)
	searchResults := utils.WeightedSearchWithMulti(ownedCards, filters, userCardMap)
	for _, card := range searchResults {
		userCard := userCardMap[card.ID]
		if utils.IsCardLiquefyEligible(card, userCard) {
			return card, userCard, nil
		}
	}

	if len(searchResults) > 0 {
		return nil, nil, fmt.Errorf("matching cards cannot be liquefied (may be locked, favorite with 1 copy, level 4+, or from restricted collection)")
	}
	return nil, nil, fmt.Errorf("card '%s' not found in your inventory", query)
}

func normalizeLiquefySearchTerm(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return value
}
