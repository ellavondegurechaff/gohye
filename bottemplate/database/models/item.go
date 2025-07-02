package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Item struct {
	bun.BaseModel `bun:"table:items,alias:i"`

	ID          string                 `bun:"id,pk"`
	Name        string                 `bun:"name,notnull"`
	Description string                 `bun:"description"`
	Emoji       string                 `bun:"emoji,notnull"`
	Type        string                 `bun:"type,notnull"`
	Rarity      int                    `bun:"rarity,notnull"`
	MaxStack    int                    `bun:"max_stack,notnull"`
	Metadata    map[string]interface{} `bun:"metadata,type:jsonb"`
	CreatedAt   time.Time              `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt   time.Time              `bun:"updated_at,notnull,default:current_timestamp"`
}

type UserItem struct {
	bun.BaseModel `bun:"table:user_items,alias:ui"`

	UserID     string    `bun:"user_id,pk"`
	ItemID     string    `bun:"item_id,pk"`
	Quantity   int       `bun:"quantity,notnull"`
	ObtainedAt time.Time `bun:"obtained_at,notnull,default:current_timestamp"`
	UpdatedAt  time.Time `bun:"updated_at,notnull,default:current_timestamp"`

	// Relations
	Item *Item `bun:"rel:has-one,join:item_id=id"`
}

const (
	ItemTypeMaterial   = "material"
	ItemTypeConsumable = "consumable"
	ItemTypeSpecial    = "special"

	// Material item IDs
	ItemBrokenDisc    = "broken_disc"
	ItemMicrophone    = "microphone"
	ItemForgottenSong = "forgotten_song"
)