package models

import "time"

type UserRecipe struct {
	UserID    string    `bun:"user_id,pk"`
	ItemID    string    `bun:"item_id,pk"`
	CardIDs   []int64   `bun:"card_ids,array"`
	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}
