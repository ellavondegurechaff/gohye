package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var SearchCards = discord.SlashCommandCreate{
	Name:        "searchcards",
	Description: "🔍 Search through the card collection with various filters",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "name",
			Description: "Search by card name",
			Required:    false,
		},
		discord.ApplicationCommandOptionInt{
			Name:        "id",
			Description: "Search by card ID",
			Required:    false,
		},
		discord.ApplicationCommandOptionInt{
			Name:        "level",
			Description: "Filter by card level (1-5)",
			Required:    false,
			Choices: []discord.ApplicationCommandOptionChoiceInt{
				{Name: "⭐ Common", Value: 1},
				{Name: "⭐⭐ Uncommon", Value: 2},
				{Name: "⭐⭐⭐ Rare", Value: 3},
				{Name: "⭐⭐⭐⭐ Epic", Value: 4},
				{Name: "⭐⭐⭐⭐⭐ Legendary", Value: 5},
			},
		},
		discord.ApplicationCommandOptionString{
			Name:        "collection",
			Description: "Filter by collection ID",
			Required:    false,
		},
		discord.ApplicationCommandOptionString{
			Name:        "type",
			Description: "Filter by card type",
			Required:    false,
			Choices: []discord.ApplicationCommandOptionChoiceString{
				{Name: "👯‍♀️ Girl Groups", Value: "girlgroups"},
				{Name: "👯‍♂️ Boy Groups", Value: "boygroups"},
				{Name: "👤 Solo Artists", Value: "soloist"},
			},
		},
		discord.ApplicationCommandOptionBool{
			Name:        "animated",
			Description: "Filter animated cards only",
			Required:    false,
		},
	},
}

const cardsPerPage = 10

func SearchCardsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		// First, send a deferred response
		if err := e.DeferCreateMessage(false); err != nil {
			return fmt.Errorf("failed to defer response: %w", err)
		}

		// Extract search filters from command options
		filters := repositories.SearchFilters{
			Name:       e.SlashCommandInteractionData().String("name"),
			ID:         int64(e.SlashCommandInteractionData().Int("id")),
			Level:      int(e.SlashCommandInteractionData().Int("level")),
			Collection: e.SlashCommandInteractionData().String("collection"),
			Type:       e.SlashCommandInteractionData().String("type"),
			Animated:   e.SlashCommandInteractionData().Bool("animated"),
		}

		// Search cards with pagination
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cards, totalCount, err := b.CardRepository.Search(ctx, filters, 0, cardsPerPage)
		if err != nil {
			_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
				Embeds: &[]discord.Embed{
					{
						Title:       "❌ Search Failed",
						Description: fmt.Sprintf("```diff\n- Error: %v\n```", err),
						Color:       0xFF0000,
					},
				},
			})
			return err
		}

		if len(cards) == 0 {
			_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
				Embeds: &[]discord.Embed{
					{
						Title:       "❌ No Results Found",
						Description: "```diff\n- No cards match your search criteria\n```",
						Color:       0xFF0000,
						Footer: &discord.EmbedFooter{
							Text: "Try different search terms or filters",
						},
					},
				},
			})
			return err
		}

		// Create embed with search results
		embed := createSearchResultsEmbed(cards, filters, 1, totalCount)

		// Add pagination buttons if needed
		var components []discord.ContainerComponent
		if totalCount > cardsPerPage {
			components = []discord.ContainerComponent{
				discord.NewActionRow(
					discord.NewPrimaryButton("◀️ Previous", "search:prev:0"),
					discord.NewPrimaryButton("Next ▶️", "search:next:2"),
					discord.NewSecondaryButton("🔍 Refine Search", "search:refine"),
				),
			}
		}

		// Update the deferred response with results
		_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &components,
		})
		return err
	}
}

func createSearchResultsEmbed(cards []*models.Card, filters repositories.SearchFilters, page int, totalCount int) discord.Embed {
	var description strings.Builder
	description.WriteString("```md\n# Search Results\n")

	// Add active filters section
	if hasActiveFilters(filters) {
		description.WriteString("\n## Active Filters\n")
		if filters.Name != "" {
			description.WriteString(fmt.Sprintf("* Name: %s\n", filters.Name))
		}
		if filters.ID != 0 {
			description.WriteString(fmt.Sprintf("* ID: %d\n", filters.ID))
		}
		if filters.Level != 0 {
			description.WriteString(fmt.Sprintf("* Level: %s\n", getRarityName(filters.Level)))
		}
		if filters.Collection != "" {
			description.WriteString(fmt.Sprintf("* Collection: %s\n", filters.Collection))
		}
		if filters.Type != "" {
			description.WriteString(fmt.Sprintf("* Type: %s\n", formatCardType(filters.Type)))
		}
		if filters.Animated {
			description.WriteString("* Animated Only: Yes\n")
		}
	}

	description.WriteString("\n## Cards\n")
	for _, card := range cards {
		stars := strings.Repeat("⭐", card.Level)
		description.WriteString(fmt.Sprintf("* %s %s [%s] (#%d)\n",
			stars,
			utils.FormatCardName(card.Name),
			utils.FormatCollectionName(card.ColID),
			card.ID,
		))
	}

	totalPages := (totalCount + cardsPerPage - 1) / cardsPerPage
	description.WriteString(fmt.Sprintf("\n> Page %d of %d (%d total cards)\n", page, totalPages, totalCount))
	description.WriteString("```")

	return discord.Embed{
		Title:       "🔍 Card Search Results",
		Description: description.String(),
		Color:       0x00FF00,
		Footer: &discord.EmbedFooter{
			Text: "Use the buttons below to navigate or refine your search",
		},
	}
}

func hasActiveFilters(filters repositories.SearchFilters) bool {
	return filters.Name != "" ||
		filters.ID != 0 ||
		filters.Level != 0 ||
		filters.Collection != "" ||
		filters.Type != "" ||
		filters.Animated
}

func formatCardType(cardType string) string {
	switch cardType {
	case "girlgroups":
		return "👯‍♀️ Girl Groups"
	case "boygroups":
		return "👯‍♂️ Boy Groups"
	case "soloist":
		return "👤 Solo Artist"
	default:
		return cardType
	}
}
