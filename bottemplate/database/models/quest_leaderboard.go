package models

import (
	"time"

	"github.com/uptrace/bun"
)

type QuestLeaderboard struct {
	bun.BaseModel `bun:"table:quest_leaderboards,alias:ql"`

	ID              int64     `bun:"id,pk,autoincrement"`
	PeriodType      string    `bun:"period_type,notnull"` // daily, weekly, monthly
	PeriodStart     time.Time `bun:"period_start,notnull"`
	UserID          string    `bun:"user_id,notnull"`
	QuestsCompleted int       `bun:"quests_completed,notnull,default:0"`
	PointsEarned    int       `bun:"points_earned,notnull,default:0"`
	CreatedAt       time.Time `bun:"created_at,notnull"`
	UpdatedAt       time.Time `bun:"updated_at,notnull"`
}
