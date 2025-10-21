package social

import (
	"context"
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Has = discord.SlashCommandCreate{
	Name:        "has",
	Description: "Check if a user has a specific card",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionUser{
			Name:        "user",
			Description: "The user to check",
			Required:    true,
		},
		discord.ApplicationCommandOptionString{
			Name:        "card_query",
			Description: "Card to check (name or ID)",
			Required:    true,
		},
	},
}

func HasHandler(b *bottemplate.Bot) handler.CommandHandler {
    cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)

    return func(e *handler.CommandEvent) error {
        if err := e.DeferCreateMessage(false); err != nil { return err }
        ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
        defer cancel()

		// Get target user
		targetUser := e.SlashCommandInteractionData().User("user")
		query := e.SlashCommandInteractionData().String("card_query")

		// Try direct query first (optimized approach)
		var card *models.Card
		var err error

		// First try GetByQuery for exact matches
		if directCard, queryErr := b.CardRepository.GetByQuery(ctx, query); queryErr == nil {
			card = directCard
		} else {
			// Fallback to comprehensive search for fuzzy matches
			cards, getAllErr := b.CardRepository.GetAll(ctx)
            if getAllErr != nil { return utils.EH.UpdateInteractionResponse(e, "Error", "Failed to search for cards") }

			// Use enhanced search filters
			filters := utils.ParseSearchQuery(query)
			filters.SortBy = utils.SortByLevel
			filters.SortDesc = true

			searchResults := cardOperationsService.SearchCardsInCollection(ctx, cards, filters)
            if len(searchResults) == 0 { return utils.EH.UpdateInteractionResponse(e, "Not Found", fmt.Sprintf("No cards found matching '%s'", query)) }

			card = searchResults[0]
		}

		// Check if user has the card
		userCard, err := b.UserCardRepository.GetUserCard(ctx, targetUser.ID.String(), card.ID)
        var hasEmbed discord.Embed
        if err != nil { hasEmbed = createHasEmbed(targetUser, card, 0, false, b) } else { hasEmbed = createHasEmbed(targetUser, card, userCard.Amount, true, b) }
        _, updErr := e.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{hasEmbed}})
        return updErr
    }
}

func createHasEmbed(user discord.User, card *models.Card, amount int64, hasCard bool, b *bottemplate.Bot) discord.Embed {
	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		utils.GetGroupType(card.Tags),
		b.SpacesService.GetSpacesConfig(),
	)

	var description strings.Builder
	description.WriteString("```ansi\n")

	if hasCard {
		description.WriteString(fmt.Sprintf("✅ \x1b[32m%s owns this card!\x1b[0m\n", user.Username))
		description.WriteString(fmt.Sprintf("Amount owned: \x1b[33m%d\x1b[0m\n", amount))
	} else {
		description.WriteString(fmt.Sprintf("❌ \x1b[31m%s does not own this card\x1b[0m\n", user.Username))
	}

	description.WriteString(fmt.Sprintf("\n\x1b[33m%s\x1b[0m [%s] %s",
		cardInfo.FormattedName,
		strings.Repeat("⭐", card.Level),
		cardInfo.FormattedCollection))

	description.WriteString("\n```")

	return discord.NewEmbedBuilder().
		SetTitle("Card Ownership Check").
		SetDescription(description.String()).
		SetColor(utils.GetColorByLevel(card.Level)).
		SetThumbnail(cardInfo.ImageURL).
		Build()
}
