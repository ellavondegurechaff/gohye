package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Collection struct {
	bun.BaseModel `bun:"table:collections,alias:col"`

	ID         string    `bun:"id,pk"`
	Name       string    `bun:"name,notnull"`
	Origin     string    `bun:"origin,notnull"`
	Aliases    []string  `bun:"aliases,type:jsonb"`
	Promo      bool      `bun:"promo,notnull"`
	Compressed bool      `bun:"compressed,notnull"`
	Tags       []string  `bun:"tags,type:jsonb"`
	CreatedAt  time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt  time.Time `bun:"updated_at,notnull"`

	// Relations
	Cards []*Card `bun:"rel:has-many,join:id=col_id"`
}
