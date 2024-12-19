package repositories

import (
	"context"
	"math"
	"time"

	"github.com/disgoorg/bot-template/internal/gateways/database/models"
	"github.com/uptrace/bun"
)

type EconomyStatsRepository interface {
	Create(ctx context.Context, stats *models.EconomyStats) error
	GetLatest(ctx context.Context) (*models.EconomyStats, error)
	GetHistorical(ctx context.Context, start, end time.Time) ([]*models.EconomyStats, error)
	UpdateEconomicHealth(ctx context.Context) error
	GetTrends(ctx context.Context) (map[string]float64, error)
}

type economyStatsRepository struct {
	db *bun.DB
}

func NewEconomyStatsRepository(db *bun.DB) EconomyStatsRepository {
	return &economyStatsRepository{db: db}
}

func (r *economyStatsRepository) Create(ctx context.Context, stats *models.EconomyStats) error {
	stats.Timestamp = time.Now()
	_, err := r.db.NewInsert().Model(stats).Exec(ctx)
	return err
}

func (r *economyStatsRepository) GetLatest(ctx context.Context) (*models.EconomyStats, error) {
	stats := new(models.EconomyStats)
	err := r.db.NewSelect().
		Model(stats).
		Order("timestamp DESC").
		Limit(1).
		Scan(ctx)
	return stats, err
}

func (r *economyStatsRepository) GetHistorical(ctx context.Context, start, end time.Time) ([]*models.EconomyStats, error) {
	var stats []*models.EconomyStats
	err := r.db.NewSelect().
		Model(&stats).
		Where("timestamp BETWEEN ? AND ?", start, end).
		Order("timestamp ASC").
		Scan(ctx)
	return stats, err
}

func (r *economyStatsRepository) UpdateEconomicHealth(ctx context.Context) error {
	// Get latest stats
	latest, err := r.GetLatest(ctx)
	if err != nil {
		return err
	}

	// Calculate economic health score (0-100)
	healthScore := calculateHealthScore(latest)

	// Update both economic health and needs_correction in the database
	_, err = r.db.NewUpdate().
		Model(latest).
		Set("economic_health = ?", healthScore).
		Set("needs_correction = ?", latest.NeedsCorrection).
		Where("id = ?", latest.ID).
		Exec(ctx)

	return err
}

func (r *economyStatsRepository) GetTrends(ctx context.Context) (map[string]float64, error) {
	start := time.Now().Add(-30 * 24 * time.Hour) // Default to 30 days
	stats, err := r.GetHistorical(ctx, start, time.Now())
	if err != nil {
		return nil, err
	}

	trends := calculateTrends(stats)
	return trends, nil
}

func calculateHealthScore(stats *models.EconomyStats) float64 {
	weights := map[string]float64{
		"wealth_distribution": 0.30,
		"market_activity":     0.25,
		"user_participation":  0.25,
		"price_stability":     0.20,
	}

	// Wealth distribution score (based on Gini coefficient and wealth concentration)
	wealthDistScore := (1-stats.GiniCoefficient)*70 +
		(1-stats.WealthConcentration)*30

	// Market activity score
	avgTransactionsPerUser := float64(stats.DailyTransactions) / math.Max(1.0, float64(stats.ActiveUsers))
	marketActivityScore := math.Min(100, (avgTransactionsPerUser/5.0)*100)

	// User participation score
	participationRate := float64(stats.ActiveUsers) / math.Max(1.0, float64(stats.TotalUsers))
	participationScore := participationRate * 100

	// Price stability score (inverse of volatility)
	stabilityScore := (1 - math.Min(1.0, stats.PriceVolatility)) * 100

	// Calculate final weighted score
	healthScore := wealthDistScore*weights["wealth_distribution"] +
		marketActivityScore*weights["market_activity"] +
		participationScore*weights["user_participation"] +
		stabilityScore*weights["price_stability"]

	// Set needs correction based on multiple factors
	stats.NeedsCorrection = false
	if healthScore < 60 || // Poor economic health
		stats.GiniCoefficient > 0.6 || // High inequality
		stats.ActiveUsers == 0 || // No active users
		stats.DailyTransactions == 0 || // No market activity
		participationRate < 0.1 || // Less than 10% user participation
		stats.WealthConcentration > 0.15 || // Wealth too concentrated
		stats.PriceVolatility > 0.5 { // High price volatility
		stats.NeedsCorrection = true
	}

	return math.Max(0, math.Min(100, healthScore))
}

func calculateTrends(stats []*models.EconomyStats) map[string]float64 {
	if len(stats) < 2 {
		return nil
	}

	first := stats[0]
	last := stats[len(stats)-1]

	trends := make(map[string]float64)

	// Calculate percentage changes
	trends["wealth_change"] = calculatePercentageChange(float64(first.AverageWealth), float64(last.AverageWealth))
	trends["activity_change"] = calculatePercentageChange(float64(first.DailyTransactions), float64(last.DailyTransactions))
	trends["inequality_change"] = calculatePercentageChange(first.GiniCoefficient, last.GiniCoefficient)
	trends["market_volume_change"] = calculatePercentageChange(float64(first.MarketVolume), float64(last.MarketVolume))

	return trends
}

func calculatePercentageChange(old, new float64) float64 {
	if old == 0 {
		return 0
	}
	return ((new - old) / old) * 100
}
