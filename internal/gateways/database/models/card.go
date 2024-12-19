package models

import (
	"time"

	"github.com/uptrace/bun"
)

type SearchFilters struct {
	Name       string
	ID         int64
	Level      int
	Collection string
	Type       string
	Animated   bool
}

type Card struct {
	bun.BaseModel `bun:"table:cards,alias:c"`

	ID        int64     `bun:"id,pk"` // Using the ID from JSON as primary key
	Name      string    `bun:"name,notnull"`
	Level     int       `bun:"level,notnull"`
	Animated  bool      `bun:"animated,notnull"`
	ColID     string    `bun:"col_id,notnull,type:text"`
	Tags      []string  `bun:"tags,type:jsonb"`
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`

	// Relations
	Collection *Collection `bun:"rel:belongs-to,join:col_id=id"`
}
