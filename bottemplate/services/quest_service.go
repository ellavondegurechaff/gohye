package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
)

type QuestService struct {
	questRepo repositories.QuestRepository
	userRepo  repositories.UserRepository
}

func NewQuestService(questRepo repositories.QuestRepository, userRepo repositories.UserRepository) *QuestService {
	return &QuestService{
		questRepo: questRepo,
		userRepo:  userRepo,
	}
}

// AssignDailyQuests assigns 3 daily quests (1 of each tier) to a user
func (qs *QuestService) AssignDailyQuests(ctx context.Context, userID string) error {
	// Check if user already has daily quests assigned
	activeQuests, err := qs.questRepo.GetActiveQuests(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get active quests: %w", err)
	}

	// Count daily quests
	dailyCount := 0
	for _, q := range activeQuests {
		if q.QuestDefinition != nil && q.QuestDefinition.Type == models.QuestTypeDaily {
			dailyCount++
		}
	}

	// If user already has 3 daily quests, don't assign more
	if dailyCount >= 3 {
		slog.Debug("User already has daily quests assigned",
			slog.String("user_id", userID),
			slog.Int("daily_count", dailyCount))
		return nil
	}

	// Assign one quest per tier
	for tier := 1; tier <= 3; tier++ {
		// Check if user already has a quest of this tier
		hasTier := false
		for _, q := range activeQuests {
			if q.QuestDefinition != nil &&
				q.QuestDefinition.Type == models.QuestTypeDaily &&
				q.QuestDefinition.Tier == tier {
				hasTier = true
				break
			}
		}

		if hasTier {
			continue
		}

		// Get random quest for this tier
		quests, err := qs.questRepo.GetRandomQuestsByTier(ctx, models.QuestTypeDaily, tier, 1)
		if err != nil || len(quests) == 0 {
			slog.Error("Failed to get random quest for tier",
				slog.Int("tier", tier),
				slog.Any("error", err))
			continue
		}

		// Create quest progress
		progress := &models.UserQuestProgress{
			UserID:    userID,
			QuestID:   quests[0].QuestID,
			ExpiresAt: qs.getNextReset(models.QuestTypeDaily),
		}

		if err := qs.questRepo.CreateQuestProgress(ctx, progress); err != nil {
			slog.Error("Failed to create quest progress",
				slog.String("user_id", userID),
				slog.String("quest_id", quests[0].QuestID),
				slog.Any("error", err))
			continue
		}

		slog.Info("Assigned daily quest to user",
			slog.String("user_id", userID),
			slog.String("quest_id", quests[0].QuestID),
			slog.Int("tier", tier))
	}

	return nil
}

// AssignWeeklyQuests assigns 3 weekly quests (1 of each tier) to a user
func (qs *QuestService) AssignWeeklyQuests(ctx context.Context, userID string) error {
	// Similar logic to daily quests but for weekly
	activeQuests, err := qs.questRepo.GetActiveQuests(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get active quests: %w", err)
	}

	weeklyCount := 0
	for _, q := range activeQuests {
		if q.QuestDefinition != nil && q.QuestDefinition.Type == models.QuestTypeWeekly {
			weeklyCount++
		}
	}

	if weeklyCount >= 3 {
		return nil
	}

	for tier := 1; tier <= 3; tier++ {
		hasTier := false
		for _, q := range activeQuests {
			if q.QuestDefinition != nil &&
				q.QuestDefinition.Type == models.QuestTypeWeekly &&
				q.QuestDefinition.Tier == tier {
				hasTier = true
				break
			}
		}

		if hasTier {
			continue
		}

		quests, err := qs.questRepo.GetRandomQuestsByTier(ctx, models.QuestTypeWeekly, tier, 1)
		if err != nil || len(quests) == 0 {
			continue
		}

		progress := &models.UserQuestProgress{
			UserID:    userID,
			QuestID:   quests[0].QuestID,
			ExpiresAt: qs.getNextReset(models.QuestTypeWeekly),
		}

		if err := qs.questRepo.CreateQuestProgress(ctx, progress); err != nil {
			continue
		}
	}

	return nil
}

// AssignMonthlyQuests assigns 3 monthly quests (1 of each tier) to a user
func (qs *QuestService) AssignMonthlyQuests(ctx context.Context, userID string) error {
	// Similar logic for monthly quests
	activeQuests, err := qs.questRepo.GetActiveQuests(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get active quests: %w", err)
	}

	monthlyCount := 0
	for _, q := range activeQuests {
		if q.QuestDefinition != nil && q.QuestDefinition.Type == models.QuestTypeMonthly {
			monthlyCount++
		}
	}

	if monthlyCount >= 3 {
		return nil
	}

	for tier := 1; tier <= 3; tier++ {
		hasTier := false
		for _, q := range activeQuests {
			if q.QuestDefinition != nil &&
				q.QuestDefinition.Type == models.QuestTypeMonthly &&
				q.QuestDefinition.Tier == tier {
				hasTier = true
				break
			}
		}

		if hasTier {
			continue
		}

		quests, err := qs.questRepo.GetRandomQuestsByTier(ctx, models.QuestTypeMonthly, tier, 1)
		if err != nil || len(quests) == 0 {
			continue
		}

		progress := &models.UserQuestProgress{
			UserID:    userID,
			QuestID:   quests[0].QuestID,
			ExpiresAt: qs.getNextReset(models.QuestTypeMonthly),
		}

		if err := qs.questRepo.CreateQuestProgress(ctx, progress); err != nil {
			continue
		}
	}

	return nil
}

// UpdateProgress updates quest progress based on user actions
func (qs *QuestService) UpdateProgress(ctx context.Context, userID string, action string, metadata map[string]interface{}) error {
	// Get all active quests for the user
	activeQuests, err := qs.questRepo.GetActiveQuests(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get active quests: %w", err)
	}

	// Track quest completions for daily/weekly complete requirements
	completedQuestsByType := make(map[string]int)

	// Check each quest to see if this action contributes to it
	for _, quest := range activeQuests {
		if quest.Completed || quest.QuestDefinition == nil {
			continue
		}

		// Check if this action matches the quest requirement
		if qs.actionMatchesRequirement(action, quest.QuestDefinition, metadata) {
			// Handle special progress tracking based on requirement type
			shouldUpdate := false

			switch quest.QuestDefinition.RequirementType {
			case models.RequirementTypeSnowflakesEarned:
				// Accumulate snowflakes instead of incrementing
				if earned, ok := metadata["snowflakes_earned"].(int64); ok && earned > 0 {
					quest.CurrentProgress += int(earned)
					shouldUpdate = true
				}

			case models.RequirementTypeSnowflakesFromSource:
				// Accumulate snowflakes from specific source only
				if earned, ok := metadata["snowflakes_earned"].(int64); ok && earned > 0 {
					if source, ok := metadata["source"].(string); ok && source == quest.QuestDefinition.RequirementTarget {
						quest.CurrentProgress += int(earned)
						shouldUpdate = true
					}
				}

			case models.RequirementTypeWorkDays:
				// Track unique days for work command
				if action == "work" {
					shouldUpdate = qs.trackWorkDay(quest, metadata)
				}

			case models.RequirementTypeCardLevelUp:
				// Check if this quest tracks days instead of total levelups
				if quest.QuestDefinition.RequirementMetadata != nil {
					if trackDays, ok := quest.QuestDefinition.RequirementMetadata["track_days"].(bool); ok && trackDays {
						shouldUpdate = qs.trackLevelUpDay(quest, metadata)
					} else {
						// Standard levelup tracking
						quest.CurrentProgress++
						shouldUpdate = true
					}
				} else {
					// Standard levelup tracking
					quest.CurrentProgress++
					shouldUpdate = true
				}

			case models.RequirementTypeCommandCount:
				// Track unique commands used
				slog.Debug("Processing command count quest",
					slog.String("user_id", userID),
					slog.String("quest_id", quest.QuestID),
					slog.String("action", action),
					slog.Any("metadata", metadata),
					slog.Int("current_progress", quest.CurrentProgress),
					slog.Int("requirement", quest.QuestDefinition.RequirementCount))
				shouldUpdate = qs.trackUniqueCommand(quest, metadata)
				slog.Debug("Command tracking result",
					slog.String("user_id", userID),
					slog.String("quest_id", quest.QuestID),
					slog.Bool("should_update", shouldUpdate),
					slog.Int("new_progress", quest.CurrentProgress))

			case models.RequirementTypeCommandUsage:
				// Count every command execution (optionally filtered by target)
				quest.CurrentProgress++
				shouldUpdate = true

			case models.RequirementTypeCombo:
				// Track multiple requirements for combo quests
				shouldUpdate = qs.trackComboProgress(quest, action, metadata)

			default:
				// Standard increment for other quest types
				quest.CurrentProgress++
				shouldUpdate = true
			}

			if !shouldUpdate {
				continue
			}

			// Check milestones
			milestones := quest.CheckMilestones()

			// Update quest progress
			if err := qs.questRepo.UpdateQuestProgress(ctx, quest); err != nil {
				slog.Error("Failed to update quest progress",
					slog.String("user_id", userID),
					slog.String("quest_id", quest.QuestID),
					slog.Any("error", err))
				continue
			}

			// Log milestone achievements
			for _, milestone := range milestones {
				slog.Info("User reached quest milestone",
					slog.String("user_id", userID),
					slog.String("quest_id", quest.QuestID),
					slog.Int("milestone", milestone))
			}

			// Update leaderboard and track completions if quest is completed
			if quest.Completed && quest.QuestDefinition != nil {
				qs.updateLeaderboard(ctx, userID, quest.QuestDefinition)
				completedQuestsByType[quest.QuestDefinition.Type]++
			}
		}
	}

	// After processing all quests, check if any completions trigger completion quests
	if len(completedQuestsByType) > 0 {
		qs.updateCompletionQuests(ctx, userID, completedQuestsByType)
	}

	return nil
}

// ClaimRewards claims rewards for completed quests
func (qs *QuestService) ClaimRewards(ctx context.Context, userID string) (*QuestRewardResult, error) {
	// Get all unclaimed completed quests
	unclaimedQuests, err := qs.questRepo.GetUnclaimedQuests(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unclaimed quests: %w", err)
	}

	if len(unclaimedQuests) == 0 {
		return &QuestRewardResult{
			Success: false,
			Message: "No completed quests to claim!",
		}, nil
	}

	result := &QuestRewardResult{
		Success:         true,
		ClaimedQuests:   make([]ClaimedQuest, 0),
		TotalSnowflakes: 0,
		TotalVials:      0,
		TotalXP:         0,
	}

	// Count by type
	dailyCount := 0
	weeklyCount := 0
	monthlyCount := 0

	// Claim each quest
	for _, quest := range unclaimedQuests {
		if quest.QuestDefinition == nil {
			continue
		}

		// Mark as claimed
		quest.Claimed = true
		now := time.Now()
		quest.ClaimedAt = &now

		if err := qs.questRepo.UpdateQuestProgress(ctx, quest); err != nil {
			slog.Error("Failed to mark quest as claimed",
				slog.String("user_id", userID),
				slog.String("quest_id", quest.QuestID),
				slog.Any("error", err))
			continue
		}

		// Add to result
		claimed := ClaimedQuest{
			QuestName:        quest.QuestDefinition.Name,
			Type:             quest.QuestDefinition.Type,
			Tier:             quest.QuestDefinition.Tier,
			RewardSnowflakes: quest.QuestDefinition.RewardSnowflakes,
			RewardVials:      quest.QuestDefinition.RewardVials,
			RewardXP:         quest.QuestDefinition.RewardXP,
		}

		result.ClaimedQuests = append(result.ClaimedQuests, claimed)
		result.TotalSnowflakes += quest.QuestDefinition.RewardSnowflakes
		result.TotalVials += quest.QuestDefinition.RewardVials
		result.TotalXP += quest.QuestDefinition.RewardXP

		// Count by type
		switch quest.QuestDefinition.Type {
		case models.QuestTypeDaily:
			dailyCount++
		case models.QuestTypeWeekly:
			weeklyCount++
		case models.QuestTypeMonthly:
			monthlyCount++
		}
	}

	// Update user balance
	user, err := qs.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Update snowflakes
	if result.TotalSnowflakes > 0 {
		if err := qs.userRepo.UpdateBalance(ctx, userID, user.Balance+result.TotalSnowflakes); err != nil {
			return nil, fmt.Errorf("failed to update balance: %w", err)
		}

		// Track snowflakes for quest progress
		metadata := map[string]interface{}{
			"snowflakes_earned": int64(result.TotalSnowflakes),
			"source":            "quest_claim",
		}
		if err := qs.UpdateProgress(ctx, userID, "quest_claim", metadata); err != nil {
			slog.Debug("Failed to track snowflakes from quest claim",
				slog.String("user_id", userID),
				slog.Int64("amount", result.TotalSnowflakes),
				slog.Any("error", err))
		}
	}

	// TODO: Update vials and XP when those systems are implemented

	result.DailyCount = dailyCount
	result.WeeklyCount = weeklyCount
	result.MonthlyCount = monthlyCount

	return result, nil
}

// GetUserQuestStatus returns the current quest status for a user
func (qs *QuestService) GetUserQuestStatus(ctx context.Context, userID string) (*UserQuestStatus, error) {
	activeQuests, err := qs.questRepo.GetActiveQuests(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active quests: %w", err)
	}

	status := &UserQuestStatus{
		DailyQuests:   make([]*models.UserQuestProgress, 0),
		WeeklyQuests:  make([]*models.UserQuestProgress, 0),
		MonthlyQuests: make([]*models.UserQuestProgress, 0),
	}

	for _, quest := range activeQuests {
		if quest.QuestDefinition == nil {
			continue
		}

		switch quest.QuestDefinition.Type {
		case models.QuestTypeDaily:
			status.DailyQuests = append(status.DailyQuests, quest)
		case models.QuestTypeWeekly:
			status.WeeklyQuests = append(status.WeeklyQuests, quest)
		case models.QuestTypeMonthly:
			status.MonthlyQuests = append(status.MonthlyQuests, quest)
		}
	}

	return status, nil
}

// Helper methods

func (qs *QuestService) actionMatchesRequirement(action string, quest *models.QuestDefinition, metadata map[string]interface{}) bool {
	// Defensive check
	if quest == nil {
		return false
	}

	switch quest.RequirementType {
	case models.RequirementTypeSpecificCommand:
		return action == quest.RequirementTarget

	case models.RequirementTypeCommandCount:
		// Any command counts, tracked through metadata
		_, hasCommand := metadata["command"]
		return hasCommand

	case models.RequirementTypeCommandUsage:
		// Count every executed command via the central "command_count" action
		if action != "command_count" {
			return false
		}
		cmd, ok := metadata["command"].(string)
		if !ok || cmd == "" {
			return false
		}
		if quest.RequirementTarget != "" {
			return cmd == quest.RequirementTarget
		}
		return true

	case models.RequirementTypeCardClaim:
		return action == "claim"

	case models.RequirementTypeCardLevelUp:
		// Check if quest requires specific levelup behavior
		if quest.RequirementMetadata != nil {
			// Only count combine-style levelups if requested
			if onlyCombine, ok := quest.RequirementMetadata["only_combine"].(bool); ok && onlyCombine {
				if metadata != nil {
					if act, ok := metadata["action"].(string); ok && act == "combine" {
						return action == "levelup"
					}
				}
				return false
			}

			// Check if quest requires max level only
			if maxLevelOnly, ok := quest.RequirementMetadata["max_level_only"].(bool); ok && maxLevelOnly {
				// Check if the metadata indicates max level was reached
				if metadata != nil {
					if isMaxLevel, ok := metadata["is_max_level"].(bool); ok && isMaxLevel {
						return action == "levelup"
					}
				}
				return false
			}
			// Check if quest tracks days (like Level Addict)
			if trackDays, ok := quest.RequirementMetadata["track_days"].(bool); ok && trackDays {
				return action == "levelup"
			}
		}
		return action == "levelup"

	case models.RequirementTypeCardDraw:
		return action == "draw"

	case models.RequirementTypeCardTrade:
		return action == "trade"

	case models.RequirementTypeAuctionBid:
		return action == "auction_bid"

	case models.RequirementTypeAuctionWin:
		return action == "auction_win"

	case models.RequirementTypeAuctionCreate:
		return action == "auction_create"

	case models.RequirementTypeWorkCommand:
		return action == "work"

	case models.RequirementTypeWorkDays:
		// Work days are tracked specially
		return action == "work"

	case models.RequirementTypeSnowflakesEarned:
		if earned, ok := metadata["snowflakes_earned"].(int64); ok {
			// For snowflakes earned, we track the total amount
			// This requires special handling in UpdateProgress
			return earned > 0
		}
		return false

	case models.RequirementTypeSnowflakesFromSource:
		if earned, ok := metadata["snowflakes_earned"].(int64); ok {
			if earned > 0 {
				// Check if the source matches the requirement target
				if source, ok := metadata["source"].(string); ok {
					return source == quest.RequirementTarget
				}
			}
		}
		return false

	case models.RequirementTypeCombo:
		// Combo quests can match multiple actions
		if quest.RequirementMetadata != nil {
			if _, hasAction := quest.RequirementMetadata[action]; hasAction {
				return true
			}
		}
		return false

	case models.RequirementTypeDailyComplete, models.RequirementTypeWeeklyComplete:
		// These are handled specially after other quests complete
		return false

	case models.RequirementTypeAscend:
		return action == "ascend"

	default:
		return false
	}
}

func (qs *QuestService) getNextReset(questType string) time.Time {
	now := time.Now()

	switch questType {
	case models.QuestTypeDaily:
		// Next day at midnight
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())

	case models.QuestTypeWeekly:
		// Next Monday at midnight
		days := (7 - int(now.Weekday()) + 1) % 7
		if days == 0 {
			days = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()+days, 0, 0, 0, 0, now.Location())

	case models.QuestTypeMonthly:
		// First day of next month at midnight
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())

	default:
		// Default to 24 hours
		return now.Add(24 * time.Hour)
	}
}

func (qs *QuestService) updateLeaderboard(ctx context.Context, userID string, quest *models.QuestDefinition) {
	// Calculate points based on tier
	points := quest.Tier * 100

	// Get current period
	periodStart := qs.getPeriodStart(quest.Type)

	// Get or create leaderboard entry
	entry, err := qs.questRepo.GetUserLeaderboardEntry(ctx, userID, quest.Type, periodStart)
	if err != nil || entry == nil {
		now := time.Now()
		entry = &models.QuestLeaderboard{
			PeriodType:      quest.Type,
			PeriodStart:     periodStart,
			UserID:          userID,
			QuestsCompleted: 0,
			PointsEarned:    0,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	}

	entry.QuestsCompleted++
	entry.PointsEarned += points

	if err := qs.questRepo.UpdateLeaderboard(ctx, entry); err != nil {
		slog.Error("Failed to update leaderboard",
			slog.String("user_id", userID),
			slog.String("quest_type", quest.Type),
			slog.Any("error", err))
	}
}

// trackWorkDay tracks unique days for work command quests
func (qs *QuestService) trackWorkDay(quest *models.UserQuestProgress, metadata map[string]interface{}) bool {
	// Defensive check
	if quest == nil {
		return false
	}

	// Initialize metadata if needed
	if quest.Metadata == nil {
		quest.Metadata = make(map[string]interface{})
	}

	// Get or create work days tracking
	workDaysData, exists := quest.Metadata["work_days"]
	workDays := make(map[string]bool)

	if exists {
		// Convert existing data
		if data, ok := workDaysData.(map[string]interface{}); ok {
			for k, v := range data {
				if b, ok := v.(bool); ok {
					workDays[k] = b
				}
			}
		}
	}

	// Add today's date
	today := time.Now().Format("2006-01-02")
	if !workDays[today] {
		workDays[today] = true
		quest.Metadata["work_days"] = workDays
		quest.CurrentProgress = len(workDays)
		return true
	}

	return false
}

// trackLevelUpDay tracks unique days for levelup command quests (like Level Addict)
func (qs *QuestService) trackLevelUpDay(quest *models.UserQuestProgress, metadata map[string]interface{}) bool {
	// Defensive check
	if quest == nil {
		return false
	}

	// Initialize metadata if needed
	if quest.Metadata == nil {
		quest.Metadata = make(map[string]interface{})
	}

	// Get or create levelup days tracking
	levelupDaysData, exists := quest.Metadata["levelup_days"]
	levelupDays := make(map[string]bool)

	if exists {
		// Convert existing data
		if data, ok := levelupDaysData.(map[string]interface{}); ok {
			for k, v := range data {
				if b, ok := v.(bool); ok {
					levelupDays[k] = b
				}
			}
		}
	}

	// Add today's date
	today := time.Now().Format("2006-01-02")
	if !levelupDays[today] {
		levelupDays[today] = true
		quest.Metadata["levelup_days"] = levelupDays
		quest.CurrentProgress = len(levelupDays)
		return true
	}

	return false
}

// trackUniqueCommand tracks unique commands for command count quests
func (qs *QuestService) trackUniqueCommand(quest *models.UserQuestProgress, metadata map[string]interface{}) bool {
	slog.Debug("trackUniqueCommand called",
		slog.String("quest_id", quest.QuestID),
		slog.Any("metadata", metadata),
		slog.Any("quest_metadata", quest.Metadata))

	// Defensive check
	if quest == nil {
		slog.Debug("trackUniqueCommand: quest is nil")
		return false
	}

	// Initialize metadata if needed
	if quest.Metadata == nil {
		quest.Metadata = make(map[string]interface{})
	}

	// Get command name from metadata
	commandName, ok := metadata["command"].(string)
	if !ok {
		return false
	}

	// Get or create commands tracking
	commandsData, exists := quest.Metadata["commands_used"]
	commandsUsed := make(map[string]bool)

	if exists {
		// Convert existing data
		if data, ok := commandsData.(map[string]interface{}); ok {
			for k, v := range data {
				if b, ok := v.(bool); ok {
					commandsUsed[k] = b
				}
			}
		}
	}

	// Add this command
	if !commandsUsed[commandName] {
		commandsUsed[commandName] = true
		quest.Metadata["commands_used"] = commandsUsed
		quest.CurrentProgress = len(commandsUsed)
		return true
	}

	return false
}

// trackComboProgress tracks multiple requirements for combo quests
func (qs *QuestService) trackComboProgress(quest *models.UserQuestProgress, action string, metadata map[string]interface{}) bool {
	// Defensive check
	if quest == nil || quest.QuestDefinition == nil {
		return false
	}

	// Initialize metadata if needed
	if quest.Metadata == nil {
		quest.Metadata = make(map[string]interface{})
	}

	// Get combo requirements from quest definition metadata
	// Format is like: {"claim": 8, "work": 3, "levelup": 10, "auction_create": 1}
	comboReqs := quest.QuestDefinition.RequirementMetadata
	if comboReqs == nil {
		return false
	}

	// Get or create combo tracking
	comboData, exists := quest.Metadata["combo_progress"]
	comboProgress := make(map[string]int)

	if exists {
		// Convert existing data
		if data, ok := comboData.(map[string]interface{}); ok {
			for k, v := range data {
				if count, ok := v.(float64); ok {
					comboProgress[k] = int(count)
				}
			}
		}
	}

	// Check if this action is part of the combo
	requiredCount := 0
	if reqCountInterface, ok := comboReqs[action]; ok {
		// Convert the required count
		switch v := reqCountInterface.(type) {
		case float64:
			requiredCount = int(v)
		case int:
			requiredCount = v
		default:
			return false
		}
	} else {
		// Action not part of this combo
		return false
	}

	// Update progress for this action
	current := comboProgress[action]
	if current < requiredCount {
		comboProgress[action]++
		quest.Metadata["combo_progress"] = comboProgress

		// Calculate total progress (sum of all completed requirements)
		totalCompleted := 0
		allRequirementsMet := true

		for reqAction, reqCountInterface := range comboReqs {
			reqCount := 0
			switch v := reqCountInterface.(type) {
			case float64:
				reqCount = int(v)
			case int:
				reqCount = v
			}

			currentCount := comboProgress[reqAction]
			if currentCount >= reqCount {
				totalCompleted++
			} else {
				allRequirementsMet = false
			}
		}

		// For combo quests, progress is the number of requirements completed
		quest.CurrentProgress = totalCompleted

		// If all requirements are met, mark the quest as complete
		if allRequirementsMet && quest.QuestDefinition.RequirementCount > 0 {
			quest.CurrentProgress = quest.QuestDefinition.RequirementCount
		}

		return true
	}

	return false
}

// updateCompletionQuests updates daily/weekly completion quests when other quests are completed
func (qs *QuestService) updateCompletionQuests(ctx context.Context, userID string, completedByType map[string]int) {
	// Get active quests again to check for completion quests
	activeQuests, err := qs.questRepo.GetActiveQuests(ctx, userID)
	if err != nil {
		return
	}

	for _, quest := range activeQuests {
		if quest.Completed || quest.QuestDefinition == nil {
			continue
		}

		switch quest.QuestDefinition.RequirementType {
		case models.RequirementTypeDailyComplete:
			if count, ok := completedByType[models.QuestTypeDaily]; ok && count > 0 {
				quest.CurrentProgress += count
				quest.CheckMilestones()

				if err := qs.questRepo.UpdateQuestProgress(ctx, quest); err != nil {
					slog.Error("Failed to update daily completion quest",
						slog.String("user_id", userID),
						slog.Any("error", err))
				}
			}

		case models.RequirementTypeWeeklyComplete:
			if count, ok := completedByType[models.QuestTypeWeekly]; ok && count > 0 {
				quest.CurrentProgress += count
				quest.CheckMilestones()

				if err := qs.questRepo.UpdateQuestProgress(ctx, quest); err != nil {
					slog.Error("Failed to update weekly completion quest",
						slog.String("user_id", userID),
						slog.Any("error", err))
				}
			}
		}
	}
}

func (qs *QuestService) getPeriodStart(periodType string) time.Time {
	now := time.Now()

	switch periodType {
	case models.QuestTypeDaily:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	case models.QuestTypeWeekly:
		// Get start of week (Monday)
		days := int(now.Weekday()) - 1
		if days < 0 {
			days = 6
		}
		return time.Date(now.Year(), now.Month(), now.Day()-days, 0, 0, 0, 0, now.Location())

	case models.QuestTypeMonthly:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	default:
		return now
	}
}

// Result types

type QuestRewardResult struct {
	Success         bool
	Message         string
	ClaimedQuests   []ClaimedQuest
	TotalSnowflakes int64
	TotalVials      int
	TotalXP         int
	DailyCount      int
	WeeklyCount     int
	MonthlyCount    int
}

type ClaimedQuest struct {
	QuestName        string
	Type             string
	Tier             int
	RewardSnowflakes int64
	RewardVials      int
	RewardXP         int
}

type UserQuestStatus struct {
	DailyQuests   []*models.UserQuestProgress
	WeeklyQuests  []*models.UserQuestProgress
	MonthlyQuests []*models.UserQuestProgress
}

// QuestRouletteResult represents the result of spinning the quest roulette
type QuestRouletteResult struct {
	Quest      *models.QuestDefinition
	Multiplier float64
	IsRare     bool
}
