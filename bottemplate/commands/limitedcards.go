package commands

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var LimitedCards = discord.SlashCommandCreate{
	Name:        "limitedcards",
	Description: "🎴 List all unowned cards from limited collection",
}

func LimitedCardsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if err := e.DeferCreateMessage(false); err != nil {
			return fmt.Errorf("failed to defer message: %w", err)
		}

		// Match Discord's interaction timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		slog.Info("Fetching unowned limited cards",
			slog.String("user_id", e.User().ID.String()))

		// Get limited cards that have no owners at all
		var unownedCards []*models.Card
		err := b.DB.BunDB().NewSelect().
			Model((*models.Card)(nil)).
			TableExpr("cards").
			ColumnExpr("DISTINCT cards.*").
			Join("LEFT JOIN user_cards ON user_cards.card_id = cards.id").
			Where("cards.col_id = ?", "limited").
			Where("user_cards.id IS NULL").
			OrderExpr("cards.level DESC, cards.name ASC").
			Scan(ctx, &unownedCards)

		if err != nil {
			slog.Error("Failed to fetch limited card IDs",
				slog.String("error", err.Error()),
				slog.String("user_id", e.User().ID.String()))
			return utils.EH.CreateErrorEmbed(e, "Failed to fetch cards")
		}

		if len(unownedCards) == 0 {
			_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
				Embeds: &[]discord.Embed{{
					Description: "All limited cards are currently owned",
					Color:       utils.ErrorColor,
				}},
			})
			if err != nil {
				return fmt.Errorf("failed to send empty message: %w", err)
			}
			return nil
		}

		totalPages := int(math.Ceil(float64(len(unownedCards)) / float64(utils.CardsPerPage)))
		startIdx := 0
		endIdx := min(utils.CardsPerPage, len(unownedCards))

		// Build the response
		description := formatUnownedCardsDescription(unownedCards[startIdx:endIdx])

		embed := discord.NewEmbedBuilder().
			SetTitle("Limited Collection - Unowned Cards").
			SetDescription(description).
			SetColor(utils.SuccessColor).
			SetFooter(fmt.Sprintf("Page 1/%d • Total: %d unowned cards", totalPages, len(unownedCards)), "").
			Build()

		components := []discord.ContainerComponent{
			discord.NewActionRow(
				discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/limitedcards/prev/%s/0", e.User().ID.String())),
				discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/limitedcards/next/%s/0", e.User().ID.String())),
				discord.NewSecondaryButton("📋 Copy Page", fmt.Sprintf("/limitedcards/copy/%s/0", e.User().ID.String())),
			),
		}

		_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &components,
		})
		if err != nil {
			return fmt.Errorf("failed to send response: %w", err)
		}
		return nil
	}
}

func formatUnownedCardsDescription(cards []*models.Card) string {
	var description strings.Builder
	description.WriteString("**🎴 Unowned Limited Cards**\n\n")

	for _, card := range cards {
		stars := utils.GetStarsDisplay(card.Level)
		description.WriteString(fmt.Sprintf("%s **%s** `[#%d]`\n",
			stars,
			utils.FormatCardName(card.Name),
			card.ID))
	}

	return description.String()
}

func LimitedCardsComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		// Defer the update immediately
		if err := e.DeferUpdateMessage(); err != nil {
			return fmt.Errorf("failed to defer message: %w", err)
		}

		data := e.Data.(discord.ButtonInteractionData)
		customID := data.CustomID()

		parts := strings.Split(customID, "/")
		if len(parts) != 5 {
			return fmt.Errorf("invalid component ID format")
		}

		userID := parts[3]
		currentPage, err := strconv.Atoi(parts[4])
		if err != nil {
			return fmt.Errorf("failed to parse page number: %w", err)
		}

		if e.User().ID.String() != userID {
			return e.CreateMessage(discord.MessageCreate{
				Content: "You cannot use these buttons",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		var unownedCards []*models.Card
		err = b.DB.BunDB().NewSelect().
			Model((*models.Card)(nil)).
			TableExpr("cards").
			ColumnExpr("DISTINCT cards.*").
			Join("LEFT JOIN user_cards ON user_cards.card_id = cards.id").
			Where("cards.col_id = ?", "limited").
			Where("user_cards.id IS NULL").
			OrderExpr("cards.level DESC, cards.name ASC").
			Scan(context.Background(), &unownedCards)

		if err != nil {
			slog.Error("Failed to fetch limited cards",
				slog.String("error", err.Error()),
				slog.String("user_id", e.User().ID.String()))
			return e.CreateMessage(discord.MessageCreate{
				Content: "Failed to fetch cards",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		totalPages := int(math.Ceil(float64(len(unownedCards)) / float64(utils.CardsPerPage)))

		// Handle copy button
		if strings.HasPrefix(customID, "/limitedcards/copy/") {
			startIdx := currentPage * utils.CardsPerPage
			endIdx := min(startIdx+utils.CardsPerPage, len(unownedCards))
			copyText := formatUnownedCardsCopyText(unownedCards[startIdx:endIdx])
			return e.CreateMessage(discord.MessageCreate{
				Content: "```\n" + copyText + "```",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Calculate new page
		newPage := currentPage
		if strings.HasPrefix(customID, "/limitedcards/next/") {
			newPage = (currentPage + 1) % totalPages
		} else if strings.HasPrefix(customID, "/limitedcards/prev/") {
			newPage = (currentPage - 1 + totalPages) % totalPages
		}

		startIdx := newPage * utils.CardsPerPage
		endIdx := min(startIdx+utils.CardsPerPage, len(unownedCards))

		description := formatUnownedCardsDescription(unownedCards[startIdx:endIdx])

		embed := e.Message.Embeds[0]
		embed.Description = description
		embed.Footer.Text = fmt.Sprintf("Page %d/%d • Total: %d unowned cards", newPage+1, totalPages, len(unownedCards))

		_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds: &[]discord.Embed{embed},
			Components: &[]discord.ContainerComponent{
				discord.NewActionRow(
					discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/limitedcards/prev/%s/%d", userID, newPage)),
					discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/limitedcards/next/%s/%d", userID, newPage)),
					discord.NewSecondaryButton("📋 Copy Page", fmt.Sprintf("/limitedcards/copy/%s/%d", userID, newPage)),
				),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to update message: %w", err)
		}
		return nil
	}
}

func formatUnownedCardsCopyText(cards []*models.Card) string {
	var sb strings.Builder
	for _, card := range cards {
		sb.WriteString(fmt.Sprintf("%s [#%d]\n",
			card.Name,
			card.ID))
	}
	return sb.String()
}
