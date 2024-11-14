// models/user_card.go
package models

import (
	"time"

	"github.com/uptrace/bun"
)

type UserCard struct {
	bun.BaseModel `bun:"table:user_cards,alias:uc"`

	ID        int64     `bun:"id,pk,autoincrement"`
	UserID    string    `bun:"user_id,notnull"`
	CardID    int64     `bun:"card_id,notnull"`
	Level     int       `bun:"level,notnull,default:1"`
	Balance   int64     `bun:"balance,notnull,default:0"`
	Amount    int64     `bun:"amount,notnull,default:1"`
	Favorite  bool      `bun:"favorite,notnull,default:false"`
	Locked    bool      `bun:"locked,notnull,default:false"`
	Rating    int64     `bun:"rating,notnull,default:0"`
	Obtained  time.Time `bun:"obtained,notnull"`
	Mark      string    `bun:"mark"`
	CreatedAt time.Time `bun:"created_at,notnull"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}
