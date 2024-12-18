package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var AnalyzeEconomy = discord.SlashCommandCreate{
	Name:        "analyze-economy",
	Description: "📊 Analyze the current economic state",
}

func AnalyzeEconomyHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(event *handler.CommandEvent) error {
		start := time.Now()
		defer func() {
			slog.Info("Command completed",
				slog.String("type", "cmd"),
				slog.String("name", "analyze-economy"),
				slog.Duration("total_time", time.Since(start)),
			)
		}()

		if err := event.DeferCreateMessage(false); err != nil {
			return fmt.Errorf("failed to defer message: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Get latest stats
		stats, err := b.EconomyStatsRepository.GetLatest(ctx)
		if err != nil {
			slog.Info("No existing economic stats found, initializing...")

			// Create initial stats
			monitor := economy.NewEconomyMonitor(b.EconomyStatsRepository, b.PriceCalculator, b.UserRepository)
			stats, err = monitor.CollectStats(ctx)
			if err != nil {
				slog.Error("Failed to collect economic statistics",
					slog.String("error", err.Error()))
				_, err = event.UpdateInteractionResponse(discord.MessageUpdate{
					Embeds: &[]discord.Embed{{
						Title:       "Error",
						Description: "Failed to initialize economic statistics. Please try again later.",
						Color:       utils.ErrorColor,
					}},
				})
				return err
			}

			// Store initial stats
			if err := b.EconomyStatsRepository.Create(ctx, stats); err != nil {
				slog.Error("Failed to store economic statistics",
					slog.String("error", err.Error()))
				_, err = event.UpdateInteractionResponse(discord.MessageUpdate{
					Embeds: &[]discord.Embed{{
						Title:       "Error",
						Description: "Failed to store economic statistics. Please try again later.",
						Color:       utils.ErrorColor,
					}},
				})
				return err
			}
		}

		// Initialize trends with zeros if no historical data
		trends := map[string]float64{
			"wealth_change":        0.0,
			"activity_change":      0.0,
			"market_volume_change": 0.0,
			"inequality_change":    0.0,
		}

		// Try to get historical trends if available
		historicalTrends, err := b.EconomyStatsRepository.GetTrends(ctx)
		if err == nil {
			trends = historicalTrends
		}

		// Format health status
		var healthStatus string
		switch {
		case stats.EconomicHealth >= 90:
			healthStatus = "🟢 Excellent"
		case stats.EconomicHealth >= 75:
			healthStatus = "🟡 Good"
		case stats.EconomicHealth >= 60:
			healthStatus = "🟠 Fair"
		default:
			healthStatus = "🔴 Poor"
		}

		// Create wealth distribution graph
		distribution := createDistributionGraph(stats)

		// Format trends
		trendAnalysis := formatTrends(trends)

		// Create the response embed
		embed := discord.NewEmbedBuilder().
			SetTitle("📊 Economic Analysis Report").
			AddField("Economic Health", fmt.Sprintf("```\nScore: %.1f/100\nStatus: %s\nNeeds Correction: %v\n```",
				stats.EconomicHealth,
				healthStatus,
				stats.NeedsCorrection,
			), false).
			AddField("Wealth Statistics", fmt.Sprintf("```\nTotal Wealth: %s\nAverage Wealth: %s\nMedian Wealth: %s\nGini Coefficient: %.3f\n```",
				utils.FormatNumber(stats.TotalWealth),
				utils.FormatNumber(stats.AverageWealth),
				utils.FormatNumber(stats.MedianWealth),
				stats.GiniCoefficient,
			), false).
			AddField("User Activity", fmt.Sprintf("```\nTotal Users: %d\nActive Users: %d\nDaily Transactions: %d\nMarket Volume: %s\n```",
				stats.TotalUsers,
				stats.ActiveUsers,
				stats.DailyTransactions,
				utils.FormatNumber(stats.MarketVolume),
			), false).
			AddField("Wealth Distribution", distribution, false).
			AddField("30-Day Trends", trendAnalysis, false).
			SetColor(getHealthColor(stats.EconomicHealth)).
			SetTimestamp(time.Now()).
			SetFooter("Last updated", "")

		// Update the deferred response instead of creating a follow-up
		_, err = event.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds: &[]discord.Embed{embed.Build()},
		})
		return err
	}
}

func createDistributionGraph(stats *models.EconomyStats) string {
	var sb strings.Builder
	sb.WriteString("```\n")

	// Calculate percentages
	wealthyPct := float64(stats.WealthyPlayerCount) / float64(stats.TotalUsers) * 100
	poorPct := float64(stats.PoorPlayerCount) / float64(stats.TotalUsers) * 100
	middlePct := 100 - wealthyPct - poorPct

	// Create simplified bar graph using shorter bars and cleaner format
	const barWidth = 20 // Maximum bar width
	sb.WriteString(fmt.Sprintf("W │%s %.1f%%\n", strings.Repeat("■", int(wealthyPct*barWidth/100)), wealthyPct))
	sb.WriteString(fmt.Sprintf("M │%s %.1f%%\n", strings.Repeat("■", int(middlePct*barWidth/100)), middlePct))
	sb.WriteString(fmt.Sprintf("P │%s %.1f%%\n", strings.Repeat("■", int(poorPct*barWidth/100)), poorPct))
	sb.WriteString("```")

	return sb.String()
}

func formatTrends(trends map[string]float64) string {
	return fmt.Sprintf("```diff\n"+
		"Wealth: %s%.1f%%\n"+
		"Activity: %s%.1f%%\n"+
		"Market Volume: %s%.1f%%\n"+
		"Inequality: %s%.1f%%\n"+
		"```",
		getTrendSymbol(trends["wealth_change"]), trends["wealth_change"],
		getTrendSymbol(trends["activity_change"]), trends["activity_change"],
		getTrendSymbol(trends["market_volume_change"]), trends["market_volume_change"],
		getTrendSymbol(-trends["inequality_change"]), trends["inequality_change"],
	)
}

func getTrendSymbol(value float64) string {
	if value > 0 {
		return "+"
	}
	return ""
}

func getHealthColor(health float64) int {
	switch {
	case health >= 90:
		return 0x2ECC71 // Green
	case health >= 75:
		return 0xF1C40F // Yellow
	case health >= 60:
		return 0xE67E22 // Orange
	default:
		return 0xE74C3C // Red
	}
}
