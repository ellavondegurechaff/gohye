package economy

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	economicUtils "github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var PriceStats = discord.SlashCommandCreate{
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

		return utils.EH.CreateErrorEmbed(event, "Please provide either a card ID or card name")
	}
}

func handleCardByID(b *bottemplate.Bot, event *handler.CommandEvent, cardID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
	defer cancel()

	// Get the card details first
	card, err := b.CardRepository.GetByID(ctx, cardID)
	if err != nil {
		return utils.EH.CreateError(event, "Card Not Found",
			fmt.Sprintf("Card #%d does not exist", cardID))
	}

	// Get current price and stats
	price, err := b.PriceCalculator.GetLatestPrice(ctx, cardID)
	if err != nil {
		return utils.EH.CreateErrorEmbed(event, "Failed to fetch current price")
	}

	// Get market stats
	marketStats, err := b.PriceCalculator.GetMarketStats(ctx, cardID, price)
	if err != nil {
		slog.Error("Failed to fetch market statistics", 
			slog.String("type", "cmd"),
			slog.String("name", "price-stats"),
			slog.Int64("card_id", cardID),
			slog.String("error", err.Error()))
		return utils.EH.CreateErrorEmbed(event, fmt.Sprintf("Failed to fetch market statistics: %s", err.Error()))
	}

	// Get card stats for market status
	cardStatsMap, err := b.PriceCalculator.GetCardStats(ctx, []int64{cardID})
	if err != nil {
		return utils.EH.CreateErrorEmbed(event, "Failed to fetch card statistics")
	}

	cardStats, ok := cardStatsMap[cardID]
	if !ok {
		return utils.EH.CreateErrorEmbed(event, "Failed to fetch card data")
	}

	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		utils.GetGroupType(card.Tags),
		b.SpacesService.GetSpacesConfig(),
	)

	// Get market status
	marketStatus := utils.ActiveMarketStatus
	if cardStats.ActiveOwners < economy.MinimumActiveOwners {
		marketStatus = utils.InactiveMarketStatus
	}

	// Generate star display based on card level
	stars := strings.Repeat("⭐", card.Level)

	timestamp := fmt.Sprintf("<t:%d:R>", time.Now().Unix())

	description := fmt.Sprintf("```md\n"+
		"# Market Information\n"+
		"* Collection: %s\n"+
		"* Level: %s\n"+
		"* Current Price: %d 💰\n"+
		"* 24h Change: %.2f%%\n"+
		"* Status: %s\n"+
		"* Total Owners: %d\n"+
		"* Active Owners: %d\n"+
		"\n"+
		"# 24h Price Range\n"+
		"* Minimum: %d 💰\n"+
		"* Maximum: %d 💰\n"+
		"* Average: %.0f 💰\n"+
		"\n"+
		"# Vial Information\n"+
		"* Current Vial Value: %d 🧪\n"+
		"* Vial Rate: %.0f%%\n"+
		"```",
		cardInfo.FormattedCollection,
		stars,
		price,
		marketStats.PriceChangePercent,
		marketStatus,
		cardStats.UniqueOwners,
		cardStats.ActiveOwners,
		marketStats.MinPrice24h,
		marketStats.MaxPrice24h,
		marketStats.AvgPrice24h,
		calculateVialValue(price, card.Level),
		getVialRate(card.Level)*100,
	)

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title:       fmt.Sprintf("%s %s", cardInfo.Stars, cardInfo.FormattedName),
			Description: description,
			Color:       utils.GetColorByLevel(card.Level),
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
			return utils.EH.CreateEphemeralError(event, "Invalid button data")
		}

		cardID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: "Invalid card ID",
					Color:       config.ErrorColor,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}

		ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
		defer cancel()

		// Get the card details first
		card, err := b.CardRepository.GetByID(ctx, cardID)
		if err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: fmt.Sprintf("Card #%d does not exist", cardID),
					Color:       config.ErrorColor,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}

		// Get current price
		price, err := b.PriceCalculator.GetLatestPrice(ctx, cardID)
		if err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: "Failed to fetch current price",
					Color:       config.ErrorColor,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}

		// Get card stats
		stats, err := b.PriceCalculator.GetCardStats(ctx, []int64{cardID})
		if err != nil {
			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "❌ Error",
					Description: "Failed to fetch card statistics",
					Color:       config.ErrorColor,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}

		cardStats := stats[cardID]
		factors := b.PriceCalculator.CalculatePriceFactors(cardStats)

		// Check if card is inactive (no owners/activity)
		isInactive := cardStats.UniqueOwners == 0 || cardStats.ActiveOwners == 0

		// Format price factors with N/A handling
		priceFactors := fmt.Sprintf("```ansi\n"+
			"# Price Factors\n"+
			"* Current Price: %d 💰\n"+
			"* Vial Value: %d 🧪 (%.0f%%)\n"+
			"* Scarcity: %s\n"+
			"* Distribution: %s\n"+
			"* Hoarding: %s\n"+
			"* Activity: %s\n"+
			"```",
			price,
			calculateVialValue(price, card.Level),
			getVialRate(card.Level)*100,
			formatFactor(factors.ScarcityFactor, isInactive),
			formatFactor(factors.DistributionFactor, isInactive),
			formatFactor(factors.HoardingFactor, isInactive),
			formatFactor(factors.ActivityFactor, isInactive),
		)

		// Format distribution stats
		distribution := fmt.Sprintf("```ansi\n"+
			"# Card Distribution\n"+
			"* Unique Owners: %d\n"+
			"* Active Owners: %d\n"+
			"* Max Per User: %d\n"+
			"* Avg Per User: %.2f\n"+
			"```",
			cardStats.UniqueOwners,
			cardStats.ActiveOwners,
			cardStats.MaxCopiesPerUser,
			cardStats.AvgCopiesPerUser,
		)

		// Format price explanation for inactive cards
		explanation := factors.Reason
		if isInactive {
			explanation = "Card is currently inactive due to insufficient owners or activity"
		}

		cardInfo := utils.GetCardDisplayInfo(
			card.Name,
			card.ColID,
			card.Level,
			utils.GetGroupType(card.Tags),
			b.SpacesService.GetSpacesConfig(),
		)

		inlineFalse := false

		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{{
				Title: fmt.Sprintf("%s %s - Detailed Information", cardInfo.Stars, cardInfo.FormattedName),
				Color: utils.GetColorByLevel(card.Level),
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
						Value:  fmt.Sprintf("```%s```", explanation),
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

// Helper function to format factor values
func formatFactor(factor float64, isInactive bool) string {
	if isInactive {
		return "N/A"
	}
	return fmt.Sprintf("%.2fx", factor)
}

func handleCardByName(b *bottemplate.Bot, event *handler.CommandEvent, cardName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
	defer cancel()

	// Get all cards for unified search (more comprehensive than GetByName)
	allCards, err := b.CardRepository.GetAll(ctx)
	if err != nil {
		return utils.EH.CreateError(event, "Failed to search for cards",
			"An error occurred while searching for cards")
	}

	// Use UnifiedSearchService for improved search accuracy
	cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)
	unifiedSearchService := services.NewUnifiedSearchService(cardOperationsService)
	
	card, err := unifiedSearchService.SearchSingleCard(ctx, allCards, cardName)
	if err != nil {
		return utils.EH.CreateError(event, "Search failed", err.Error())
	}
	
	if card == nil {
		return utils.EH.CreateError(event, "Card Not Found",
			fmt.Sprintf("No card found matching '%s'", cardName))
	}

	// Use the best match
	return handleCardByID(b, event, card.ID)
}

// Helper function to calculate vial value
func calculateVialValue(price int64, level int) int64 {
	rate := getVialRate(level)
	return int64(float64(price) * rate)
}

// Helper function to get vial rate based on card level
func getVialRate(level int) float64 {
	switch level {
	case 1:
		return economicUtils.VialRate1Star
	case 2:
		return economicUtils.VialRate2Star
	case 3:
		return economicUtils.VialRate3Star
	default:
		return 0
	}
}
