package models

import (
	"time"

	"github.com/uptrace/bun"
)

type UserQuestProgress struct {
	bun.BaseModel `bun:"table:user_quest_progress,alias:uqp"`

	ID              int64                  `bun:"id,pk,autoincrement"`
	UserID          string                 `bun:"user_id,notnull"`
	QuestID         string                 `bun:"quest_id,notnull"`
	CurrentProgress int                    `bun:"current_progress,notnull,default:0"`
	Completed       bool                   `bun:"completed,notnull,default:false"`
	Claimed         bool                   `bun:"claimed,notnull,default:false"`
	Milestone25     bool                   `bun:"milestone_25,notnull,default:false"`
	Milestone50     bool                   `bun:"milestone_50,notnull,default:false"`
	Milestone75     bool                   `bun:"milestone_75,notnull,default:false"`
	Metadata        map[string]interface{} `bun:"metadata,type:jsonb"`
	StartedAt       time.Time              `bun:"started_at,notnull"`
	CompletedAt     *time.Time             `bun:"completed_at"`
	ClaimedAt       *time.Time             `bun:"claimed_at"`
	ExpiresAt       time.Time              `bun:"expires_at,notnull"`
	CreatedAt       time.Time              `bun:"created_at,notnull"`
	UpdatedAt       time.Time              `bun:"updated_at,notnull"`

	// Relations
	QuestDefinition *QuestDefinition `bun:"rel:has-one,join:quest_id=quest_id"`
}

// CheckMilestones checks if quest has reached milestone percentages and returns newly achieved milestones
func (q *UserQuestProgress) CheckMilestones() []int {
	if q.QuestDefinition == nil {
		return nil
	}

	var newMilestones []int
	progress := float64(q.CurrentProgress) / float64(q.QuestDefinition.RequirementCount) * 100

	if progress >= 25 && !q.Milestone25 {
		q.Milestone25 = true
		newMilestones = append(newMilestones, 25)
	}
	if progress >= 50 && !q.Milestone50 {
		q.Milestone50 = true
		newMilestones = append(newMilestones, 50)
	}
	if progress >= 75 && !q.Milestone75 {
		q.Milestone75 = true
		newMilestones = append(newMilestones, 75)
	}
	if progress >= 100 && !q.Completed {
		q.Completed = true
		now := time.Now()
		q.CompletedAt = &now
		newMilestones = append(newMilestones, 100)
	}

	return newMilestones
}

// GetProgressPercentage returns the current progress as a percentage
func (q *UserQuestProgress) GetProgressPercentage() float64 {
	if q.QuestDefinition == nil || q.QuestDefinition.RequirementCount == 0 {
		return 0
	}

	percentage := float64(q.CurrentProgress) / float64(q.QuestDefinition.RequirementCount) * 100
	if percentage > 100 {
		percentage = 100
	}

	return percentage
}
