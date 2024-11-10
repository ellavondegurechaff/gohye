package models

import (
	"time"

	"github.com/uptrace/bun"
)

type CardMarketHistory struct {
	bun.BaseModel `bun:"table:card_market_history,alias:cmh"`

	ID                 int64     `bun:"id,pk,autoincrement"`
	CardID             int64     `bun:"card_id,notnull"`
	Price              int64     `bun:"price,notnull"`
	IsActive           bool      `bun:"is_active,notnull,default:false"`
	ActiveOwners       int       `bun:"active_owners,notnull"`
	TotalCopies        int       `bun:"total_copies,notnull"`
	ActiveCopies       int       `bun:"active_copies,notnull"`
	UniqueOwners       int       `bun:"unique_owners,notnull"`
	MaxCopiesPerUser   int       `bun:"max_copies_per_user,notnull"`
	AvgCopiesPerUser   float64   `bun:"avg_copies_per_user,notnull"`
	ScarcityFactor     float64   `bun:"scarcity_factor,notnull"`
	DistributionFactor float64   `bun:"distribution_factor,notnull"`
	HoardingFactor     float64   `bun:"hoarding_factor,notnull"`
	ActivityFactor     float64   `bun:"activity_factor,notnull"`
	PriceReason        string    `bun:"price_reason,notnull"`
	Timestamp          time.Time `bun:"timestamp,notnull"`
	PriceChangePercent float64   `bun:"price_change_percentage"`
}
