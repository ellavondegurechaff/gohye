package commands

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var priceStats = discord.SlashCommandCreate{
	Name:        "price-stats",
	Description: "📊 View detailed price statistics for a card",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionInt{
			Name:        "card_id",
			Description: "The ID of the card to check",
			Required:    true,
		},
	},
}

var inlineFalse = false

func PriceStatsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(event *handler.CommandEvent) error {
		cardID := int64(event.SlashCommandInteractionData().Int("card_id"))
		log.Printf("[GoHYE] [%s] [INFO] [PRICE-STATS] Starting price stats calculation for card ID: %d",
			time.Now().Format("15:04:05"),
			cardID,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Get the latest market history entry
		var history models.CardMarketHistory
		err := b.DB.BunDB().NewSelect().
			Model(&history).
			Where("card_id = ?", cardID).
			Order("timestamp DESC").
			Limit(1).
			Scan(ctx)

		if err != nil {
			log.Printf("[GoHYE] [%s] [ERROR] [PRICE-STATS] Market history lookup failed: %v",
				time.Now().Format("15:04:05"),
				err,
			)
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: "```diff\n- Failed to fetch market data\n```",
					Color:       0xFF0000,
				}},
			})
		}

		// Get the card details
		card, err := b.CardRepository.GetByID(ctx, cardID)
		if err != nil {
			log.Printf("[GoHYE] [%s] [ERROR] [PRICE-STATS] Card lookup failed: %v",
				time.Now().Format("15:04:05"),
				err,
			)
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Card Not Found",
					Description: fmt.Sprintf("```diff\n- Card #%d does not exist\n```", cardID),
					Color:       0xFF0000,
				}},
			})
		}

		// Get market stats
		stats, err := b.PriceCalculator.GetMarketStats(ctx, cardID, history.Price)
		if err != nil {
			log.Printf("[GoHYE] [%s] [ERROR] [PRICE-STATS] Market stats calculation failed: %v",
				time.Now().Format("15:04:05"),
				err,
			)
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: "```diff\n- Failed to fetch market statistics\n```",
					Color:       0xFF0000,
				}},
			})
		}

		cardInfo := utils.GetCardDisplayInfo(
			card.Name,
			card.ColID,
			card.Level,
			getGroupType(card.Tags),
			utils.SpacesConfig{
				Bucket:   b.SpacesService.GetBucket(),
				Region:   b.SpacesService.GetRegion(),
				CardRoot: b.SpacesService.GetCardRoot(),
			},
		)

		timestamp := fmt.Sprintf("<t:%d:R>", time.Now().Unix())
		inlineTrue := true

		// Format market status
		marketStatus := "🟢 Active"
		if !history.IsActive {
			marketStatus = "🔴 Inactive"
		}

		// Format price factors explanation
		priceFactors := fmt.Sprintf("```md\n"+
			"# Price Factors\n"+
			"* Scarcity: %.2fx\n"+
			"* Distribution: %.2fx\n"+
			"* Hoarding: %.2fx\n"+
			"* Activity: %.2fx\n"+
			"```",
			history.ScarcityFactor,
			history.DistributionFactor,
			history.HoardingFactor,
			history.ActivityFactor,
		)

		// Format distribution stats
		distribution := fmt.Sprintf("```md\n"+
			"# Card Distribution\n"+
			"* Total Copies: %d\n"+
			"* Active Copies: %d\n"+
			"* Unique Owners: %d\n"+
			"* Active Owners: %d\n"+
			"* Max Per User: %d\n"+
			"* Avg Per User: %.2f\n"+
			"```",
			history.TotalCopies,
			history.ActiveCopies,
			history.UniqueOwners,
			history.ActiveOwners,
			history.MaxCopiesPerUser,
			history.AvgCopiesPerUser,
		)

		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{{
				Title: fmt.Sprintf("%s %s", cardInfo.Stars, cardInfo.FormattedName),
				Description: fmt.Sprintf("```md\n"+
					"# Market Information\n"+
					"* Collection: %s\n"+
					"* Rarity: %s\n"+
					"* Current Price: %d\n"+
					"* 24h Change: %.2f%%\n"+
					"* Status: %s\n"+
					"```",
					cardInfo.FormattedCollection,
					getRarityName(card.Level),
					history.Price,
					history.PriceChangePercent,
					marketStatus,
				),
				Color: getColorByLevel(card.Level),
				Fields: []discord.EmbedField{
					{
						Name:   "📈 24h Price Range",
						Value:  fmt.Sprintf("```\nLow: %d\nHigh: %d\nAvg: %.0f\n```", stats.MinPrice24h, stats.MaxPrice24h, stats.AvgPrice24h),
						Inline: &inlineTrue,
					},
					{
						Name:   "📊 Price Factors",
						Value:  priceFactors,
						Inline: &inlineTrue,
					},
					{
						Name:   "📑 Distribution",
						Value:  distribution,
						Inline: &inlineFalse,
					},
					{
						Name:   "💡 Price Explanation",
						Value:  fmt.Sprintf("```%s```", history.PriceReason),
						Inline: &inlineFalse,
					},
				},
				Thumbnail: &discord.EmbedResource{
					URL: cardInfo.ImageURL,
				},
				Footer: &discord.EmbedFooter{
					Text:    fmt.Sprintf("Last updated %s • Requested by %s", timestamp, event.User().Username),
					IconURL: event.User().EffectiveAvatarURL(),
				},
			}},
			Components: []discord.ContainerComponent{
				discord.NewActionRow(
					discord.NewSecondaryButton("🔍 View Details", fmt.Sprintf("details:%d", cardID)),
					discord.NewPrimaryButton("💖 Add to Favorites", fmt.Sprintf("favorite:%d", cardID)),
					discord.NewSuccessButton("📈 Price History", fmt.Sprintf("pricehistory:%d", cardID)),
				),
			},
		})
	}
}
