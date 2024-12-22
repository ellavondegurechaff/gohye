package commands

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
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

		// Sort cards by level (descending) and name (ascending)
		sort.Slice(missingCards, func(i, j int) bool {
			if missingCards[i].Level != missingCards[j].Level {
				return missingCards[i].Level > missingCards[j].Level
			}
			return strings.ToLower(missingCards[i].Name) < strings.ToLower(missingCards[j].Name)
		})

		totalPages := int(math.Ceil(float64(len(missingCards)) / float64(utils.CardsPerPage)))
		startIdx := 0
		endIdx := min(utils.CardsPerPage, len(missingCards))

		// Create the initial embed
		embed := discord.NewEmbedBuilder().
			SetTitle("Missing Cards").
			SetDescription(formatMissingCardsDescription(missingCards[startIdx:endIdx], b)).
			SetColor(0x2B2D31).
			SetFooter(fmt.Sprintf("Page 1/%d • Total Missing: %d", totalPages, len(missingCards)), "")

		// Apply search filter if provided
		query := strings.TrimSpace(e.SlashCommandInteractionData().String("card_query"))
		if query != "" {
			embed.SetDescription(fmt.Sprintf("🔍`%s`\n\n%s", query, embed.Description))
		}

		// Create the navigation buttons
		components := []discord.ContainerComponent{
			discord.NewActionRow(
				discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/miss/prev/%s/0", e.User().ID.String())),
				discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/miss/next/%s/0", e.User().ID.String())),
				discord.NewSecondaryButton("📋 Copy Page", fmt.Sprintf("/miss/copy/%s/0", e.User().ID.String())),
			),
		}

		return e.CreateMessage(discord.MessageCreate{
			Embeds:     []discord.Embed{embed.Build()},
			Components: components,
		})
	}
}

// Update formatMissingCardsDescription to match cards.go style
func formatMissingCardsDescription(cards []*models.Card, b *bottemplate.Bot) string {
	var description strings.Builder
	for _, card := range cards {
		// Get group type from tags
		groupType := utils.GetGroupType(card.Tags)

		displayInfo := utils.GetCardDisplayInfo(
			card.Name,
			card.ColID,
			card.Level,
			groupType,
			b.SpacesService.GetSpacesConfig(),
		)

		// Format card entry with hyperlink
		entry := utils.FormatCardEntry(
			displayInfo,
			false, // not favorite since it's missing
			card.Animated,
			0, // amount is 0 since it's missing
		)

		description.WriteString(entry + "\n")
	}
	return description.String()
}

// Update formatMissingCopyText to match cards.go style
func formatMissingCopyText(cards []*models.Card, b *bottemplate.Bot) string {
	var sb strings.Builder
	sb.WriteString("Missing Cards\n")

	for _, card := range cards {
		stars := strings.Repeat("★", card.Level)
		sb.WriteString(fmt.Sprintf("%s %s [%s]\n", stars, utils.FormatCardName(card.Name), card.ColID))
	}

	return sb.String()
}

// Add component handler for miss command
func MissComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		data := e.Data.(discord.ButtonInteractionData)
		customID := data.CustomID()

		parts := strings.Split(customID, "/")
		if len(parts) != 5 {
			return nil
		}

		userID := parts[3]
		currentPage, err := strconv.Atoi(parts[4])
		if err != nil {
			return nil
		}

		// Only the original user can interact
		if e.User().ID.String() != userID {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Only the command user can navigate through these cards.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Get all cards and user's cards again to ensure up-to-date data
		ctx := context.Background()
		allCards, err := b.CardRepository.GetAll(ctx)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Failed to fetch cards",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		userCards, err := b.UserCardRepository.GetAllByUserID(ctx, userID)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Failed to fetch your cards",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Create map of owned cards
		ownedCards := make(map[int64]bool)
		for _, uc := range userCards {
			ownedCards[uc.CardID] = true
		}

		// Get missing cards
		var missingCards []*models.Card
		for _, card := range allCards {
			if !ownedCards[card.ID] {
				missingCards = append(missingCards, card)
			}
		}

		// Sort missing cards
		sort.Slice(missingCards, func(i, j int) bool {
			if missingCards[i].Level != missingCards[j].Level {
				return missingCards[i].Level > missingCards[j].Level
			}
			return strings.ToLower(missingCards[i].Name) < strings.ToLower(missingCards[j].Name)
		})

		totalPages := int(math.Ceil(float64(len(missingCards)) / float64(utils.CardsPerPage)))

		// Handle copy button
		if strings.HasPrefix(customID, "/miss/copy/") {
			startIdx := currentPage * utils.CardsPerPage
			endIdx := min(startIdx+utils.CardsPerPage, len(missingCards))

			copyText := formatMissingCopyText(missingCards[startIdx:endIdx], b)

			return e.CreateMessage(discord.MessageCreate{
				Content: "```\n" + copyText + "```",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Calculate new page
		newPage := currentPage
		if strings.HasPrefix(customID, "/miss/next/") {
			newPage = (currentPage + 1) % totalPages
		} else if strings.HasPrefix(customID, "/miss/prev/") {
			newPage = (currentPage - 1 + totalPages) % totalPages
		}

		startIdx := newPage * utils.CardsPerPage
		endIdx := min(startIdx+utils.CardsPerPage, len(missingCards))

		// Update the embed
		embed := e.Message.Embeds[0]
		embed.Description = formatMissingCardsDescription(missingCards[startIdx:endIdx], b)
		embed.Footer.Text = fmt.Sprintf("Page %d/%d • Total Missing: %d", newPage+1, totalPages, len(missingCards))

		// Update the navigation buttons
		components := []discord.ContainerComponent{
			discord.NewActionRow(
				discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/miss/prev/%s/%d", userID, newPage)),
				discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/miss/next/%s/%d", userID, newPage)),
				discord.NewSecondaryButton("📋 Copy Page", fmt.Sprintf("/miss/copy/%s/%d", userID, newPage)),
			),
		}

		return e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &components,
		})
	}
}
