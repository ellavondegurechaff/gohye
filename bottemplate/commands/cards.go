package commands

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
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

		// Convert UserCards to Cards for searching
		var cards []*models.Card
		cardMap := make(map[int64]*models.UserCard)

		for _, userCard := range userCards {
			cardData, err := b.CardRepository.GetByID(context.Background(), userCard.CardID)
			if err != nil {
				continue
			}
			cards = append(cards, cardData)
			cardMap[cardData.ID] = userCard
		}

		var displayCards []*models.UserCard
		if len(query) > 0 {
			// Apply search filters if query exists
			var results []*models.Card
			if filters.MultiOnly {
				results = utils.WeightedSearchWithMulti(cards, filters, cardMap)
			} else {
				results = utils.WeightedSearch(cards, filters)
			}

			// Map filtered cards back to UserCards
			for _, card := range results {
				if userCard, ok := cardMap[card.ID]; ok {
					displayCards = append(displayCards, userCard)
				}
			}
		} else {
			// If no query, use all cards but sort them
			displayCards = userCards
			sortUserCards(displayCards, b)
		}

		if len(displayCards) == 0 {
			return utils.EH.CreateErrorEmbed(event, "No cards match your search")
		}

		totalPages := int(math.Ceil(float64(len(displayCards)) / float64(utils.CardsPerPage)))

		// Create the initial embed
		embed := discord.NewEmbedBuilder().
			SetTitle("My Collection").
			SetDescription(formatCardsDescription(b, displayCards[0:min(utils.CardsPerPage, len(displayCards))])).
			SetColor(0x2B2D31).
			SetFooter(fmt.Sprintf("Page 1/%d • Total: %d", totalPages, len(displayCards)), "")

		if query != "" {
			embed.SetDescription(fmt.Sprintf("`🔍 %s`\n\n%s", query, embed.Description))
		}

		// Create the navigation buttons
		components := []discord.ContainerComponent{
			discord.NewActionRow(
				discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/cards/prev/%s/0", event.User().ID.String())),
				discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/cards/next/%s/0", event.User().ID.String())),
			),
		}

		return event.CreateMessage(discord.MessageCreate{
			Embeds:     []discord.Embed{embed.Build()},
			Components: components,
		})
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

// Add a new ComponentHandler for the cards navigation
func CardsComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		data := e.Data.(discord.ButtonInteractionData)
		customID := data.CustomID()

		slog.Info("Cards component interaction received",
			slog.String("custom_id", customID),
			slog.String("user_id", e.User().ID.String()))

		parts := strings.Split(customID, "/")
		if len(parts) != 5 {
			slog.Error("Invalid custom ID format",
				slog.String("custom_id", customID),
				slog.Int("parts_length", len(parts)))
			return nil
		}

		userID := parts[3]
		currentPage, err := strconv.Atoi(parts[4])
		if err != nil {
			slog.Error("Failed to parse page number",
				slog.String("page_str", parts[4]),
				slog.String("error", err.Error()))
			return nil
		}

		// Only the collection owner can navigate
		if e.User().ID.String() != userID {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Only the collection owner can navigate through these cards.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Get the user's cards again
		userCards, err := b.UserCardRepository.GetAllByUserID(context.Background(), userID)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Failed to fetch cards",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Sort the cards using the same logic
		sortUserCards(userCards, b)

		totalPages := int(math.Ceil(float64(len(userCards)) / float64(utils.CardsPerPage)))

		// Calculate new page
		newPage := currentPage
		if strings.HasPrefix(customID, "/cards/next/") {
			newPage = (currentPage + 1) % totalPages
		} else if strings.HasPrefix(customID, "/cards/prev/") {
			newPage = (currentPage - 1 + totalPages) % totalPages
		}

		// Calculate start and end indices for the current page
		startIdx := newPage * utils.CardsPerPage
		endIdx := min(startIdx+utils.CardsPerPage, len(userCards))

		// Update the embed
		embed := e.Message.Embeds[0]
		embed.Description = formatCardsDescription(b, userCards[startIdx:endIdx])
		embed.Footer.Text = fmt.Sprintf("Page %d/%d • Total: %d", newPage+1, totalPages, len(userCards))

		// Update the navigation buttons
		components := []discord.ContainerComponent{
			discord.NewActionRow(
				discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/cards/prev/%s/%d", userID, newPage)),
				discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/cards/next/%s/%d", userID, newPage)),
			),
		}

		return e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &components,
		})
	}
}

// Add this sorting function to match search_utils.go logic
func sortUserCards(cards []*models.UserCard, b *bottemplate.Bot) {
	sort.Slice(cards, func(i, j int) bool {
		cardI, errI := b.CardRepository.GetByID(context.Background(), cards[i].CardID)
		cardJ, errJ := b.CardRepository.GetByID(context.Background(), cards[j].CardID)

		// Handle errors by putting cards with errors at the end
		if errI != nil || errJ != nil {
			return errJ != nil
		}

		// Primary sort by level (descending)
		if cardI.Level != cardJ.Level {
			return cardI.Level > cardJ.Level // Descending order for levels
		}

		// Secondary sort by name (ascending)
		return strings.ToLower(cardI.Name) < strings.ToLower(cardJ.Name)
	})
}
