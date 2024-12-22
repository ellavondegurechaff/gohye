package models

import (
	"time"

	"github.com/uptrace/bun"
)

type ClaimStats struct {
	bun.BaseModel `bun:"table:claim_stats,alias:cs"`

	UserID        string    `bun:"user_id,pk"`
	TotalSpent    int64     `bun:"total_spent"`
	TotalClaims   int       `bun:"total_claims"`
	DailyClaims   int       `bun:"daily_claims"`
	LastClaimDate time.Time `bun:"last_claim_date"`
	LastClaimAt   time.Time `bun:"last_claim_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}
