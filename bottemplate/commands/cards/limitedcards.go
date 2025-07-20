package cards

import (
	"context"
	"fmt"

	"log/slog"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/services"
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

		ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
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
					Color:       config.ErrorColor,
				}},
			})
			if err != nil {
				return fmt.Errorf("failed to send empty message: %w", err)
			}
			return nil
		}

		// Use service-based pagination
		cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)
		displayItems := cardDisplayService.ConvertCardsToLimitedDisplayItems(unownedCards)

		paginationHandler := &utils.PaginationHandler{
			Config: utils.PaginationConfig{
				ItemsPerPage: config.CardsPerPage,
				Prefix:       "limitedcards",
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
					"Limited Collection - Unowned Cards",
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
				copyText, _ := cardDisplayService.FormatCopyText(ctx, pageItems, "Limited Collection - Unowned Cards")
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

func LimitedCardsComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
		defer cancel()

		paginationHandler := &utils.PaginationHandler{
			Config: utils.PaginationConfig{
				ItemsPerPage: config.CardsPerPage,
				Prefix:       "limitedcards",
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
					"Limited Collection - Unowned Cards",
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
				copyText, _ := cardDisplayService.FormatCopyText(ctx, pageItems, "Limited Collection - Unowned Cards")
				return copyText
			},
			ValidateUser: func(eventUserID, targetUserID string) bool {
				return eventUserID == targetUserID
			},
		}

		return paginationHandler.HandlePagination(ctx, e, func(userID, query string) (*utils.PaginationData, error) {
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
				slog.Error("Failed to fetch limited cards",
					slog.String("error", err.Error()),
					slog.String("user_id", userID))
				return nil, fmt.Errorf("failed to fetch cards")
			}

			// Convert to display items using service
			cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)
			displayItems := cardDisplayService.ConvertCardsToLimitedDisplayItems(unownedCards)

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
