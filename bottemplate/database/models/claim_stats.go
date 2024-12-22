package models

import (
	"time"

	"github.com/uptrace/bun"
)

type ClaimStats struct {
	bun.BaseModel `bun:"table:claim_stats,alias:cs"`

	UserID           string    `bun:"user_id,pk"`
	TotalSpent       int64     `bun:"total_spent,notnull,default:0"`
	TotalClaims      int       `bun:"total_claims,notnull"`
	SuccessfulClaims int       `bun:"successful_claims,notnull"`
	LastClaimAt      time.Time `bun:"last_claim_at,notnull"`
	DailyClaims      int       `bun:"daily_claims,notnull"`
	WeeklyClaims     int       `bun:"weekly_claims,notnull"`
	UpdatedAt        time.Time `bun:"updated_at,notnull"`
}
