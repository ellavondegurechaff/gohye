package commands

import (
	"context"
	"fmt"
	"math"
	"sort"
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
	Options: append(utils.CommonFilterOptions,
		discord.ApplicationCommandOptionBool{
			Name:        "favorites",
			Description: "Show only favorite cards",
			Required:    false,
		},
	),
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

		// Extract filters into common structure
		filters := utils.FilterInfo{
			Name:       strings.TrimSpace(event.SlashCommandInteractionData().String("name")),
			Level:      int(event.SlashCommandInteractionData().Int("level")),
			Tags:       strings.TrimSpace(event.SlashCommandInteractionData().String("tags")),
			Collection: strings.TrimSpace(event.SlashCommandInteractionData().String("collection")),
			Animated:   event.SlashCommandInteractionData().Bool("animated"),
			Favorites:  event.SlashCommandInteractionData().Bool("favorites"),
		}

		var filteredCards []*models.UserCard
		for _, userCard := range userCards {
			cardData, err := b.CardRepository.GetByID(context.Background(), userCard.CardID)
			if err != nil {
				continue
			}

			// Apply filters
			if filters.Name != "" && !strings.Contains(strings.ToLower(cardData.Name), strings.ToLower(filters.Name)) {
				continue
			}
			if filters.Level != 0 && cardData.Level != filters.Level {
				continue
			}
			if filters.Tags != "" && !contains(cardData.Tags, filters.Tags) {
				continue
			}
			if filters.Collection != "" && !strings.Contains(strings.ToLower(cardData.ColID), strings.ToLower(filters.Collection)) {
				continue
			}
			if filters.Animated && !cardData.Animated {
				continue
			}
			if filters.Favorites && !userCard.Favorite {
				continue
			}

			filteredCards = append(filteredCards, userCard)
		}

		if len(filteredCards) == 0 {
			return utils.EH.CreateErrorEmbed(event, "No cards match your filters")
		}

		// Sort cards by level (descending) and then by name
		sort.Slice(filteredCards, func(i, j int) bool {
			cardI, _ := b.CardRepository.GetByID(context.Background(), filteredCards[i].CardID)
			cardJ, _ := b.CardRepository.GetByID(context.Background(), filteredCards[j].CardID)

			if cardI.Level != cardJ.Level {
				return cardI.Level > cardJ.Level // Descending order
			}
			return cardI.Name < cardJ.Name // Alphabetical order for same level
		})

		totalPages := int(math.Ceil(float64(len(filteredCards)) / float64(utils.CardsPerPage)))

		return b.Paginator.Create(event.Respond, paginator.Pages{
			ID:      event.ID().String(),
			Creator: event.User().ID,
			PageFunc: func(page int, embed *discord.EmbedBuilder) {
				startIdx := page * utils.CardsPerPage
				endIdx := min(startIdx+utils.CardsPerPage, len(filteredCards))

				description := formatCardsDescription(b, filteredCards[startIdx:endIdx])

				if utils.HasActiveFilters(filters) {
					description = utils.BuildFilterDescription(filters) + "\n\n" + description
				}

				embed.
					SetTitle("My Collection").
					SetDescription(description).
					SetColor(0x2B2D31).
					SetFooter(fmt.Sprintf("Page %d/%d • Total: %d", page+1, totalPages, len(filteredCards)), "")
			},
			Pages:      totalPages,
			ExpireMode: paginator.ExpireModeAfterLastUsage,
		}, false)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

		description.WriteString(fmt.Sprintf("* %s %s %s%s [%s]\n",
			starRating,
			utils.FormatCardName(cardData.Name),
			favoriteIcon,
			animatedIcon,
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
