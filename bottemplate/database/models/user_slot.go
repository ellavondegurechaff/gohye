package models

import (
	"time"

	"github.com/uptrace/bun"
)

type UserSlot struct {
	bun.BaseModel `bun:"table:user_slots,alias:us"`

	ID          int64     `bun:"id,pk,autoincrement"`
	DiscordID   string    `bun:"discord_id,notnull"`
	EffectName  string    `bun:"effect_name"`
	SlotExpires time.Time `bun:"slot_expires"`
	Cooldown    time.Time `bun:"cooldown"`
	IsActive    bool      `bun:"is_active,notnull,default:true"`
	CreatedAt   time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt   time.Time `bun:"updated_at,notnull"`
}
