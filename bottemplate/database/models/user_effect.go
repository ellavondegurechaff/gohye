package models

import (
	"time"

	"github.com/uptrace/bun"
)

type UserEffect struct {
	bun.BaseModel `bun:"table:user_effects,alias:ue"`

	ID           int64     `bun:"id,pk,autoincrement"`
	UserID       string    `bun:"user_id,notnull"`
	EffectID     string    `bun:"effect_id,notnull"`
	Uses         int       `bun:"uses,notnull,default:0"`
	CooldownEnds time.Time `bun:"cooldown_ends"`
	Expires      time.Time `bun:"expires"`
	Notified     bool      `bun:"notified,notnull,default:true"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:"updated_at,notnull"`
}
