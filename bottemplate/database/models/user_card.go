// models/user_card.go
package models

import (
	"time"

	"github.com/uptrace/bun"
)

type UserCard struct {
	bun.BaseModel `bun:"table:user_cards,alias:uc"`

	ID       int64     `bun:"id,pk,autoincrement"`
	UserID   string    `bun:"user_id,notnull"`
	CardID   int64     `bun:"card_id,notnull"`
	Favorite bool      `bun:"favorite,notnull,default:false"`
	Locked   bool      `bun:"locked,notnull,default:false"`
	Amount   int64     `bun:"amount,notnull,default:1"`
	Rating   int64     `bun:"rating,notnull,default:0"`
	Obtained time.Time `bun:"obtained,notnull,default:current_timestamp"`
	Exp      int64     `bun:"exp,notnull,default:0"`
	Mark     string    `bun:"mark,type:text,default:''"`

	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}
