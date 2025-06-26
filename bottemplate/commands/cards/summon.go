package cards

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Summon = discord.SlashCommandCreate{
	Name:        "summon",
	Description: "✨ Summon a card from your collection",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "name",
			Description: "Name of the card to summon from your collection",
			Required:    true,
		},
	},
}

func SummonHandler(b *bottemplate.Bot) handler.CommandHandler {
	cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)
	cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)

	return func(e *handler.CommandEvent) error {
		cardName := e.SlashCommandInteractionData().String("name")
		userID := e.User().ID.String()

		ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
		defer cancel()

		// Use CardOperationsService to get user cards with search applied
		userCards, cards, err := cardOperationsService.GetUserCardsWithDetails(ctx, userID, cardName)
		if err != nil {
			return utils.EH.CreateSystemError(e, "Failed to fetch your collection")
		}

		// Find the first matching card from filtered results
		var matchedCard *models.Card
		if len(userCards) > 0 && len(cards) > 0 {
			// Build card mappings for efficient lookup
			_, cardMap := cardOperationsService.BuildCardMappings(userCards, cards)
			
			// Find first card with amount > 0
			for _, userCard := range userCards {
				if userCard.Amount > 0 {
					if card, exists := cardMap[userCard.CardID]; exists {
						matchedCard = card
						break
					}
				}
			}
		}

		if matchedCard == nil {
			return utils.EH.CreateNotFoundError(e, "card", cardName)
		}

		return displayCard(e, matchedCard, b, cardDisplayService)
	}
}

// displayCard handles the card display logic
func displayCard(e *handler.CommandEvent, card *models.Card, b *bottemplate.Bot, _ *services.CardDisplayService) error {
	// Use the existing CardDisplayService pattern
	config := b.SpacesService.GetSpacesConfig()
	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		utils.GetGroupType(card.Tags),
		config,
	)

	timestamp := fmt.Sprintf("<t:%d:R>", time.Now().Unix())

	embed := discord.Embed{
		Title: cardInfo.FormattedName,
		Color: utils.GetColorByLevel(card.Level),
		Description: fmt.Sprintf("```md\n"+
			"# Card Information\n"+
			"* Collection: %s\n"+
			"* Level: %s\n"+
			"* ID: #%d\n"+
			"%s\n"+
			"```\n"+
			"> %s\n\n"+
			"Use `/inventory` to view your collection",
			cardInfo.FormattedCollection,
			strings.Repeat("⭐", card.Level),
			card.ID,
			utils.GetAnimatedTag(card.Animated),
			getCardQuote(card.Level)),
		Image: &discord.EmbedResource{
			URL: cardInfo.ImageURL,
		},
		Footer: &discord.EmbedFooter{
			Text:    fmt.Sprintf("Summoned by %s • %s", e.User().Username, timestamp),
			IconURL: e.User().EffectiveAvatarURL(),
		},
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
}

// getCardQuote returns an inspirational quote based on card level
func getCardQuote(level int) string {
	quotes := []string{
		"A humble beginning to your collection journey.",
		"An uncommon find, growing in power.",
		"A rare gem shines in your collection!",
		"An epic discovery that few possesses!",
		"A legendary artifact of immense power!",
	}
	if level >= 1 && level <= 5 {
		return quotes[level-1]
	}
	return "A mysterious card of unknown origin."
}
