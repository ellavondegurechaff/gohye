package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
)

// InitializeQuestData inserts all quest definitions into the database
func (db *DB) InitializeQuestData(ctx context.Context) error {
	// Check if quests already exist
	var questCount int
	err := db.pool.QueryRow(ctx, "SELECT COUNT(*) FROM quest_definitions").Scan(&questCount)
	if err == nil && questCount > 0 {
		slog.Info("Quest data already initialized, skipping", 
			slog.Int("existing_quests", questCount))
		return nil
	}

	slog.Info("Initializing quest definitions...")

	// Daily Quests - Tier 1
	dailyTier1 := []models.QuestDefinition{
		{
			QuestID:          "daily_workaholic",
			Name:             "Workaholic",
			Description:      "Use /work 3 times",
			Tier:             1,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeWorkCommand,
			RequirementCount: 3,
			RewardSnowflakes: 250,
			RewardVials:      20,
			RewardXP:         25,
		},
		{
			QuestID:          "daily_vial_collector",
			Name:             "Vial Collector",
			Description:      "Liquefy cards to gain 100 vials",
			Tier:             1,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeSpecificCommand,
			RequirementTarget: "liquefy",
			RequirementCount: 100,
			RewardSnowflakes: 250,
			RewardVials:      20,
			RewardXP:         25,
		},
		{
			QuestID:          "daily_card_claimer",
			Name:             "Card Claimer",
			Description:      "Claim any card",
			Tier:             1,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeCardClaim,
			RequirementCount: 1,
			RewardSnowflakes: 250,
			RewardVials:      20,
			RewardXP:         25,
		},
		{
			QuestID:          "daily_draw_master",
			Name:             "Draw Master",
			Description:      "Draw a card 1 time",
			Tier:             1,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeCardDraw,
			RequirementCount: 1,
			RewardSnowflakes: 250,
			RewardVials:      20,
			RewardXP:         25,
		},
		{
			QuestID:          "daily_trader",
			Name:             "Trader",
			Description:      "Trade 1 card with another user",
			Tier:             1,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeCardTrade,
			RequirementCount: 1,
			RewardSnowflakes: 250,
			RewardVials:      20,
			RewardXP:         25,
		},
	}

	// Daily Quests - Tier 2
	dailyTier2 := []models.QuestDefinition{
		{
			QuestID:          "daily_command_master",
			Name:             "Command Master",
			Description:      "Use 5 different commands",
			Tier:             2,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeCommandCount,
			RequirementCount: 5,
			RewardSnowflakes: 400,
			RewardVials:      30,
			RewardXP:         35,
		},
		{
			QuestID:          "daily_auction_participant",
			Name:             "Auction Participant",
			Description:      "Bid on any auction",
			Tier:             2,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeAuctionBid,
			RequirementCount: 1,
			RewardSnowflakes: 400,
			RewardVials:      30,
			RewardXP:         35,
		},
		{
			QuestID:          "daily_snowflake_earner",
			Name:             "Snowflake Earner",
			Description:      "Earn 5000 snowflakes from any source",
			Tier:             2,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeSnowflakesEarned,
			RequirementCount: 5000,
			RewardSnowflakes: 400,
			RewardVials:      30,
			RewardXP:         35,
		},
		{
			QuestID:          "daily_card_upgrader",
			Name:             "Card Upgrader",
			Description:      "Level up any card 2 times",
			Tier:             2,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeCardLevelUp,
			RequirementCount: 2,
			RewardSnowflakes: 400,
			RewardVials:      30,
			RewardXP:         35,
		},
		{
			QuestID:          "daily_ascender",
			Name:             "Ascender",
			Description:      "Ascend 1 card",
			Tier:             2,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeAscend,
			RequirementCount: 1,
			RewardSnowflakes: 400,
			RewardVials:      30,
			RewardXP:         35,
		},
		{
			QuestID:          "daily_flake_farmer",
			Name:             "Flake Farmer",
			Description:      "Earn 3000 snowflakes from any source",
			Tier:             2,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeSnowflakesEarned,
			RequirementCount: 3000,
			RewardSnowflakes: 400,
			RewardVials:      30,
			RewardXP:         35,
		},
	}

	// Daily Quests - Tier 3
	dailyTier3 := []models.QuestDefinition{
		{
			QuestID:          "daily_collection_master",
			Name:             "Collection Master",
			Description:      "Claim 10 cards and level up 5 cards",
			Tier:             3,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeCombo,
			RequirementCount: 1,
			RequirementMetadata: map[string]interface{}{
				"claim": 10,
				"levelup": 5,
			},
			RewardSnowflakes: 650,
			RewardVials:      50,
			RewardXP:         50,
		},
		{
			QuestID:          "daily_mega_worker",
			Name:             "Mega Worker",
			Description:      "Work 10 times",
			Tier:             3,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeWorkCommand,
			RequirementCount: 10,
			RewardSnowflakes: 650,
			RewardVials:      50,
			RewardXP:         50,
		},
		{
			QuestID:          "daily_auction_winner",
			Name:             "Auction Winner",
			Description:      "Win an auction",
			Tier:             3,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeAuctionWin,
			RequirementCount: 1,
			RewardSnowflakes: 650,
			RewardVials:      50,
			RewardXP:         50,
		},
		{
			QuestID:          "daily_auction_creator",
			Name:             "Auction Creator",
			Description:      "Create an auction",
			Tier:             3,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeAuctionCreate,
			RequirementCount: 1,
			RewardSnowflakes: 650,
			RewardVials:      50,
			RewardXP:         50,
		},
		{
			QuestID:          "daily_combo_player",
			Name:             "Combo Player",
			Description:      "/claim 8 cards, /work 3 times, /levelup 10 times and auction 1 card",
			Tier:             3,
			Type:             models.QuestTypeDaily,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeCombo,
			RequirementCount: 1,
			RequirementMetadata: map[string]interface{}{
				"claim": 8,
				"work": 3,
				"levelup": 10,
				"auction_create": 1,
			},
			RewardSnowflakes: 650,
			RewardVials:      50,
			RewardXP:         50,
		},
	}

	// Weekly Quests - Tier 1
	weeklyTier1 := []models.QuestDefinition{
		{
			QuestID:          "weekly_card_collector",
			Name:             "Card Collector",
			Description:      "Claim 20 cards",
			Tier:             1,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeCardClaim,
			RequirementCount: 20,
			RewardSnowflakes: 1000,
			RewardVials:      40,
			RewardXP:         50,
		},
		{
			QuestID:          "weekly_work_enthusiast",
			Name:             "Work Enthusiast",
			Description:      "/work 15 times",
			Tier:             1,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeWorkCommand,
			RequirementCount: 15,
			RewardSnowflakes: 1000,
			RewardVials:      40,
			RewardXP:         50,
		},
		{
			QuestID:          "weekly_card_trader",
			Name:             "Card Trader",
			Description:      "Trade 5 cards",
			Tier:             1,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeCardTrade,
			RequirementCount: 5,
			RewardSnowflakes: 1000,
			RewardVials:      40,
			RewardXP:         50,
		},
		{
			QuestID:          "weekly_draw_expert",
			Name:             "Draw Expert",
			Description:      "Draw 10 cards",
			Tier:             1,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeCardDraw,
			RequirementCount: 10,
			RewardSnowflakes: 1000,
			RewardVials:      40,
			RewardXP:         50,
		},
		{
			QuestID:          "weekly_auction_bidder",
			Name:             "Auction Bidder",
			Description:      "Bid on 3 auctions",
			Tier:             1,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeAuctionBid,
			RequirementCount: 3,
			RewardSnowflakes: 1000,
			RewardVials:      40,
			RewardXP:         50,
		},
	}

	// Weekly Quests - Tier 2
	weeklyTier2 := []models.QuestDefinition{
		{
			QuestID:          "weekly_command_variety",
			Name:             "Command Variety",
			Description:      "Use 15 different commands",
			Tier:             2,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeCommandCount,
			RequirementCount: 15,
			RewardSnowflakes: 1500,
			RewardVials:      60,
			RewardXP:         75,
		},
		{
			QuestID:          "weekly_work_days",
			Name:             "Work Days",
			Description:      "Work on 5 different days",
			Tier:             2,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeWorkDays,
			RequirementCount: 5,
			RewardSnowflakes: 1500,
			RewardVials:      60,
			RewardXP:         75,
		},
		{
			QuestID:          "weekly_snowflake_accumulator",
			Name:             "Snowflake Accumulator",
			Description:      "Earn 20,000 snowflakes from any source",
			Tier:             2,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeSnowflakesEarned,
			RequirementCount: 20000,
			RewardSnowflakes: 1500,
			RewardVials:      60,
			RewardXP:         75,
		},
		{
			QuestID:          "weekly_card_master",
			Name:             "Card Master",
			Description:      "Level up 10 cards",
			Tier:             2,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeCardLevelUp,
			RequirementCount: 10,
			RewardSnowflakes: 1500,
			RewardVials:      60,
			RewardXP:         75,
		},
		{
			QuestID:          "weekly_daily_completer",
			Name:             "Daily Completer",
			Description:      "Complete all 3 daily quests on 3 separate days",
			Tier:             2,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeDailyComplete,
			RequirementCount: 3,
			RewardSnowflakes: 1500,
			RewardVials:      60,
			RewardXP:         75,
		},
		{
			QuestID:          "weekly_balanced_routine",
			Name:             "Balanced Routine",
			Description:      "Level up cards on 5 different days",
			Tier:             2,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeCardLevelUp,
			RequirementCount: 5,
			RequirementMetadata: map[string]interface{}{
				"track_days": true,
			},
			RewardSnowflakes: 1500,
			RewardVials:      60,
			RewardXP:         75,
		},
	}

	// Weekly Quests - Tier 3
	weeklyTier3 := []models.QuestDefinition{
		{
			QuestID:          "weekly_mega_collector",
			Name:             "Mega Collector",
			Description:      "Claim 50 cards and level up 20 cards",
			Tier:             3,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeCombo,
			RequirementCount: 1,
			RequirementMetadata: map[string]interface{}{
				"claim": 50,
				"levelup": 20,
			},
			RewardSnowflakes: 2500,
			RewardVials:      100,
			RewardXP:         100,
		},
		{
			QuestID:          "weekly_full_dedication",
			Name:             "Full Dedication",
			Description:      "Complete all 3 daily quests on 6 separate days",
			Tier:             3,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeDailyComplete,
			RequirementCount: 6,
			RewardSnowflakes: 2500,
			RewardVials:      100,
			RewardXP:         100,
		},
		{
			QuestID:          "weekly_auction_expert",
			Name:             "Auction Expert",
			Description:      "Win 3 auctions",
			Tier:             3,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeAuctionWin,
			RequirementCount: 3,
			RewardSnowflakes: 2500,
			RewardVials:      100,
			RewardXP:         100,
		},
		{
			QuestID:          "weekly_trade_master",
			Name:             "Trade Master",
			Description:      "Trade 15 cards",
			Tier:             3,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeCardTrade,
			RequirementCount: 15,
			RewardSnowflakes: 2500,
			RewardVials:      100,
			RewardXP:         100,
		},
		{
			QuestID:          "weekly_snowflake_millionaire",
			Name:             "Snowflake Millionaire",
			Description:      "Earn 50,000 snowflakes from any source",
			Tier:             3,
			Type:             models.QuestTypeWeekly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeSnowflakesEarned,
			RequirementCount: 50000,
			RewardSnowflakes: 2500,
			RewardVials:      100,
			RewardXP:         100,
		},
	}

	// Monthly Quests - Tier 1
	monthlyTier1 := []models.QuestDefinition{
		{
			QuestID:          "monthly_card_hoarder",
			Name:             "Card Hoarder",
			Description:      "Claim 100 cards",
			Tier:             1,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeCardClaim,
			RequirementCount: 100,
			RewardSnowflakes: 3000,
			RewardVials:      80,
			RewardXP:         100,
		},
		{
			QuestID:          "monthly_work_marathon",
			Name:             "Work Marathon",
			Description:      "/work 60 times",
			Tier:             1,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeWorkCommand,
			RequirementCount: 60,
			RewardSnowflakes: 3000,
			RewardVials:      80,
			RewardXP:         100,
		},
		{
			QuestID:          "monthly_draw_addict",
			Name:             "Draw Addict",
			Description:      "Draw 50 cards",
			Tier:             1,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeCardDraw,
			RequirementCount: 50,
			RewardSnowflakes: 3000,
			RewardVials:      80,
			RewardXP:         100,
		},
		{
			QuestID:          "monthly_trade_network",
			Name:             "Trade Network",
			Description:      "Trade 25 cards",
			Tier:             1,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeCardTrade,
			RequirementCount: 25,
			RewardSnowflakes: 3000,
			RewardVials:      80,
			RewardXP:         100,
		},
		{
			QuestID:          "monthly_auction_regular",
			Name:             "Auction Regular",
			Description:      "Bid on 15 auctions",
			Tier:             1,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryTrainee,
			RequirementType:  models.RequirementTypeAuctionBid,
			RequirementCount: 15,
			RewardSnowflakes: 3000,
			RewardVials:      80,
			RewardXP:         100,
		},
	}

	// Monthly Quests - Tier 2
	monthlyTier2 := []models.QuestDefinition{
		{
			QuestID:          "monthly_command_explorer",
			Name:             "Command Explorer",
			Description:      "Use 30 different commands",
			Tier:             2,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeCommandCount,
			RequirementCount: 30,
			RewardSnowflakes: 5000,
			RewardVials:      120,
			RewardXP:         150,
		},
		{
			QuestID:          "monthly_work_consistency",
			Name:             "Work Consistency",
			Description:      "Work on 20 different days",
			Tier:             2,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeWorkDays,
			RequirementCount: 20,
			RewardSnowflakes: 5000,
			RewardVials:      120,
			RewardXP:         150,
		},
		{
			QuestID:          "monthly_card_evolution",
			Name:             "Card Evolution",
			Description:      "Level up 50 cards",
			Tier:             2,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeCardLevelUp,
			RequirementCount: 50,
			RewardSnowflakes: 5000,
			RewardVials:      120,
			RewardXP:         150,
		},
		{
			QuestID:          "monthly_snowflake_baron",
			Name:             "Snowflake Baron",
			Description:      "Earn 100,000 snowflakes from any source",
			Tier:             2,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeSnowflakesEarned,
			RequirementCount: 100000,
			RewardSnowflakes: 5000,
			RewardVials:      120,
			RewardXP:         150,
		},
		{
			QuestID:          "monthly_weekly_dedication",
			Name:             "Weekly Dedication",
			Description:      "Complete all 3 Weekly quests in 2 separate weeks",
			Tier:             2,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryDebut,
			RequirementType:  models.RequirementTypeWeeklyComplete,
			RequirementCount: 2,
			RewardSnowflakes: 5000,
			RewardVials:      120,
			RewardXP:         150,
		},
	}

	// Monthly Quests - Tier 3
	monthlyTier3 := []models.QuestDefinition{
		{
			QuestID:          "monthly_ultimate_collector",
			Name:             "Ultimate Collector",
			Description:      "Claim 200 cards and level up 100 cards",
			Tier:             3,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeCombo,
			RequirementCount: 1,
			RequirementMetadata: map[string]interface{}{
				"claim": 200,
				"levelup": 100,
			},
			RewardSnowflakes: 8000,
			RewardVials:      200,
			RewardXP:         200,
		},
		{
			QuestID:          "monthly_auction_magnate",
			Name:             "Auction Magnate",
			Description:      "Win 10 auctions",
			Tier:             3,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeAuctionWin,
			RequirementCount: 10,
			RewardSnowflakes: 8000,
			RewardVials:      200,
			RewardXP:         200,
		},
		{
			QuestID:          "monthly_trading_empire",
			Name:             "Trading Empire",
			Description:      "Trade 50 cards",
			Tier:             3,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeCardTrade,
			RequirementCount: 50,
			RewardSnowflakes: 8000,
			RewardVials:      200,
			RewardXP:         200,
		},
		{
			QuestID:          "monthly_ultimate_flipper",
			Name:             "Ultimate Flipper",
			Description:      "Earn 20,000 snowflakes through auctions",
			Tier:             3,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeSnowflakesFromSource,
			RequirementTarget: "auction",
			RequirementCount: 20000,
			RewardSnowflakes: 8000,
			RewardVials:      200,
			RewardXP:         200,
		},
		{
			QuestID:          "monthly_perfect_dedication",
			Name:             "Perfect Dedication",
			Description:      "Complete all Daily quests on 20 different days",
			Tier:             3,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeDailyComplete,
			RequirementCount: 20,
			RewardSnowflakes: 8000,
			RewardVials:      200,
			RewardXP:         200,
		},
		{
			QuestID:          "monthly_full_completion",
			Name:             "Full Completion",
			Description:      "Complete all Weekly quests every week this month",
			Tier:             3,
			Type:             models.QuestTypeMonthly,
			Category:         models.QuestCategoryIdol,
			RequirementType:  models.RequirementTypeWeeklyComplete,
			RequirementCount: 4,
			RewardSnowflakes: 8000,
			RewardVials:      200,
			RewardXP:         200,
		},
	}

	// Combine all quests
	allQuests := make([]models.QuestDefinition, 0)
	allQuests = append(allQuests, dailyTier1...)
	allQuests = append(allQuests, dailyTier2...)
	allQuests = append(allQuests, dailyTier3...)
	allQuests = append(allQuests, weeklyTier1...)
	allQuests = append(allQuests, weeklyTier2...)
	allQuests = append(allQuests, weeklyTier3...)
	allQuests = append(allQuests, monthlyTier1...)
	allQuests = append(allQuests, monthlyTier2...)
	allQuests = append(allQuests, monthlyTier3...)

	// Insert all quests
	for _, quest := range allQuests {
		_, err := db.bunDB.NewInsert().
			Model(&quest).
			On("CONFLICT (quest_id) DO UPDATE").
			Set("name = EXCLUDED.name").
			Set("description = EXCLUDED.description").
			Set("requirement_count = EXCLUDED.requirement_count").
			Set("reward_snowflakes = EXCLUDED.reward_snowflakes").
			Set("reward_vials = EXCLUDED.reward_vials").
			Set("reward_xp = EXCLUDED.reward_xp").
			Set("updated_at = NOW()").
			Set("requirement_metadata = EXCLUDED.requirement_metadata").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to insert quest %s: %w", quest.QuestID, err)
		}
	}

	slog.Info("Quest data initialization completed",
		slog.Int("total_quests", len(allQuests)))
	
	// Apply quest fixes for existing quests in database
	slog.Info("Applying quest fixes...")
	
	// Fix Level Addict quest to track days
	_, err = db.bunDB.NewUpdate().
		Model((*models.QuestDefinition)(nil)).
		Set("description = ?", "Use /levelup on 3 different days").
		Set("requirement_metadata = ?", map[string]interface{}{"track_days": true}).
		Set("updated_at = NOW()").
		Where("quest_id = ? OR (name = ? AND requirement_type = ?)", 
			"monthly_level_addict", "Level Addict", "card_levelup").
		Exec(ctx)
	if err != nil {
		slog.Warn("Failed to update Level Addict quest",
			slog.Any("error", err))
	}
	
	// Reset Level Addict progress for proper day tracking
	_, err = db.bunDB.NewUpdate().
		Model((*models.UserQuestProgress)(nil)).
		Set("current_progress = 0").
		Set("metadata = ?", map[string]interface{}{"levelup_days": map[string]bool{}}).
		Set("completed = false").
		Set("claimed = false").
		Where("quest_id IN (SELECT quest_id FROM quest_definitions WHERE (quest_id = ? OR name = ?) AND requirement_type = ?)",
			"monthly_level_addict", "Level Addict", "card_levelup").
		Exec(ctx)
	if err != nil {
		slog.Warn("Failed to reset Level Addict progress",
			slog.Any("error", err))
	}
	
	// Reset Balanced Routine progress if it exists
	_, err = db.bunDB.NewUpdate().
		Model((*models.UserQuestProgress)(nil)).
		Set("current_progress = 0").
		Set("metadata = ?", map[string]interface{}{"levelup_days": map[string]bool{}}).
		Set("completed = false").
		Set("claimed = false").
		Where("quest_id = ?", "weekly_balanced_routine").
		Exec(ctx)
	if err != nil {
		slog.Warn("Failed to reset Balanced Routine progress",
			slog.Any("error", err))
	}
	
	slog.Info("Quest fixes applied successfully")
	
	return nil
}