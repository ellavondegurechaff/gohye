package cards

import (
	"context"
	"fmt"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/services"
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

		ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
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
					Color:       config.ErrorColor,
				}},
			})
			if err != nil {
				return fmt.Errorf("failed to send empty message: %w", err)
			}
			return nil
		}

		// Convert stats to display items using service
		cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)
		displayItems := convertStatsToDisplayItems(stats)

		paginationHandler := &utils.PaginationHandler{
			Config: utils.PaginationConfig{
				ItemsPerPage: config.CardsPerPage,
				Prefix:       "limitedstats",
			},
			FormatItems: func(items []interface{}, page, totalPages int, userID, query string) (discord.Embed, error) {
				// Convert items to CardDisplayItem slice
				pageItems := make([]services.CardDisplayItem, len(items))
				for i, item := range items {
					pageItems[i] = item.(services.CardDisplayItem)
				}

				// Calculate total items from page data
				totalItems := totalPages * config.CardsPerPage
				if page == totalPages-1 && len(items) < config.CardsPerPage {
					// Last page might have fewer items
					totalItems = (totalPages-1)*config.CardsPerPage + len(items)
				}

				return cardDisplayService.CreateCardsEmbed(
					ctx,
					"Limited Collection Statistics",
					pageItems,
					page,
					totalPages,
					totalItems,
					query,
					config.SuccessColor,
				)
			},
			FormatCopy: func(items []interface{}) string {
				pageItems := make([]services.CardDisplayItem, len(items))
				for i, item := range items {
					pageItems[i] = item.(services.CardDisplayItem)
				}
				copyText, _ := cardDisplayService.FormatCopyText(ctx, pageItems, "Limited Collection Statistics")
				return copyText
			},
			ValidateUser: func(eventUserID, targetUserID string) bool {
				return eventUserID == targetUserID
			},
		}

		items := make([]interface{}, len(displayItems))
		for i, item := range displayItems {
			items[i] = item
		}

		embed, components, err := paginationHandler.CreateInitialPaginationEmbed(items, e.User().ID.String(), "")
		if err != nil {
			return fmt.Errorf("failed to create pagination: %w", err)
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

// convertStatsToDisplayItems converts cardStat slice to CardDisplayItem slice for service
func convertStatsToDisplayItems(stats []cardStat) []services.CardDisplayItem {
	items := make([]services.CardDisplayItem, len(stats))
	for i, stat := range stats {
		items[i] = &services.LimitedStatsDisplay{
			Card:   stat.Card,
			Owners: stat.Owners,
		}
	}
	return items
}

func LimitedStatsComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
		defer cancel()

		paginationHandler := &utils.PaginationHandler{
			Config: utils.PaginationConfig{
				ItemsPerPage: config.CardsPerPage,
				Prefix:       "limitedstats",
			},
			FormatItems: func(items []interface{}, page, totalPages int, userID, query string) (discord.Embed, error) {
				startIdx := page * config.CardsPerPage
				endIdx := startIdx + config.CardsPerPage
				if endIdx > len(items) {
					endIdx = len(items)
				}
				pageItems := make([]services.CardDisplayItem, endIdx-startIdx)
				for i, item := range items[startIdx:endIdx] {
					pageItems[i] = item.(services.CardDisplayItem)
				}

				cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)
				return cardDisplayService.CreateCardsEmbed(
					ctx,
					"Limited Collection Statistics",
					pageItems,
					page,
					totalPages,
					len(items),
					query,
					config.SuccessColor,
				)
			},
			FormatCopy: func(items []interface{}) string {
				pageItems := make([]services.CardDisplayItem, len(items))
				for i, item := range items {
					pageItems[i] = item.(services.CardDisplayItem)
				}
				cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)
				copyText, _ := cardDisplayService.FormatCopyText(ctx, pageItems, "Limited Collection Statistics")
				return copyText
			},
			ValidateUser: func(eventUserID, targetUserID string) bool {
				return eventUserID == targetUserID
			},
		}

		return paginationHandler.HandlePagination(ctx, e, func(userID, query string) (*utils.PaginationData, error) {
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
				Where("user_cards.id IS NOT NULL").
				GroupExpr("cards.id").
				Having("COUNT(DISTINCT user_cards.user_id) > 0").
				OrderExpr("owners ASC, cards.level DESC, cards.name ASC").
				Scan(ctx, &stats)

			if err != nil {
				return nil, fmt.Errorf("failed to fetch stats")
			}

			// Convert to display items using service
			displayItems := convertStatsToDisplayItems(stats)

			items := make([]interface{}, len(displayItems))
			for i, item := range displayItems {
				items[i] = item
			}

			return &utils.PaginationData{
				Items:      items,
				TotalItems: len(items),
				UserID:     userID,
				Query:      query,
			}, nil
		})
	}
}
