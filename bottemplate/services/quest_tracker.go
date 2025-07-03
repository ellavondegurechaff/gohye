package services

import (
	"context"
	"log/slog"
)

// QuestTracker provides a simple interface for tracking quest progress
type QuestTracker struct {
	questService *QuestService
}

func NewQuestTracker(questService *QuestService) *QuestTracker {
	return &QuestTracker{
		questService: questService,
	}
}

// TrackCommand tracks a command execution for quest progress
func (qt *QuestTracker) TrackCommand(ctx context.Context, userID string, commandName string) {
	slog.Debug("TrackCommand called",
		slog.String("user_id", userID),
		slog.String("command", commandName))
		
	if qt.questService == nil {
		slog.Error("Quest service is nil in TrackCommand")
		return
	}

	// Track for command count quests
	metadata := map[string]interface{}{
		"command": commandName,
	}

	// First update for RequirementTypeCommandCount
	if err := qt.questService.UpdateProgress(ctx, userID, "command_count", metadata); err != nil {
		slog.Debug("Failed to track quest progress for command count",
			slog.String("user_id", userID),
			slog.String("command", commandName),
			slog.Any("error", err))
	}

	// Also update for specific command quests
	if err := qt.questService.UpdateProgress(ctx, userID, commandName, metadata); err != nil {
		slog.Debug("Failed to track quest progress for specific command",
			slog.String("user_id", userID),
			slog.String("command", commandName),
			slog.Any("error", err))
	}
}

// TrackCardClaim tracks card claiming for quest progress
func (qt *QuestTracker) TrackCardClaim(ctx context.Context, userID string, cardCount int) {
	if qt.questService == nil {
		return
	}

	// Track each claim individually
	for i := 0; i < cardCount; i++ {
		metadata := map[string]interface{}{
			"action": "claim",
		}

		if err := qt.questService.UpdateProgress(ctx, userID, "claim", metadata); err != nil {
			slog.Debug("Failed to track quest progress for claim",
				slog.String("user_id", userID),
				slog.Any("error", err))
			break
		}
	}
}

// TrackCardLevelUp tracks card level up for quest progress
func (qt *QuestTracker) TrackCardLevelUp(ctx context.Context, userID string, count int) {
	if qt.questService == nil {
		return
	}

	for i := 0; i < count; i++ {
		metadata := map[string]interface{}{
			"action": "levelup",
		}

		if err := qt.questService.UpdateProgress(ctx, userID, "levelup", metadata); err != nil {
			slog.Debug("Failed to track quest progress for levelup",
				slog.String("user_id", userID),
				slog.Any("error", err))
			break
		}
	}
}

// TrackCardLevelUpWithMetadata tracks card level up with additional metadata
func (qt *QuestTracker) TrackCardLevelUpWithMetadata(ctx context.Context, userID string, count int, metadata map[string]interface{}) {
	if qt.questService == nil {
		return
	}

	// Add action to metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["action"] = "levelup"

	for i := 0; i < count; i++ {
		if err := qt.questService.UpdateProgress(ctx, userID, "levelup", metadata); err != nil {
			slog.Debug("Failed to track quest progress for levelup",
				slog.String("user_id", userID),
				slog.Any("error", err))
			break
		}
	}
}

// TrackCardDraw tracks card drawing for quest progress
func (qt *QuestTracker) TrackCardDraw(ctx context.Context, userID string) {
	if qt.questService == nil {
		return
	}

	metadata := map[string]interface{}{
		"action": "draw",
	}

	if err := qt.questService.UpdateProgress(ctx, userID, "draw", metadata); err != nil {
		slog.Debug("Failed to track quest progress for draw",
			slog.String("user_id", userID),
			slog.Any("error", err))
	}
}

// TrackWork tracks work command for quest progress
func (qt *QuestTracker) TrackWork(ctx context.Context, userID string) {
	if qt.questService == nil {
		return
	}

	metadata := map[string]interface{}{
		"action": "work",
	}

	if err := qt.questService.UpdateProgress(ctx, userID, "work", metadata); err != nil {
		slog.Debug("Failed to track quest progress for work",
			slog.String("user_id", userID),
			slog.Any("error", err))
	}
}

// TrackAuctionBid tracks auction bidding for quest progress
func (qt *QuestTracker) TrackAuctionBid(ctx context.Context, userID string) {
	if qt.questService == nil {
		return
	}

	metadata := map[string]interface{}{
		"action": "auction_bid",
	}

	if err := qt.questService.UpdateProgress(ctx, userID, "auction_bid", metadata); err != nil {
		slog.Debug("Failed to track quest progress for auction bid",
			slog.String("user_id", userID),
			slog.Any("error", err))
	}
}

// TrackAuctionWin tracks auction winning for quest progress
func (qt *QuestTracker) TrackAuctionWin(ctx context.Context, userID string) {
	if qt.questService == nil {
		return
	}

	metadata := map[string]interface{}{
		"action": "auction_win",
	}

	if err := qt.questService.UpdateProgress(ctx, userID, "auction_win", metadata); err != nil {
		slog.Debug("Failed to track quest progress for auction win",
			slog.String("user_id", userID),
			slog.Any("error", err))
	}
}

// TrackAuctionCreate tracks auction creation for quest progress
func (qt *QuestTracker) TrackAuctionCreate(ctx context.Context, userID string) {
	if qt.questService == nil {
		return
	}

	metadata := map[string]interface{}{
		"action": "auction_create",
	}

	if err := qt.questService.UpdateProgress(ctx, userID, "auction_create", metadata); err != nil {
		slog.Debug("Failed to track quest progress for auction create",
			slog.String("user_id", userID),
			slog.Any("error", err))
	}
}

// TrackTrade tracks card trading for quest progress
func (qt *QuestTracker) TrackTrade(ctx context.Context, userID string) {
	if qt.questService == nil {
		return
	}

	metadata := map[string]interface{}{
		"action": "trade",
	}

	if err := qt.questService.UpdateProgress(ctx, userID, "trade", metadata); err != nil {
		slog.Debug("Failed to track quest progress for trade",
			slog.String("user_id", userID),
			slog.Any("error", err))
	}
}

// TrackSnowflakesEarned tracks snowflakes earned for quest progress
func (qt *QuestTracker) TrackSnowflakesEarned(ctx context.Context, userID string, amount int64, source string) {
	if qt.questService == nil || amount <= 0 {
		return
	}

	metadata := map[string]interface{}{
		"snowflakes_earned": amount,
	}
	
	// Add source if provided
	if source != "" {
		metadata["source"] = source
	}

	if err := qt.questService.UpdateProgress(ctx, userID, "snowflakes_earned", metadata); err != nil {
		slog.Debug("Failed to track quest progress for snowflakes earned",
			slog.String("user_id", userID),
			slog.Int64("amount", amount),
			slog.Any("error", err))
	}
}