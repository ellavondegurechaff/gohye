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
	card1, err := h.findCard(ctx, query1, userID, 0)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Error finding first card: %v", err),
		})
	}

	// Find second card, excluding the first card
	card2, err := h.findCard(ctx, query2, userID, card1.ID)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Error finding second card: %v", err),
		})
	}

	// Validate cards can be forged
	if card1.Level != card2.Level {
		return e.CreateMessage(discord.MessageCreate{
			Content: "❌ Cards must be of the same level to forge",
		})
	}

	if card1.ID == card2.ID {
		return e.CreateMessage(discord.MessageCreate{
			Content: "❌ You must use two different cards to forge",
		})
	}

	// Calculate forge cost
	cost, err := fm.CalculateForgeCost(ctx, card1, card2)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("❌ Error calculating forge cost: %v", err),
		})
	}

	return h.showForgeConfirmation(e, card1, card2, cost)
}

func (h *ForgeHandler) showForgeConfirmation(e *handler.CommandEvent, card1, card2 *models.Card, cost int64) error {
	// Get group type from card tags
	groupType := "girlgroups" // default
	for _, tag := range card1.Tags {
		if tag == "boygroups" {
			groupType = "boygroups"
			break
		}
	}

	config := utils.SpacesConfig{
		Bucket:   h.bot.SpacesService.GetBucket(),
		Region:   h.bot.SpacesService.GetRegion(),
		CardRoot: h.bot.SpacesService.GetCardRoot(),
		GetImageURL: func(cardName string, colID string, level int, groupType string) string {
			return h.bot.SpacesService.GetCardImageURL(cardName, colID, level, groupType)
		},
	}

	card1Display := utils.GetCardDisplayInfo(card1.Name, card1.ColID, card1.Level, groupType, config)
	card2Display := utils.GetCardDisplayInfo(card2.Name, card2.ColID, card2.Level, groupType, config)

	embed := discord.NewEmbedBuilder().
		SetTitle("⚔️ Confirm Forging").
		SetColor(0x2b2d31).
		SetDescription(fmt.Sprintf("# Forge Details\n\n"+
			"## Selected Cards\n"+
			"• [%s](%s) %s\n"+
			"• [%s](%s) %s\n\n"+
			"## Forge Information\n"+
			"• Level: %s\n"+
			"• Cost: %d 💰\n"+
			"• Result: Random %s card%s\n\n"+
			"⚠️ **Warning:** This action cannot be undone!",
			card1Display.FormattedName, card1Display.ImageURL, card1Display.FormattedCollection,
			card2Display.FormattedName, card2Display.ImageURL, card2Display.FormattedCollection,
			strings.Repeat("⭐", card1.Level),
			cost,
			strings.Repeat("⭐", card1.Level),
			getSameCollectionBonus(card1, card2))).
		SetTimestamp(time.Now()).
		Build()

	actionRow := discord.NewActionRow(
		discord.NewSuccessButton("Confirm", fmt.Sprintf("/forge/confirm/%d/%d", card1.ID, card2.ID)),
		discord.NewDangerButton("Cancel", fmt.Sprintf("/forge/cancel/%d/%d", card1.ID, card2.ID)),
	)

	return e.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: []discord.ContainerComponent{actionRow},
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

		// Get group type from card tags
		groupType := "girlgroups" // default
		for _, tag := range newCard.Tags {
			if tag == "boygroups" {
				groupType = "boygroups"
				break
			}
		}

		config := h.bot.SpacesService.GetSpacesConfig()
		cardDisplay := utils.GetCardDisplayInfo(newCard.Name, newCard.ColID, newCard.Level, groupType, config)

		embed := discord.NewEmbedBuilder().
			SetTitle("⚔️ Forge Successful").
			SetColor(0x57F287).
			SetDescription(fmt.Sprintf("## Result\n"+
				"• Name: [%s](%s)\n"+
				"• Collection: %s\n"+
				"• Level: %s",
				cardDisplay.FormattedName,
				cardDisplay.ImageURL,
				cardDisplay.FormattedCollection,
				strings.Repeat("⭐", newCard.Level))).
			SetImage(cardDisplay.ImageURL).
			SetTimestamp(time.Now()).
			Build()

		return e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &[]discord.ContainerComponent{},
		})

	case "cancel":
		embed := discord.NewEmbedBuilder().
			SetTitle("Forge Cancelled").
			SetDescription("The forging process has been cancelled.").
			SetColor(0xED4245).
			SetTimestamp(time.Now()).
			Build()

		return e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &[]discord.ContainerComponent{},
		})

	default:
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Invalid action"),
			Components: &[]discord.ContainerComponent{},
		})
	}
}

func (h *ForgeHandler) findCard(ctx context.Context, query, userID string, excludeCardID int64) (*models.Card, error) {
	// Handle empty query
	if query == "" {
		return nil, fmt.Errorf("please provide a card name")
	}

	// Get all user's cards first
	userCards, err := h.bot.UserCardRepository.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error fetching your cards: %v", err)
	}

	// Convert UserCards to Cards for searching
	var cards []*models.Card
	for _, userCard := range userCards {
		if userCard.Amount <= 0 {
			continue
		}
		cardData, err := h.bot.CardRepository.GetByID(ctx, userCard.CardID)
		if err != nil {
			continue
		}
		if cardData.ID == excludeCardID {
			continue
		}
		cards = append(cards, cardData)
	}

	// Try filter-based search first (for queries like "-2")
	if strings.HasPrefix(query, "-") {
		filters := utils.ParseSearchQuery(query)
		searchResults := utils.WeightedSearch(cards, filters)
		if len(searchResults) > 0 {
			return searchResults[0], nil
		}
	}

	// Try direct name search
	normalizedQuery := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(query, "_", " ")))
	queryWords := strings.Fields(normalizedQuery)

	// First try exact match
	for _, card := range cards {
		cardName := strings.ToLower(strings.ReplaceAll(card.Name, "_", " "))
		if cardName == normalizedQuery {
			return card, nil
		}
	}

	// Then try matching all words in any order
	for _, card := range cards {
		cardName := strings.ToLower(strings.ReplaceAll(card.Name, "_", " "))
		allWordsMatch := true
		for _, word := range queryWords {
			if !strings.Contains(cardName, word) {
				allWordsMatch = false
				break
			}
		}
		if allWordsMatch {
			return card, nil
		}
	}

	// If no direct matches found, try weighted search as last resort
	filters := utils.ParseSearchQuery(query)
	searchResults := utils.WeightedSearch(cards, filters)
	if len(searchResults) > 0 {
		return searchResults[0], nil
	}

	return nil, fmt.Errorf("no matching cards found in your collection")
}

func getSameCollectionBonus(card1, card2 *models.Card) string {
	if card1.ColID == card2.ColID {
		return fmt.Sprintf(" from %s collection", card1.ColID)
	}
	return ""
}
