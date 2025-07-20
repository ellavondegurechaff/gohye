package models

import (
	"time"

	"github.com/uptrace/bun"
)

type QuestChain struct {
	bun.BaseModel `bun:"table:quest_chains,alias:qc"`

	ID             int64                  `bun:"id,pk,autoincrement"`
	ChainID        string                 `bun:"chain_id,notnull,unique"`
	Name           string                 `bun:"name,notnull"`
	Description    string                 `bun:"description,notnull"`
	Theme          string                 `bun:"theme,notnull"` // trainee_journey, debut_story, etc.
	RequiredQuests []string               `bun:"required_quests,type:jsonb,notnull"`
	RewardData     map[string]interface{} `bun:"reward_data,type:jsonb,notnull"`
	CreatedAt      time.Time              `bun:"created_at,notnull"`
}

// Chain themes
const (
	ChainThemeTraineeJourney = "trainee_journey"
	ChainThemeDebutStory     = "debut_story"
	ChainThemeIdolLife       = "idol_life"
	ChainThemeWorldTour      = "world_tour"
)
