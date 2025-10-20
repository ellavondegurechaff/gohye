package models

import (
	"time"

	"github.com/uptrace/bun"
)

type TradeStatus string

const (
	TradePending  TradeStatus = "pending"
	TradeAccepted TradeStatus = "accepted"
	TradeDeclined TradeStatus = "declined"
	TradeExpired  TradeStatus = "expired"
)

type Trade struct {
	bun.BaseModel `bun:"table:trades,alias:t"`

	ID            int64       `bun:"id,pk,autoincrement"`
	TradeID       string      `bun:"trade_id,notnull,unique"`
	OffererID     string      `bun:"offerer_id,notnull"`
	TargetID      string      `bun:"target_id,notnull"`
	OffererCardID int64       `bun:"offerer_card_id,notnull"`
	TargetCardID  int64       `bun:"target_card_id,notnull"`
	Status        TradeStatus `bun:"status,notnull"`
	ExpiresAt     time.Time   `bun:"expires_at,notnull"`
	CreatedAt     time.Time   `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt     time.Time   `bun:"updated_at,notnull,default:current_timestamp"`

	// Relations for easy access
	OffererCard *Card `bun:"rel:belongs-to,join:offerer_card_id=id"`
	TargetCard  *Card `bun:"rel:belongs-to,join:target_card_id=id"`
}