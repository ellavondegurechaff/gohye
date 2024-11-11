package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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
			Required:    false,
		},
		discord.ApplicationCommandOptionString{
			Name:        "card_name",
			Description: "The name of the card to check",
			Required:    false,
		},
	},
}

func PriceStatsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(event *handler.CommandEvent) error {
		if cardID := event.SlashCommandInteractionData().Int("card_id"); cardID != 0 {
			return handleCardByID(b, event, int64(cardID))
		}

		if cardName := event.SlashCommandInteractionData().String("card_name"); cardName != "" {
			return handleCardByName(b, event, cardName)
		}

		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{{
				Title:       "❌ Invalid Input",
				Description: "Please provide either a card ID or card name",
				Color:       0xFF0000,
			}},
		})
	}
}

func handleCardByID(b *bottemplate.Bot, event *handler.CommandEvent, cardID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), utils.MarketQueryTimeout)
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
		return utils.EH.CreateErrorEmbed(event, "Failed to fetch market data")
	}

	// Get the card details and stats
	card, err := b.CardRepository.GetByID(ctx, cardID)
	if err != nil {
		return utils.EH.CreateError(event, "Card Not Found",
			fmt.Sprintf("Card #%d does not exist", cardID))
	}

	stats, err := b.PriceCalculator.GetMarketStats(ctx, cardID, history.Price)
	if err != nil {
		return utils.EH.CreateErrorEmbed(event, "Failed to fetch market statistics")
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

	marketInfo := utils.FormatMarketInfo(history, utils.MarketStats{
		MinPrice24h: stats.MinPrice24h,
		MaxPrice24h: stats.MaxPrice24h,
		AvgPrice24h: stats.AvgPrice24h,
	})

	timestamp := fmt.Sprintf("<t:%d:R>", time.Now().Unix())
	inlineTrue := true

	// Format market status
	marketStatus := utils.ActiveMarketStatus
	if !history.IsActive {
		marketStatus = utils.InactiveMarketStatus
	}

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title: fmt.Sprintf("%s %s", cardInfo.Stars, cardInfo.FormattedName),
			Description: fmt.Sprintf("```md\n"+
				"# Market Information\n"+
				"* Collection: %s\n"+
				"* Rarity: %s\n"+
				"* Current Price: %d 💰\n"+
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
					Value:  marketInfo.PriceRange,
					Inline: &inlineTrue,
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
				discord.NewSecondaryButton("🔍 View Details", fmt.Sprintf("/details/%d", cardID)),
			),
		},
	})
}

// Component handler for the details button
func PriceDetailsHandler(b *bottemplate.Bot) handler.ComponentHandler {
	return func(event *handler.ComponentEvent) error {
		// Extract card ID from custom ID
		data := event.Data.(discord.ButtonInteractionData)
		parts := strings.Split(strings.TrimPrefix(data.CustomID(), "/details/"), "/")
		if len(parts) != 1 {
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: "Invalid button data",
					Color:       0xFF0000,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}

		// Convert string ID to int64
		cardID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: "Invalid card ID format",
					Color:       0xFF0000,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Get the latest market history entry
		var history models.CardMarketHistory
		err = b.DB.BunDB().NewSelect().
			Model(&history).
			Where("card_id = ?", cardID).
			Order("timestamp DESC").
			Limit(1).
			Scan(ctx)

		if err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: "Failed to fetch market data",
					Color:       0xFF0000,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}

		card, err := b.CardRepository.GetByID(ctx, cardID)
		if err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: fmt.Sprintf("Card #%d does not exist", cardID),
					Color:       0xFF0000,
				}},
				Flags: discord.MessageFlagEphemeral,
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

		inlineFalse := false

		// Format price factors
		priceFactors := fmt.Sprintf("```md\n"+
			"# Price Factors\n"+
			"* Current Price: %d 💰\n"+
			"* Scarcity: %.2fx\n"+
			"* Distribution: %.2fx\n"+
			"* Hoarding: %.2fx\n"+
			"* Activity: %.2fx\n"+
			"```",
			history.Price,
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
				Title: fmt.Sprintf("%s %s - Detailed Information", cardInfo.Stars, cardInfo.FormattedName),
				Color: getColorByLevel(card.Level),
				Fields: []discord.EmbedField{
					{
						Name:   "📊 Price Factors",
						Value:  priceFactors,
						Inline: &inlineFalse,
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
			}},
			Flags: discord.MessageFlagEphemeral,
		})
	}
}

func handleCardByName(b *bottemplate.Bot, event *handler.CommandEvent, cardName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Normalize search term
	searchTerm := strings.TrimSpace(cardName)
	searchTerm = strings.ToLower(searchTerm)
	searchTerm = strings.ReplaceAll(searchTerm, " ", "_")

	// Get all cards matching the name
	cards, err := b.CardRepository.GetByName(ctx, searchTerm)
	if err != nil {
		return utils.EH.CreateError(event, "Failed to search for cards",
			"An error occurred while searching for cards")
	}

	// Find best matching card using weighted search
	sortedCards := utils.WeightedSearch(cards, cardName, utils.SearchModeExact)
	if len(sortedCards) == 0 {
		return utils.EH.CreateError(event, "Card Not Found",
			fmt.Sprintf("No card found matching '%s'", cardName))
	}

	// Use the best match
	return handleCardByID(b, event, sortedCards[0].ID)
}
