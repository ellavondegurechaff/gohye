package commands

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/paginator"
)

var Miss = discord.SlashCommandCreate{
	Name:        "miss",
	Description: "View cards you don't have in your collection",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "card_query",
			Description: "Filter missing cards by name, collection, or other attributes",
			Required:    false,
		},
	},
}

func MissHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Get all cards from database
		allCards, err := b.CardRepository.GetAll(ctx)
		if err != nil {
			return utils.EH.CreateErrorEmbed(e, "Failed to fetch cards")
		}

		// Get user's cards
		userCards, err := b.UserCardRepository.GetAllByUserID(ctx, e.User().ID.String())
		if err != nil {
			return utils.EH.CreateErrorEmbed(e, "Failed to fetch your cards")
		}

		// Create a map of owned card IDs for O(1) lookup
		ownedCards := make(map[int64]bool)
		for _, uc := range userCards {
			ownedCards[uc.CardID] = true
		}

		// Filter out owned cards to get missing cards
		var missingCards []*models.Card
		for _, card := range allCards {
			if !ownedCards[card.ID] {
				missingCards = append(missingCards, card)
			}
		}

		if len(missingCards) == 0 {
			return utils.EH.CreateErrorEmbed(e, "You own all available cards! 🎉")
		}

		// Apply search filter if provided
		query := strings.TrimSpace(e.SlashCommandInteractionData().String("card_query"))
		if query != "" {
			filteredCards := utils.WeightedSearch(missingCards, query, utils.SearchModePartial)
			if len(filteredCards) == 0 {
				return utils.EH.CreateErrorEmbed(e, fmt.Sprintf("No missing cards match the query: %s", query))
			}
			missingCards = filteredCards
		}

		// Sort cards by level (descending) and name (ascending)
		sort.Slice(missingCards, func(i, j int) bool {
			if missingCards[i].Level != missingCards[j].Level {
				return missingCards[i].Level > missingCards[j].Level
			}
			return missingCards[i].Name < missingCards[j].Name
		})

		totalPages := int(math.Ceil(float64(len(missingCards)) / float64(utils.CardsPerPage)))

		return b.Paginator.Create(e.Respond, paginator.Pages{
			ID:      e.ID().String(),
			Creator: e.User().ID,
			PageFunc: func(page int, embed *discord.EmbedBuilder) {
				startIdx := page * utils.CardsPerPage
				endIdx := min(startIdx+utils.CardsPerPage, len(missingCards))
				pageCards := missingCards[startIdx:endIdx]

				var description strings.Builder
				description.WriteString("```ansi\n")

				if query != "" {
					description.WriteString(fmt.Sprintf("Filtering by: \x1b[33m%s\x1b[0m\n\n", query))
				}

				for _, card := range pageCards {
					animatedIcon := ""
					if card.Animated {
						animatedIcon = "✨"
					}

					description.WriteString(fmt.Sprintf("%s \x1b[32m%s\x1b[0m%s [%s]\n",
						strings.Repeat("⭐", card.Level),
						utils.FormatCardName(card.Name),
						animatedIcon,
						strings.Trim(utils.FormatCollectionName(card.ColID), "[]"),
					))
				}

				description.WriteString("```")

				embed.
					SetTitle("Missing Cards").
					SetDescription(description.String()).
					SetColor(0x2B2D31).
					SetFooter(fmt.Sprintf("Page %d/%d • Total Missing: %d", page+1, totalPages, len(missingCards)), "")
			},
			Pages:      totalPages,
			ExpireMode: paginator.ExpireModeAfterLastUsage,
		}, false)
	}
}
