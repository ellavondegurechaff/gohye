package models

import (
	"time"
)

type Wishlist struct {
	ID        int64     `bun:"id,pk,autoincrement"`
	UserID    string    `bun:"user_id,notnull"`
	CardID    int64     `bun:"card_id,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}
