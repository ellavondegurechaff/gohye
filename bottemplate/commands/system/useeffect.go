package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var UseEffect = discord.SlashCommandCreate{
	Name:        "use-effect",
	Description: "Use an active effect from your inventory",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "effect",
			Description: "The effect to use",
			Required:    true,
		},
		discord.ApplicationCommandOptionString{
			Name:        "arguments",
			Description: "Additional arguments for the effect (if required)",
			Required:    false,
		},
	},
}

type UseEffectHandler struct {
	bot           *bottemplate.Bot
	effectManager *effects.Manager
}

func NewUseEffectHandler(b *bottemplate.Bot, effectManager *effects.Manager) *UseEffectHandler {
	return &UseEffectHandler{
		bot:           b,
		effectManager: effectManager,
	}
}

func (h *UseEffectHandler) Handle(event *handler.CommandEvent) error {
	ctx := context.Background()

	effectID := event.SlashCommandInteractionData().String("effect")
	args := event.SlashCommandInteractionData().String("arguments")

	// Create a context to store effect result data
	enrichedCtx := context.WithValue(ctx, "effect_result_data", make(map[string]interface{}))

	// Use the effect through the manager to get full result data
	resultMessage, err := h.effectManager.UseActiveEffect(enrichedCtx, event.User().ID.String(), effectID, args)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				{
					Title:       "❌ Effect Failed",
					Description: err.Error(),
					Color:       utils.ErrorColor,
				},
			},
			// Removed ephemeral flag to make error messages public
		})
	}

	// Check if we need to get additional data for enhanced display
	return h.createEffectResultMessage(enrichedCtx, event, effectID, args, resultMessage)
}

// createEffectResultMessage creates a rich message for effect results
func (h *UseEffectHandler) createEffectResultMessage(ctx context.Context, event *handler.CommandEvent, effectID, args, resultMessage string) error {
	// Check if this was a Judgment Day effect that might have card data
	isJudgmentDay := strings.HasPrefix(resultMessage, "[Judgment Day]")

	// Try to get card data from the effect result if available
	cardData := h.extractCardDataFromContext(ctx, effectID, args, isJudgmentDay)
	if cardData != nil {
		return h.createCardDisplayMessage(event, resultMessage, cardData)
	}

	// Fallback to recent card lookup for card-giving effects
	cardData, err := h.getCardDataFromEffect(ctx, event.User().ID.String(), effectID, args, isJudgmentDay)
	if err == nil && cardData != nil {
		return h.createCardDisplayMessage(event, resultMessage, cardData)
	}

	// Default text-based result (for non-card effects)
	embed := discord.NewEmbedBuilder().
		SetTitle("✅ Effect Used").
		SetDescription(fmt.Sprintf("```md\n# Effect Result\n%s\n```", resultMessage)).
		SetColor(utils.SuccessColor).
		SetFooter(fmt.Sprintf("Used by %s • %s", event.User().Username, fmt.Sprintf("<t:%d:R>", time.Now().Unix())), event.User().EffectiveAvatarURL()).
		Build()

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
		// Removed ephemeral flag to make results public
	})
}

// extractCardDataFromContext attempts to get card data from the effect result context
func (h *UseEffectHandler) extractCardDataFromContext(ctx context.Context, effectID, args string, isJudgmentDay bool) *models.Card {
	// Get effect result data from context
	resultData, ok := ctx.Value("effect_result_data").(map[string]interface{})
	if !ok || resultData == nil {
		return nil
	}

	// Look for card data in the result
	if cardID, exists := resultData["card_id"]; exists {
		if cardIDInt, ok := cardID.(int64); ok {
			// Fetch the card from database
			var card models.Card
			err := h.bot.DB.BunDB().NewSelect().
				Model(&card).
				Where("id = ?", cardIDInt).
				Scan(context.Background())
			if err == nil {
				return &card
			}
		}
	}

	return nil
}

// getCardDataFromEffect attempts to retrieve card data from recent effect usage
func (h *UseEffectHandler) getCardDataFromEffect(ctx context.Context, userID, effectID, args string, isJudgmentDay bool) (*models.Card, error) {
	// Only certain effects give cards - check for known card-giving effects
	targetEffectID := effectID
	if isJudgmentDay {
		// For Judgment Day, parse the actual effect used
		parts := strings.SplitN(args, " ", 2)
		if len(parts) > 0 {
			targetEffectID = parts[0]
		}
	}

	// Check if this is a card-giving effect
	switch targetEffectID {
	case "spaceunity":
		// For SpaceUnity, we need to get the most recently added card
		// This is a bit of a workaround since we don't have direct access to the effect result data
		return h.getMostRecentlyAddedCard(ctx, userID)
	default:
		return nil, fmt.Errorf("not a card-giving effect")
	}
}

// getMostRecentlyAddedCard gets the user's most recently obtained card (best effort)
func (h *UseEffectHandler) getMostRecentlyAddedCard(ctx context.Context, userID string) (*models.Card, error) {
	// This is a best-effort approach to get the most recently added card
	// In a perfect world, we'd have access to the effect result data directly

	// Get user's cards sorted by updated_at descending
	var userCards []*models.UserCard
	err := h.bot.DB.BunDB().NewSelect().
		Model(&userCards).
		Where("user_id = ? AND amount > 0", userID).
		Order("updated_at DESC").
		Limit(5). // Get the 5 most recent to find a likely candidate
		Scan(ctx)

	if err != nil || len(userCards) == 0 {
		return nil, fmt.Errorf("no recent cards found")
	}

	// Get the actual card data for the most recently updated user card
	var card models.Card
	err = h.bot.DB.BunDB().NewSelect().
		Model(&card).
		Where("id = ?", userCards[0].CardID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get card data")
	}

	return &card, nil
}

// createCardDisplayMessage creates a rich embed with card image and info
func (h *UseEffectHandler) createCardDisplayMessage(event *handler.CommandEvent, resultMessage string, card *models.Card) error {
	// Use the existing CardDisplayService pattern
	config := h.bot.SpacesService.GetSpacesConfig()
	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		utils.GetGroupType(card.Tags),
		config,
	)

	timestamp := fmt.Sprintf("<t:%d:R>", time.Now().Unix())

	embed := discord.Embed{
		Title: "✅ Effect Used - Card Obtained!",
		Description: fmt.Sprintf("**%s**\n\n%s\n\n%s\n⭐ **Level:** %s\n🆔 **ID:** `%d`\n%s",
			resultMessage,
			cardInfo.FormattedName,
			cardInfo.FormattedCollection,
			strings.Repeat("⭐", card.Level),
			card.ID,
			utils.GetAnimatedTag(card.Animated)),
		Color: utils.SuccessColor,
		Image: &discord.EmbedResource{
			URL: cardInfo.ImageURL,
		},
		Footer: &discord.EmbedFooter{
			Text:    fmt.Sprintf("Used by %s • %s", event.User().Username, timestamp),
			IconURL: event.User().EffectiveAvatarURL(),
		},
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
		// Public message - no ephemeral flag
	})
}
