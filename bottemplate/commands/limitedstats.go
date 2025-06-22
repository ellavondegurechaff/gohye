package commands

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var LimitedStats = discord.SlashCommandCreate{
	Name:        "limitedstats",
	Description: "📊 View ownership statistics for limited collection cards",
}

type cardStat struct {
	*models.Card `bun:"embed:c"`
	ColID        string `bun:"col_id"`
	Owners       int64  `bun:"owners"`
}

func LimitedStatsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if err := e.DeferCreateMessage(false); err != nil {
			return fmt.Errorf("failed to defer message: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Get limited cards that have owners
		var stats []cardStat
		err := b.DB.BunDB().NewSelect().
			Model((*models.Card)(nil)).
			TableExpr("cards").
			ColumnExpr("DISTINCT cards.*, cards.col_id").
			ColumnExpr(`(
				SELECT COUNT(DISTINCT user_id)
				FROM user_cards 
				WHERE card_id = cards.id AND amount > 0
			) as owners`).
			Join("LEFT JOIN user_cards ON user_cards.card_id = cards.id").
			Where("cards.col_id = ?", "limited").
			Where("user_cards.id IS NOT NULL"). // Only cards with owners
			GroupExpr("cards.id").
			Having("COUNT(DISTINCT user_cards.user_id) > 0").
			OrderExpr("owners ASC, cards.level DESC, cards.name ASC").
			Scan(ctx, &stats)

		if err != nil {
			return fmt.Errorf("failed to fetch stats: %w", err)
		}

		if len(stats) == 0 {
			_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
				Embeds: &[]discord.Embed{{
					Description: "No limited cards have been claimed yet",
					Color:       utils.ErrorColor,
				}},
			})
			if err != nil {
				return fmt.Errorf("failed to send empty message: %w", err)
			}
			return nil
		}

		totalPages := int(math.Ceil(float64(len(stats)) / float64(utils.CardsPerPage)))
		startIdx := 0
		endIdx := min(utils.CardsPerPage, len(stats))

		description := formatLimitedStatsDescription(stats[startIdx:endIdx])

		embed := discord.NewEmbedBuilder().
			SetTitle("Limited Collection Statistics").
			SetDescription(description).
			SetColor(utils.SuccessColor).
			SetFooter(fmt.Sprintf("Page 1/%d • Total: %d owned limited cards", totalPages, len(stats)), "").
			Build()

		_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds: &[]discord.Embed{embed},
			Components: &[]discord.ContainerComponent{
				discord.NewActionRow(
					discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/limitedstats/prev/%s/0", e.User().ID.String())),
					discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/limitedstats/next/%s/0", e.User().ID.String())),
					discord.NewSecondaryButton("📋 Copy Page", fmt.Sprintf("/limitedstats/copy/%s/0", e.User().ID.String())),
				),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to send response: %w", err)
		}
		return nil
	}
}

func formatLimitedStatsDescription(stats []cardStat) string {
	var description strings.Builder
	description.WriteString("**📊 Limited Cards Ownership**\n\n")

	for _, stat := range stats {
		stars := utils.GetStarsDisplay(stat.Card.Level)
		ownerText := "owners"
		if stat.Owners == 1 {
			ownerText = "owner"
		}

		description.WriteString(fmt.Sprintf("%s **%s** `[#%d]` • `%d %s`\n",
			stars,
			utils.FormatCardName(stat.Card.Name),
			stat.Card.ID,
			stat.Owners,
			ownerText))
	}

	return description.String()
}

func LimitedStatsComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
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
		if userID != e.User().ID.String() {
			return e.CreateMessage(discord.MessageCreate{
				Content: "You cannot use these buttons",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		currentPage, err := strconv.Atoi(parts[4])
		if err != nil {
			return fmt.Errorf("invalid page number: %w", err)
		}

		var stats []cardStat
		err = b.DB.BunDB().NewSelect().
			Model((*models.Card)(nil)).
			TableExpr("cards").
			ColumnExpr("DISTINCT cards.*, cards.col_id").
			ColumnExpr(`(
				SELECT COUNT(DISTINCT user_id)
				FROM user_cards 
				WHERE card_id = cards.id AND amount > 0
			) as owners`).
			Join("LEFT JOIN user_cards ON user_cards.card_id = cards.id").
			Where("cards.col_id = ?", "limited").
			Where("user_cards.id IS NOT NULL").
			GroupExpr("cards.id").
			Having("COUNT(DISTINCT user_cards.user_id) > 0").
			OrderExpr("owners ASC, cards.level DESC, cards.name ASC").
			Scan(context.Background(), &stats)

		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Failed to fetch stats",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		totalPages := int(math.Ceil(float64(len(stats)) / float64(utils.CardsPerPage)))

		// Handle copy button
		if strings.HasPrefix(customID, "/limitedstats/copy/") {
			startIdx := currentPage * utils.CardsPerPage
			endIdx := min(startIdx+utils.CardsPerPage, len(stats))
			copyText := formatLimitedStatsCopyText(stats[startIdx:endIdx])
			return e.CreateMessage(discord.MessageCreate{
				Content: "```\n" + copyText + "```",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Calculate new page
		newPage := currentPage
		if strings.HasPrefix(customID, "/limitedstats/next/") {
			newPage = (currentPage + 1) % totalPages
		} else if strings.HasPrefix(customID, "/limitedstats/prev/") {
			newPage = (currentPage - 1 + totalPages) % totalPages
		}

		startIdx := newPage * utils.CardsPerPage
		endIdx := min(startIdx+utils.CardsPerPage, len(stats))

		description := formatLimitedStatsDescription(stats[startIdx:endIdx])

		embed := e.Message.Embeds[0]
		embed.Description = description
		embed.Footer.Text = fmt.Sprintf("Page %d/%d • Total: %d owned limited cards", newPage+1, totalPages, len(stats))

		_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds: &[]discord.Embed{embed},
			Components: &[]discord.ContainerComponent{
				discord.NewActionRow(
					discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/limitedstats/prev/%s/%d", userID, newPage)),
					discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/limitedstats/next/%s/%d", userID, newPage)),
					discord.NewSecondaryButton("📋 Copy Page", fmt.Sprintf("/limitedstats/copy/%s/%d", userID, newPage)),
				),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to update message: %w", err)
		}
		return nil
	}
}

func formatLimitedStatsCopyText(stats []cardStat) string {
	var sb strings.Builder
	for _, stat := range stats {
		sb.WriteString(fmt.Sprintf("%s [#%d] - %d owners\n",
			stat.Card.Name,
			stat.Card.ID,
			stat.Owners))
	}
	return sb.String()
}
