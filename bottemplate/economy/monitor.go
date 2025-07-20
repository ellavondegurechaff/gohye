package economy

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
)

// Configuration variables
type EconomyConfig struct {
	DailyBonusBase    int64
	CatchupMultiplier float64
	WealthThreshold   int64
}

type EconomyMonitor struct {
	statsRepo repositories.EconomyStatsRepository
	priceCalc *PriceCalculator
	userRepo  repositories.UserRepository
	config    *EconomyConfig

	// Monitoring settings
	checkInterval   time.Duration
	correctionDelay time.Duration
	mutex           sync.RWMutex
}

func NewEconomyMonitor(
	statsRepo repositories.EconomyStatsRepository,
	priceCalc *PriceCalculator,
	userRepo repositories.UserRepository,
) *EconomyMonitor {
	return &EconomyMonitor{
		statsRepo:       statsRepo,
		priceCalc:       priceCalc,
		userRepo:        userRepo,
		checkInterval:   15 * time.Minute,
		correctionDelay: 6 * time.Hour,
		config: &EconomyConfig{
			DailyBonusBase:    1000,
			CatchupMultiplier: 1.5,
			WealthThreshold:   100000,
		},
	}
}

func (m *EconomyMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.runMonitoringCycle(ctx); err != nil {
					slog.Error("Failed to run monitoring cycle",
						slog.String("error", err.Error()))
				}
			}
		}
	}()
}

func (m *EconomyMonitor) runMonitoringCycle(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Collect current stats (which now sets NeedsCorrection)
	stats, err := m.collectCurrentStats(ctx)
	if err != nil {
		return err
	}

	// Store stats first
	if err := m.statsRepo.Create(ctx, stats); err != nil {
		return err
	}

	// Apply corrections if needed
	if stats.NeedsCorrection {
		slog.Info("Applying economic corrections",
			"gini_coefficient", stats.GiniCoefficient,
			"active_users", stats.ActiveUsers,
			"wealth_concentration", stats.WealthConcentration)

		if err := m.applyCorrections(ctx, stats); err != nil {
			return err
		}

		// Update LastCorrectionTime and reset NeedsCorrection
		stats.LastCorrectionTime = time.Now()
		stats.NeedsCorrection = false // Reset after applying corrections
		if err := m.statsRepo.Create(ctx, stats); err != nil {
			return err
		}
	}

	return nil
}

func (m *EconomyMonitor) collectCurrentStats(ctx context.Context) (*models.EconomyStats, error) {
	users, err := m.userRepo.GetUsers(ctx)
	if err != nil {
		return nil, err
	}

	stats := &models.EconomyStats{
		Timestamp: time.Now(),
	}

	// Calculate wealth statistics with validation
	var totalWealth int64
	var wealthyCount, poorCount int
	balances := make([]int64, 0, len(users))

	for _, user := range users {
		// Ensure balance is not negative
		balance := user.Balance
		if balance < 0 {
			balance = 0
		}

		balances = append(balances, balance)
		totalWealth += balance

		if balance > m.config.WealthThreshold {
			wealthyCount++
		} else if balance < m.config.WealthThreshold/10 {
			poorCount++
		}
	}

	stats.TotalWealth = totalWealth
	stats.TotalUsers = len(users)

	if len(users) > 0 {
		stats.AverageWealth = totalWealth / int64(len(users))
		stats.MedianWealth = calculateMedian(balances)
	}

	stats.WealthyPlayerCount = wealthyCount
	stats.PoorPlayerCount = poorCount
	stats.GiniCoefficient = calculateGiniCoefficient(balances)
	stats.WealthConcentration = float64(wealthyCount) / math.Max(1.0, float64(len(users)))

	// Count active users
	activeUsers := 0
	for _, user := range users {
		if time.Since(user.LastDaily) < 24*time.Hour {
			activeUsers++
		}
	}
	stats.ActiveUsers = activeUsers

	// After setting all the stats, explicitly check conditions
	stats.NeedsCorrection = false
	if stats.GiniCoefficient > 0.6 || // High inequality
		stats.ActiveUsers == 0 || // No active users
		stats.DailyTransactions == 0 || // No market activity
		stats.WealthConcentration > 0.15 || // High wealth concentration
		float64(stats.ActiveUsers)/float64(stats.TotalUsers) < 0.1 { // Low participation
		stats.NeedsCorrection = true
		slog.Info("Economy needs correction",
			"gini", stats.GiniCoefficient,
			"active_users", stats.ActiveUsers,
			"wealth_concentration", stats.WealthConcentration)
	}

	return stats, nil
}

func (m *EconomyMonitor) applyCorrections(ctx context.Context, stats *models.EconomyStats) error {
	// More aggressive corrections for severe economic issues
	if stats.GiniCoefficient > 0.7 {
		m.config.DailyBonusBase = int64(float64(m.config.DailyBonusBase) * 1.5)
		m.config.CatchupMultiplier = 3.0
	} else if stats.GiniCoefficient > 0.5 {
		m.config.DailyBonusBase = int64(float64(m.config.DailyBonusBase) * 1.2)
		m.config.CatchupMultiplier = 2.0
	}

	// Apply wealth tax for extreme concentration
	if stats.WealthConcentration > 0.15 || stats.GiniCoefficient > 0.7 {
		if err := m.applyWealthTax(ctx); err != nil {
			return err
		}
	}

	// Incentivize activity if there are no active users
	if stats.ActiveUsers == 0 {
		m.config.DailyBonusBase *= 2
	}

	return nil
}

func (m *EconomyMonitor) applyWealthTax(ctx context.Context) error {
	users, err := m.userRepo.GetUsers(ctx)
	if err != nil {
		return err
	}

	for _, user := range users {
		if user.Balance > m.config.WealthThreshold*10 {
			newBalance := int64(float64(user.Balance) * 0.99)
			user.Balance = newBalance
			if err := m.userRepo.Update(ctx, user); err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper function to calculate Gini coefficient using optimized O(n log n) algorithm
func calculateGiniCoefficient(balances []int64) float64 {
	if len(balances) == 0 {
		return 0
	}

	// Convert to float64 slice for calculations
	sortedBalances := make([]float64, len(balances))
	var totalSum float64

	for i, balance := range balances {
		sortedBalances[i] = float64(balance)
		totalSum += float64(balance)
	}

	if totalSum == 0 {
		return 0
	}

	// Sort balances for O(n log n) algorithm
	sort.Float64s(sortedBalances)

	n := float64(len(sortedBalances))
	var numerator float64

	// Optimized calculation: sum of (2*i + 1 - n) * y_i for sorted values
	for i, balance := range sortedBalances {
		numerator += (2*float64(i) + 1 - n) * balance
	}

	// Gini coefficient formula for sorted data
	return numerator / (n * totalSum)
}

// RunMonitoringCycle executes a single monitoring cycle and returns any error
func (m *EconomyMonitor) RunMonitoringCycle(ctx context.Context) error {
	slog.Info("Starting economy monitoring cycle")
	m.mutex.Lock()
	defer m.mutex.Unlock()

	stats, err := m.collectCurrentStats(ctx)
	if err != nil {
		slog.Error("Failed to collect current stats",
			"error", err)
		return err
	}

	// Store stats
	if err := m.statsRepo.Create(ctx, stats); err != nil {
		slog.Error("Failed to store economy stats",
			"error", err)
		return err
	}

	// Update economic health
	if err := m.statsRepo.UpdateEconomicHealth(ctx); err != nil {
		slog.Error("Failed to update economic health",
			"error", err)
		return err
	}

	slog.Info("Economy monitoring cycle completed successfully",
		"total_users", stats.TotalUsers,
		"active_users", stats.ActiveUsers,
		"economic_health", stats.EconomicHealth)
	return nil
}

// CollectStats gathers current economic statistics
func (m *EconomyMonitor) CollectStats(ctx context.Context) (*models.EconomyStats, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.collectCurrentStats(ctx)
}

func calculateMedian(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}

	// Create a copy to avoid modifying original slice
	sorted := make([]int64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}
