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
		cardMap := make(map[string]*models.UserCard) // Map to store UserCard data

		for _, userCard := range userCards {
			cardData, err := b.CardRepository.GetByID(context.Background(), userCard.CardID)
			if err != nil {
				fmt.Printf("Error getting card %d: %v\n", userCard.CardID, err)
				continue
			}
			cards = append(cards, cardData)
			cardMap[fmt.Sprintf("%d", cardData.ID)] = userCard // Convert ID to string
		}

		fmt.Printf("Total cards before filtering: %d\n", len(cards))

		// Apply search filters
		filteredCards := utils.WeightedSearch(cards, filters)
		fmt.Printf("Cards after search filter: %d\n", len(filteredCards))

		// Apply favorites filter if requested (only for cards command)
		if filters.Favorites {
			var favoritedCards []*models.Card
			for _, card := range filteredCards {
				cardIDStr := fmt.Sprintf("%d", card.ID)
				if userCard, ok := cardMap[cardIDStr]; ok {
					fmt.Printf("Checking favorite for card %s: %v\n", cardIDStr, userCard.Favorite)
					if userCard.Favorite {
						favoritedCards = append(favoritedCards, card)
					}
				}
			}
			filteredCards = favoritedCards
			fmt.Printf("Cards after favorites filter: %d\n", len(filteredCards))
		}

		if len(filteredCards) == 0 {
			return utils.EH.CreateErrorEmbed(event, "No cards match your search")
		}

		// Map filtered cards back to UserCards for display
		var displayCards []*models.UserCard
		for _, card := range filteredCards {
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
					description = fmt.Sprintf("```md\n# Search Query\n* %s\n```\n\n%s", query, description)
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
	description.WriteString("```md\n")

	for _, userCard := range cards {
		cardData, err := b.CardRepository.GetByID(context.Background(), userCard.CardID)
		if err != nil {
			continue
		}

		// Format the card entry
		starRating := strings.Repeat("⭐", cardData.Level)
		favoriteIcon := ""
		if userCard.Favorite {
			favoriteIcon = "❤️"
		}

		animatedIcon := ""
		if cardData.Animated {
			animatedIcon = "✨"
		}

		// Add amount if more than 1
		amountText := ""
		if userCard.Amount > 1 {
			amountText = fmt.Sprintf(" x%d", userCard.Amount)
		}

		description.WriteString(fmt.Sprintf("* %s %s %s%s%s [%s]\n",
			starRating,
			utils.FormatCardName(cardData.Name),
			favoriteIcon,
			animatedIcon,
			amountText,
			strings.Trim(utils.FormatCollectionName(cardData.ColID), "[]"),
		))
	}

	description.WriteString("```")
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
