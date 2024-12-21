package commands

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/paginator"
)

var Cards = discord.SlashCommandCreate{
	Name:        "cards",
	Description: "View your card collection",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "query",
			Description: "Search query (e.g., '5 gif winter -aespa >level')",
			Required:    false,
		},
	},
}

func CardsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(event *handler.CommandEvent) error {
		userCards, err := b.UserCardRepository.GetAllByUserID(context.Background(), event.User().ID.String())
		if err != nil {
			return utils.EH.CreateErrorEmbed(event, "Failed to fetch cards")
		}

		if len(userCards) == 0 {
			return utils.EH.CreateErrorEmbed(event, "No cards found")
		}

		// Get search query and parse filters
		query := strings.TrimSpace(event.SlashCommandInteractionData().String("query"))
		filters := utils.ParseSearchQuery(query)

		// Add logging
		fmt.Printf("Search Query: %q\n", query)
		fmt.Printf("Parsed Filters: %+v\n", filters)

		// Convert UserCards to Cards for searching
		var cards []*models.Card
		cardMap := make(map[int64]*models.UserCard) // Changed from string to int64 key

		for _, userCard := range userCards {
			cardData, err := b.CardRepository.GetByID(context.Background(), userCard.CardID)
			if err != nil {
				fmt.Printf("Error getting card %d: %v\n", userCard.CardID, err)
				continue
			}
			cards = append(cards, cardData)
			cardMap[cardData.ID] = userCard // Store with int64 key directly
		}

		fmt.Printf("Total cards before filtering: %d\n", len(cards))

		// Apply search filters
		var results []*models.Card
		if filters.MultiOnly {
			results = utils.WeightedSearchWithMulti(cards, filters, cardMap)
		} else {
			results = utils.WeightedSearch(cards, filters)
		}
		fmt.Printf("Cards after search filter: %d\n", len(results))

		// Apply favorites filter if requested (only for cards command)
		if filters.Favorites {
			var favoritedCards []*models.Card
			for _, card := range results {
				if userCard, ok := cardMap[card.ID]; ok {
					if userCard.Favorite {
						favoritedCards = append(favoritedCards, card)
					}
				}
			}
			results = favoritedCards
			fmt.Printf("Cards after favorites filter: %d\n", len(results))
		}

		if len(results) == 0 {
			return utils.EH.CreateErrorEmbed(event, "No cards match your search")
		}

		// Map filtered cards back to UserCards for display
		var displayCards []*models.UserCard
		for _, card := range results {
			for _, userCard := range userCards {
				if userCard.CardID == card.ID {
					displayCards = append(displayCards, userCard)
					break
				}
			}
		}

		totalPages := int(math.Ceil(float64(len(displayCards)) / float64(utils.CardsPerPage)))

		return b.Paginator.Create(event.Respond, paginator.Pages{
			ID:      event.ID().String(),
			Creator: event.User().ID,
			PageFunc: func(page int, embed *discord.EmbedBuilder) {
				startIdx := page * utils.CardsPerPage
				endIdx := min(startIdx+utils.CardsPerPage, len(displayCards))

				description := formatCardsDescription(b, displayCards[startIdx:endIdx])

				if query != "" {
					description = fmt.Sprintf("`🔍 %s`\n\n%s", query, description)
				}

				embed.
					SetTitle("My Collection").
					SetDescription(description).
					SetColor(0x2B2D31).
					SetFooter(fmt.Sprintf("Page %d/%d • Total: %d", page+1, totalPages, len(displayCards)), "")
			},
			Pages:      totalPages,
			ExpireMode: paginator.ExpireModeAfterLastUsage,
		}, false)
	}
}

// Format cards description for display
func formatCardsDescription(b *bottemplate.Bot, cards []*models.UserCard) string {
	var description strings.Builder

	for _, userCard := range cards {
		cardData, err := b.CardRepository.GetByID(context.Background(), userCard.CardID)
		if err != nil {
			continue
		}

		displayInfo := utils.GetCardDisplayInfo(
			cardData.Name,
			cardData.ColID,
			cardData.Level,
			utils.GetGroupType(cardData.Tags),
			b.SpacesService.GetSpacesConfig(),
		)

		description.WriteString(utils.FormatCardEntry(
			displayInfo,
			userCard.Favorite,
			cardData.Animated,
			int(userCard.Amount),
		))
		description.WriteString("\n")
	}

	return description.String()
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
