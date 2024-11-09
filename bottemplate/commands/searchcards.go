package commands

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/paginator"
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
				{Name: "1", Value: 1},
				{Name: "2", Value: 2},
				{Name: "3", Value: 3},
				{Name: "4", Value: 4},
				{Name: "5", Value: 5},
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

		// Get total count first
		cards, totalCount, err := b.CardRepository.Search(ctx, filters, 0, cardsPerPage)
		if err != nil {
			return sendErrorEmbed(e, "Search Failed", err)
		}

		if len(cards) == 0 {
			return sendNoResultsEmbed(e)
		}

		// Calculate total pages
		totalPages := int(math.Ceil(float64(totalCount) / float64(cardsPerPage)))

		// Create paginator
		err = b.Paginator.Create(e.Respond, paginator.Pages{
			ID:      e.ID().String(),
			Creator: e.User().ID,
			PageFunc: func(page int, embed *discord.EmbedBuilder) {
				// Fetch cards for current page
				offset := page * cardsPerPage
				pageCards, _, _ := b.CardRepository.Search(context.Background(), filters, offset, cardsPerPage)

				// Build embed description
				description := buildSearchDescription(pageCards, filters, page+1, totalCount, totalPages)

				embed.
					SetTitle("🔍 Card Search Results").
					SetDescription(description).
					SetColor(0x00FF00).
					SetFooter("Use the buttons below to navigate or refine your search", "")
			},
			Pages:      totalPages,
			ExpireMode: paginator.ExpireModeAfterLastUsage,
		}, false)

		if err != nil {
			return fmt.Errorf("failed to create paginator: %w", err)
		}

		return nil
	}
}

func buildSearchDescription(cards []*models.Card, filters repositories.SearchFilters, currentPage, totalCount, totalPages int) string {
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
			description.WriteString(fmt.Sprintf("* Level: %d ⭐\n", filters.Level))
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
		// Format level with stars and remove double brackets and card ID
		description.WriteString(fmt.Sprintf("* %d ⭐ %s [%s]\n",
			card.Level,
			utils.FormatCardName(card.Name),
			strings.Trim(utils.FormatCollectionName(card.ColID), "[]"), // Remove double brackets
		))
	}

	description.WriteString(fmt.Sprintf("\n> Page %d of %d (%d total cards)\n", currentPage, totalPages, totalCount))
	description.WriteString("```")

	return description.String()
}

func sendErrorEmbed(e *handler.CommandEvent, title string, err error) error {
	_, err2 := e.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds: &[]discord.Embed{
			{
				Title:       "❌ " + title,
				Description: fmt.Sprintf("```diff\n- Error: %v\n```", err),
				Color:       0xFF0000,
			},
		},
	})
	return err2
}

func sendNoResultsEmbed(e *handler.CommandEvent) error {
	_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
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
