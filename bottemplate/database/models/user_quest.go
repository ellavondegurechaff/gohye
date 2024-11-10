package models

import (
	"time"

	"github.com/uptrace/bun"
)

type UserQuest struct {
	bun.BaseModel `bun:"table:user_quests,alias:uq"`

	ID        int64     `bun:"id,pk,autoincrement"`
	UserID    string    `bun:"user_id,notnull"`
	QuestID   string    `bun:"quest_id,notnull"`
	Type      string    `bun:"type,notnull"`
	Completed bool      `bun:"completed,notnull,default:false"`
	CreatedAt time.Time `bun:"created_at,notnull"`
	ExpiresAt time.Time `bun:"expires_at,notnull"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}
