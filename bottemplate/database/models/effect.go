package models

import (
	"time"

	"github.com/uptrace/bun"
)

// EffectItem represents an effect item that can be purchased from the shop
type EffectItem struct {
	bun.BaseModel `bun:"table:effect_items,alias:ei"`

	ID          string    `bun:"id,pk"`
	Name        string    `bun:"name,notnull"`
	Description string    `bun:"description,notnull"`
	Type        string    `bun:"type,notnull"` // recipe, consumable, passive, active
	Price       int64     `bun:"price,notnull"`
	Currency    string    `bun:"currency,notnull"` // tomato, vials etc
	Duration    int       `bun:"duration,notnull"` // in hours
	Level       int       `bun:"level,notnull"`    // star level effect
	Recipe      []int64   `bun:"recipe,type:jsonb"`
	Cooldown    int       `bun:"cooldown,notnull,default:0"` // cooldown in hours for active effects
	Passive     bool      `bun:"passive,notnull,default:false"` // is this a passive effect
	Animated    bool      `bun:"animated,notnull,default:false"` // legacy animated flag
	CreatedAt   time.Time `bun:"created_at,notnull"`
	UpdatedAt   time.Time `bun:"updated_at,notnull"`
}

// UserEffect represents an effect owned by a user
type UserEffect struct {
	bun.BaseModel `bun:"table:user_effects,alias:ue"`

	ID             int64      `bun:"id,pk,autoincrement"`
	UserID         string     `bun:"user_id,notnull"`
	EffectID       string     `bun:"effect_id,notnull"`
	IsRecipe       bool       `bun:"is_recipe,notnull,default:false"`
	RecipeCards    []int64    `bun:"recipe_cards,type:jsonb"`
	Active         bool       `bun:"active,notnull,default:false"`
	Uses           int        `bun:"uses,notnull,default:0"`
	ExpiresAt      *time.Time `bun:"expires_at"` // when effect expires (legacy: expires)
	CooldownEndsAt *time.Time `bun:"cooldown_ends_at"` // when cooldown ends (legacy: cooldownends)
	Notified       bool       `bun:"notified,notnull,default:true"` // legacy notification tracking
	CreatedAt      time.Time  `bun:"created_at,notnull"`
	UpdatedAt      time.Time  `bun:"updated_at,notnull"`
}

// UserInventory represents items in a user's inventory
type UserInventory struct {
	bun.BaseModel `bun:"table:user_inventory,alias:ui"`

	UserID    string    `bun:"user_id,pk"`
	ItemID    string    `bun:"item_id,pk"`
	Col       string    `bun:"col"` // legacy collection field
	Amount    int       `bun:"amount,notnull,default:1"`
	Cards     []int64   `bun:"cards,type:jsonb"` // legacy cards array
	Acquired  time.Time `bun:"acquired,notnull"` // legacy acquired time
	CreatedAt time.Time `bun:"created_at,notnull"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}

// EffectType constants
const (
	EffectTypeRecipe     = "recipe"
	EffectTypeConsumable = "consumable"
	EffectTypePassive    = "passive"
	EffectTypeActive     = "active"
)

// Currency constants
const (
	CurrencyTomato = "tomato"
	CurrencyVials  = "vials"
)
