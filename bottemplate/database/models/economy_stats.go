package models

import (
	"time"

	"github.com/uptrace/bun"
)

type EconomyStats struct {
	bun.BaseModel `bun:"table:economy_stats,alias:es"`

	ID                int64     `bun:"id,pk,autoincrement"`
	Timestamp         time.Time `bun:"timestamp,notnull"`
	MedianWealth      int64     `bun:"median_wealth,notnull"`
	AverageWealth     int64     `bun:"average_wealth,notnull"`
	TotalWealth       int64     `bun:"total_wealth,notnull"`
	ActiveUsers       int       `bun:"active_users,notnull"`
	TotalUsers        int       `bun:"total_users,notnull"`
	NewUsers          int       `bun:"new_users,notnull"`
	DailyTransactions int       `bun:"daily_transactions,notnull"`
	InflationRate     float64   `bun:"inflation_rate,notnull"`
	GiniCoefficient   float64   `bun:"gini_coefficient,notnull"`

	// Market health indicators
	AverageDailyTrades int     `bun:"average_daily_trades,notnull"`
	MarketVolume       int64   `bun:"market_volume,notnull"`
	PriceVolatility    float64 `bun:"price_volatility,notnull"`

	// Balance distribution
	WealthyPlayerCount  int     `bun:"wealthy_player_count,notnull"`
	PoorPlayerCount     int     `bun:"poor_player_count,notnull"`
	WealthConcentration float64 `bun:"wealth_concentration,notnull"`

	// Economic health
	EconomicHealth     float64   `bun:"economic_health,notnull"`
	NeedsCorrection    bool      `bun:"needs_correction,notnull"`
	LastCorrectionTime time.Time `bun:"last_correction_time,notnull"`
}
