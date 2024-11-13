package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Claim struct {
	bun.BaseModel `bun:"table:claims,alias:cl"`

	ID        int64     `bun:"id,pk,autoincrement"`
	CardID    int64     `bun:"card_id,notnull"`
	UserID    string    `bun:"user_id,notnull"`
	ClaimedAt time.Time `bun:"claimed_at,notnull"`
	Expires   time.Time `bun:"expires,notnull"`
}

type ClaimStatus int

const (
	ClaimStatusAvailable ClaimStatus = iota
	ClaimStatusClaimed
	ClaimStatusExpired
)
