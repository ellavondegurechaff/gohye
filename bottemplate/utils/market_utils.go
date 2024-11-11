package utils

import (
	"fmt"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
)

// MarketDisplayInfo contains formatted market information
type MarketDisplayInfo struct {
	PriceFactors string
	Distribution string
	MarketStatus string
	PriceRange   string
}

// FormatMarketInfo formats all market-related information
func FormatMarketInfo(history models.CardMarketHistory, stats MarketStats) MarketDisplayInfo {
	return MarketDisplayInfo{
		PriceFactors: formatPriceFactors(history),
		Distribution: formatDistribution(history),
		MarketStatus: getMarketStatus(history.IsActive),
		PriceRange:   formatPriceRange(stats),
	}
}

func formatPriceFactors(history models.CardMarketHistory) string {
	return fmt.Sprintf("```md\n"+
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
}

func formatDistribution(history models.CardMarketHistory) string {
	return fmt.Sprintf("```md\n"+
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
}

func getMarketStatus(isActive bool) string {
	if isActive {
		return ActiveMarketStatus
	}
	return InactiveMarketStatus
}

func formatPriceRange(stats MarketStats) string {
	return fmt.Sprintf("```\nLow: %d\nHigh: %d\nAvg: %.0f\n```",
		stats.MinPrice24h,
		stats.MaxPrice24h,
		stats.AvgPrice24h,
	)
}

// MarketStats represents market statistics
type MarketStats struct {
	MinPrice24h int64
	MaxPrice24h int64
	AvgPrice24h float64
}
