package models

import (
	"time"

	"github.com/uptrace/bun"
)

type QuestDefinition struct {
	bun.BaseModel `bun:"table:quest_definitions,alias:qd"`

	ID                  int64                  `bun:"id,pk,autoincrement"`
	QuestID             string                 `bun:"quest_id,notnull,unique"`
	Name                string                 `bun:"name,notnull"`
	Description         string                 `bun:"description,notnull"`
	Tier                int                    `bun:"tier,notnull"`
	Type                string                 `bun:"type,notnull"`     // daily, weekly, monthly
	Category            string                 `bun:"category,notnull"` // trainee, debut, idol, special
	RequirementType     string                 `bun:"requirement_type,notnull"`
	RequirementTarget   string                 `bun:"requirement_target"`
	RequirementCount    int                    `bun:"requirement_count,notnull"`
	RequirementMetadata map[string]interface{} `bun:"requirement_metadata,type:jsonb"`
	RewardSnowflakes    int64                  `bun:"reward_snowflakes,notnull,default:0"`
	RewardVials         int                    `bun:"reward_vials,notnull,default:0"`
	RewardXP            int                    `bun:"reward_xp,notnull,default:0"`
	RewardItems         map[string]interface{} `bun:"reward_items,type:jsonb"`
	ChainID             *int64                 `bun:"chain_id"`
	CreatedAt           time.Time              `bun:"created_at,notnull"`
	UpdatedAt           time.Time              `bun:"updated_at,notnull"`
}

// Quest type constants
const (
	QuestTypeDaily   = "daily"
	QuestTypeWeekly  = "weekly"
	QuestTypeMonthly = "monthly"
)

// Quest category constants
const (
	QuestCategoryTrainee = "trainee"
	QuestCategoryDebut   = "debut"
	QuestCategoryIdol    = "idol"
	QuestCategorySpecial = "special"
)

// Requirement type constants
const (
    RequirementTypeCommandCount         = "command_count"
    RequirementTypeCommandUsage         = "command_usage"
    RequirementTypeSpecificCommand      = "specific_command"
    RequirementTypeCardClaim            = "card_claim"
    RequirementTypeCardLevelUp          = "card_levelup"
    RequirementTypeCardDraw             = "card_draw"
    RequirementTypeCardTrade            = "card_trade"
	RequirementTypeAuctionBid           = "auction_bid"
	RequirementTypeAuctionWin           = "auction_win"
	RequirementTypeAuctionCreate        = "auction_create"
	RequirementTypeSnowflakesEarned     = "snowflakes_earned"
	RequirementTypeSnowflakesFromSource = "snowflakes_from_source"
	RequirementTypeWorkCommand          = "work_command"
	RequirementTypeWorkDays             = "work_days"
	RequirementTypeDailyComplete        = "daily_complete"
	RequirementTypeWeeklyComplete       = "weekly_complete"
	RequirementTypeCombo                = "combo"
	RequirementTypeAscend               = "ascend"
)
