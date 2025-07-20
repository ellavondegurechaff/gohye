package models

import (
	"time"

	"github.com/uptrace/bun"
)

type CollectionProgress struct {
	bun.BaseModel `bun:"table:collection_progress,alias:cp"`

	ID           int64     `bun:"id,pk,autoincrement"`
	UserID       string    `bun:"user_id,notnull"`
	CollectionID string    `bun:"collection_id,notnull"`
	TotalCards   int       `bun:"total_cards,notnull"`
	OwnedCards   int       `bun:"owned_cards,notnull"`
	Percentage   float64   `bun:"percentage,notnull"`
	IsCompleted  bool      `bun:"is_completed,notnull,default:false"`
	IsFragment   bool      `bun:"is_fragment,notnull,default:false"`
	LastUpdated  time.Time `bun:"last_updated,notnull"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:"updated_at,notnull"`

	// Relations
	User       *User       `bun:"rel:belongs-to,join:user_id=discord_id"`
	Collection *Collection `bun:"rel:belongs-to,join:collection_id=id"`
}

type CollectionReset struct {
	bun.BaseModel `bun:"table:collection_resets,alias:cr"`

	ID            int64          `bun:"id,pk,autoincrement"`
	UserID        string         `bun:"user_id,notnull"`
	CollectionID  string         `bun:"collection_id,notnull"`
	ResetCount    int            `bun:"reset_count,notnull,default:1"`
	CloutEarned   int            `bun:"clout_earned,notnull"`
	FlakesEarned  int64          `bun:"flakes_earned,notnull"`
	CardsConsumed map[string]int `bun:"cards_consumed,type:jsonb"`
	ResetAt       time.Time      `bun:"reset_at,notnull,default:current_timestamp"`

	// Relations
	User       *User       `bun:"rel:belongs-to,join:user_id=discord_id"`
	Collection *Collection `bun:"rel:belongs-to,join:collection_id=id"`
}

type ResetRequirements struct {
	OneStars   int `json:"1"`
	TwoStars   int `json:"2"`
	ThreeStars int `json:"3"`
	FourStars  int `json:"4"`
	Total      int `json:"total"`
}

// CollectionProgressResult represents the result of a collection leaderboard query
// Used for aggregated progress data without persistence
type CollectionProgressResult struct {
	DiscordID  string  `bun:"discord_id"`
	Username   string  `bun:"username"`
	OwnedCards int     `bun:"owned_cards"`
	Progress   float64 `bun:"progress"`
}
