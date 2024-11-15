package commands

import (
	"context"
	"fmt"
	"log"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/uptrace/bun"
)

var AnalyzeEconomy = discord.SlashCommandCreate{
	Name:        "analyze-economy",
	Description: "📊 Analyze the current economic distribution",
	// DefaultMemberPermissions: json.NewNullable(discord.PermissionManageGuild),
}

type EconomyStats struct {
	TotalUsers          int
	ActiveUsers         int32
	TotalWealth         int64
	TotalCardWealth     int64
	CombinedWealth      int64
	AverageBalance      float64
	AverageCardValue    float64
	MedianBalance       int64
	MedianCardValue     int64
	TopBalance          int64
	TopCardValue        int64
	BottomBalance       int64
	BottomCardValue     int64
	GiniCoefficient     float64
	CardGiniCoefficient float64
	WealthRanges        map[string]int
	CardWealthRanges    map[string]int
}

type batchResult struct {
	wealth      int64
	cardWealth  int64
	activeUsers int32
	ranges      map[string]int
	cardRanges  map[string]int
	balances    []int64
	cardValues  []int64
}

func AnalyzeEconomyHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(event *handler.CommandEvent) error {
		// Defer the response to avoid timeout
		if err := event.DeferCreateMessage(false); err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		users, err := b.UserRepository.GetUsers(ctx)
		if err != nil {
			_, err := event.CreateFollowupMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "Error",
					Description: "Failed to fetch user data",
					Color:       utils.ErrorColor,
				}},
			})
			return err
		}

		stats := calculateEconomyStats(b, users)

		// Create wealth distribution ranges
		ranges := []struct {
			Name  string
			Min   int64
			Count int
		}{
			{"💰 1M+", 1000000, stats.WealthRanges["1M+"]},
			{"💰 500k-1M", 500000, stats.WealthRanges["500k-1M"]},
			{"💰 100k-500k", 100000, stats.WealthRanges["100k-500k"]},
			{"💰 50k-100k", 50000, stats.WealthRanges["50k-100k"]},
			{"💰 10k-50k", 10000, stats.WealthRanges["10k-50k"]},
			{"💰 1k-10k", 1000, stats.WealthRanges["1k-10k"]},
			{"💰 0-1k", 0, stats.WealthRanges["0-1k"]},
		}

		// Build distribution graph
		var distribution strings.Builder
		distribution.WriteString("```\nWealth Distribution:\n\n")

		for _, r := range ranges {
			percentage := float64(r.Count) / float64(stats.TotalUsers) * 100
			bars := int(percentage / 2) // Each bar represents 2%
			distribution.WriteString(fmt.Sprintf("%-12s %s %.1f%% (%d users)\n",
				r.Name,
				strings.Repeat("█", bars),
				percentage,
				r.Count))
		}
		distribution.WriteString("```")

		// Create the response embed
		_, err = event.CreateFollowupMessage(discord.MessageCreate{
			Embeds: []discord.Embed{{
				Title: "📊 Economy Analysis Report",
				Description: fmt.Sprintf("```md\n"+
					"# General Statistics\n"+
					"* Total Users: %d\n"+
					"* Active Users: %d (%.1f%%)\n"+
					"\n"+
					"# Currency Wealth\n"+
					"* Total Balance: %s \n"+
					"* Average Balance: %s 💰\n"+
					"* Median Balance: %s 💰\n"+
					"* Gini Coefficient: %.3f\n"+
					"\n"+
					"# Card Wealth\n"+
					"* Total Card Value: %s 💰\n"+
					"* Average Card Value: %s 💰\n"+
					"* Median Card Value: %s 💰\n"+
					"* Card Gini Coefficient: %.3f\n"+
					"\n"+
					"# Combined Wealth\n"+
					"* Total Combined: %s 💰\n"+
					"```\n%s",
					stats.TotalUsers,
					stats.ActiveUsers,
					float64(stats.ActiveUsers)/float64(stats.TotalUsers)*100,
					utils.FormatNumber(stats.TotalWealth),
					utils.FormatNumber(int64(stats.AverageBalance)),
					utils.FormatNumber(stats.MedianBalance),
					stats.GiniCoefficient,
					utils.FormatNumber(stats.TotalCardWealth),
					utils.FormatNumber(int64(stats.AverageCardValue)),
					utils.FormatNumber(stats.MedianCardValue),
					stats.CardGiniCoefficient,
					utils.FormatNumber(stats.CombinedWealth),
					distribution.String(),
				),
				Color: 0x3498db,
				Footer: &discord.EmbedFooter{
					Text: fmt.Sprintf("Generated at %s",
						time.Now().Format("2006-01-02 15:04:05")),
				},
			}},
		})
		return err
	}
}

func calculateEconomyStats(b *bottemplate.Bot, users []*models.User) EconomyStats {
	stats := EconomyStats{
		TotalUsers:       len(users),
		WealthRanges:     make(map[string]int),
		CardWealthRanges: make(map[string]int),
	}

	if len(users) == 0 {
		return stats
	}

	// Initialize PriceCalculator with proper config
	config := economy.PricingConfig{
		BasePrice:           economy.InitialBasePrice,
		LevelMultiplier:     economy.LevelMultiplier,
		InactivityThreshold: 7 * 24 * time.Hour,
		BaseMultiplier:      1000,
		OwnershipImpact:     0.01,
		RarityMultiplier:    0.2,
		MinPrice:            economy.MinPrice,
		MaxPrice:            economy.MaxPrice,
		MinActiveOwners:     economy.MinimumActiveOwners,
		MinTotalCopies:      economy.MinimumTotalCopies,
		ScarcityWeight:      0.3,
		ActivityWeight:      0.2,
		ScarcityImpact:      0.01,
		DistributionImpact:  0.05,
		HoardingThreshold:   10,
		HoardingImpact:      0.1,
		ActivityImpact:      0.02,
		PriceUpdateInterval: 1 * time.Hour,
		CacheExpiration:     15 * time.Minute,
	}
	priceCalc := economy.NewPriceCalculator(b.DB, config)

	// Count total cards for debugging
	totalCards := 0
	for _, user := range users {
		totalCards += len(user.Cards)
	}
	log.Printf("[GoHYE] [%s] [DEBUG] [MARKET] Processing total cards: %d",
		time.Now().Format("15:04:05"),
		totalCards)

	// Use worker pool for better performance
	numWorkers := runtime.NumCPU()
	workChan := make(chan []*models.User, numWorkers)
	resultChan := make(chan batchResult)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range workChan {
				result := processBatch(ctx, batch, priceCalc)
				resultChan <- result
			}
		}()
	}

	// Send work batches
	go func() {
		batchSize := 100
		for i := 0; i < len(users); i += batchSize {
			end := i + batchSize
			if end > len(users) {
				end = len(users)
			}
			workChan <- users[i:end]
		}
		close(workChan)
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var allBalances []int64
	var allCardValues []int64

	for result := range resultChan {
		// Only count positive balances
		if result.wealth > 0 {
			stats.TotalWealth += result.wealth
		}
		if result.cardWealth > 0 {
			stats.TotalCardWealth += result.cardWealth
		}
		stats.ActiveUsers += result.activeUsers

		// Filter out negative values
		for _, bal := range result.balances {
			if bal > 0 {
				allBalances = append(allBalances, bal)
			}
		}
		for _, val := range result.cardValues {
			if val > 0 {
				allCardValues = append(allCardValues, val)
			}
		}

		for k, v := range result.ranges {
			stats.WealthRanges[k] += v
		}
		for k, v := range result.cardRanges {
			stats.CardWealthRanges[k] += v
		}
	}

	// Calculate final stats with only positive values
	stats.CombinedWealth = stats.TotalWealth + stats.TotalCardWealth
	calculateFinalStats(&stats, allBalances, allCardValues)

	return stats
}

func calculateGiniCoefficient(balances []int64) float64 {
	n := float64(len(balances))
	if n <= 1 {
		return 0
	}

	// Calculate Gini coefficient
	var sumOfDifferences float64
	var sumOfBalances float64

	for i := 0; i < len(balances); i++ {
		sumOfBalances += float64(balances[i])
		for j := 0; j < len(balances); j++ {
			sumOfDifferences += math.Abs(float64(balances[i] - balances[j]))
		}
	}

	return sumOfDifferences / (2 * n * sumOfBalances)
}

func calculateFinalStats(stats *EconomyStats, allBalances []int64, allCardValues []int64) {
	// Handle balance statistics
	if len(allBalances) > 0 {
		sort.Slice(allBalances, func(i, j int) bool {
			return allBalances[i] < allBalances[j]
		})
		stats.BottomBalance = allBalances[0]
		stats.TopBalance = allBalances[len(allBalances)-1]
		stats.MedianBalance = allBalances[len(allBalances)/2]
		stats.GiniCoefficient = calculateGiniCoefficient(allBalances)
	}

	// Calculate average balance using total users to include inactive ones
	if stats.TotalUsers > 0 {
		stats.AverageBalance = float64(stats.TotalWealth) / float64(stats.TotalUsers)
	}

	// Handle card value statistics
	if len(allCardValues) > 0 {
		sort.Slice(allCardValues, func(i, j int) bool {
			return allCardValues[i] < allCardValues[j]
		})
		stats.BottomCardValue = allCardValues[0]
		stats.TopCardValue = allCardValues[len(allCardValues)-1]
		stats.MedianCardValue = allCardValues[len(allCardValues)/2]
		stats.CardGiniCoefficient = calculateGiniCoefficient(allCardValues)
	}

	// Calculate average card value using total users to include those without cards
	if stats.TotalUsers > 0 {
		stats.AverageCardValue = float64(stats.TotalCardWealth) / float64(stats.TotalUsers)
	}
}

func categorizeCurrencyWealth(balance int64, ranges map[string]int) {
	switch {
	case balance >= 1000000:
		ranges["1M+"]++
	case balance >= 500000:
		ranges["500k-1M"]++
	case balance >= 100000:
		ranges["100k-500k"]++
	case balance >= 50000:
		ranges["50k-100k"]++
	case balance >= 10000:
		ranges["10k-50k"]++
	case balance >= 1000:
		ranges["1k-10k"]++
	default:
		ranges["0-1k"]++
	}
}

func categorizeCardWealth(value int64, ranges map[string]int) {
	switch {
	case value >= 1000000:
		ranges["1M+"]++
	case value >= 500000:
		ranges["500k-1M"]++
	case value >= 100000:
		ranges["100k-500k"]++
	case value >= 50000:
		ranges["50k-100k"]++
	case value >= 10000:
		ranges["10k-50k"]++
	case value >= 1000:
		ranges["1k-10k"]++
	default:
		ranges["0-1k"]++
	}
}

func processBatch(ctx context.Context, batch []*models.User, priceCalc *economy.PriceCalculator) batchResult {
	result := batchResult{
		ranges:     make(map[string]int),
		cardRanges: make(map[string]int),
		balances:   make([]int64, 0, len(batch)),
		cardValues: make([]int64, 0, len(batch)),
	}

	activeThreshold := time.Now().AddDate(0, 0, -7)

	// Process currency wealth and active users first (no DB calls)
	for _, user := range batch {
		if user.Balance > 0 {
			result.balances = append(result.balances, user.Balance)
			result.wealth += user.Balance
			categorizeCurrencyWealth(user.Balance, result.ranges)
		}

		if user.LastDaily.After(activeThreshold) {
			atomic.AddInt32(&result.activeUsers, 1)
		}
	}

	// Batch fetch all user cards at once
	userIDs := make([]string, len(batch))
	for i, user := range batch {
		userIDs[i] = user.DiscordID
	}

	var userCards []models.UserCard
	err := priceCalc.GetDB().BunDB().NewSelect().
		Model(&userCards).
		Where("user_id IN (?)", bun.In(userIDs)).
		Scan(ctx)

	if err != nil {
		log.Printf("[GoHYE] [ERROR] Failed to fetch user cards in batch: %v", err)
		return result
	}

	// Group cards by user
	userCardMap := make(map[string][]models.UserCard)
	cardIDs := make(map[int64]struct{})
	for _, card := range userCards {
		userCardMap[card.UserID] = append(userCardMap[card.UserID], card)
		cardIDs[card.CardID] = struct{}{}
	}

	// Batch fetch all card prices at once
	uniqueCardIDs := make([]int64, 0, len(cardIDs))
	for cardID := range cardIDs {
		uniqueCardIDs = append(uniqueCardIDs, cardID)
	}

	prices, err := priceCalc.BatchCalculateCardPrices(ctx, uniqueCardIDs)
	if err != nil {
		log.Printf("[GoHYE] [ERROR] Failed to calculate prices in batch: %v", err)
		return result
	}

	// Calculate card wealth for each user
	var mu sync.Mutex
	workers := runtime.GOMAXPROCS(0)
	userBatchSize := (len(batch) + workers - 1) / workers

	var wg sync.WaitGroup
	for i := 0; i < len(batch); i += userBatchSize {
		end := i + userBatchSize
		if end > len(batch) {
			end = len(batch)
		}

		wg.Add(1)
		go func(users []*models.User) {
			defer wg.Done()

			for _, user := range users {
				userCards := userCardMap[user.DiscordID]
				if len(userCards) == 0 {
					continue
				}

				var totalValue int64
				for _, card := range userCards {
					if price, ok := prices[card.CardID]; ok {
						cardValue := price * card.Amount
						totalValue += cardValue
					}
				}

				if totalValue > 0 {
					mu.Lock()
					result.cardValues = append(result.cardValues, totalValue)
					result.cardWealth += totalValue
					categorizeCardWealth(totalValue, result.cardRanges)
					mu.Unlock()
				}
			}
		}(batch[i:end])
	}

	wg.Wait()
	return result
}
