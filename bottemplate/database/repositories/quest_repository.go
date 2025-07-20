package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

type QuestRepository interface {
	// Quest definitions
	GetQuestDefinition(ctx context.Context, questID string) (*models.QuestDefinition, error)
	GetQuestsByType(ctx context.Context, questType string) ([]*models.QuestDefinition, error)
	GetRandomQuestsByTier(ctx context.Context, questType string, tier int, count int) ([]*models.QuestDefinition, error)
	GetAllQuestDefinitions(ctx context.Context) ([]*models.QuestDefinition, error)
	CreateQuestDefinition(ctx context.Context, quest *models.QuestDefinition) error

	// User progress
	GetActiveQuests(ctx context.Context, userID string) ([]*models.UserQuestProgress, error)
	GetQuestProgress(ctx context.Context, userID string, questID string) (*models.UserQuestProgress, error)
	CreateQuestProgress(ctx context.Context, progress *models.UserQuestProgress) error
	UpdateQuestProgress(ctx context.Context, progress *models.UserQuestProgress) error
	GetUnclaimedQuests(ctx context.Context, userID string) ([]*models.UserQuestProgress, error)
	GetCompletedQuestCount(ctx context.Context, userID string, questType string, since time.Time) (int, error)
	DeleteExpiredQuests(ctx context.Context) error

	// Leaderboards
	GetLeaderboard(ctx context.Context, periodType string, periodStart time.Time, limit int) ([]*models.QuestLeaderboard, error)
	UpdateLeaderboard(ctx context.Context, entry *models.QuestLeaderboard) error
	GetUserLeaderboardEntry(ctx context.Context, userID string, periodType string, periodStart time.Time) (*models.QuestLeaderboard, error)

	// Quest chains
	GetQuestChain(ctx context.Context, chainID string) (*models.QuestChain, error)
	GetAllQuestChains(ctx context.Context) ([]*models.QuestChain, error)
}

type questRepository struct {
	db *bun.DB
}

func NewQuestRepository(db *bun.DB) QuestRepository {
	return &questRepository{db: db}
}

// Quest definitions
func (r *questRepository) GetQuestDefinition(ctx context.Context, questID string) (*models.QuestDefinition, error) {
	quest := new(models.QuestDefinition)
	err := r.db.NewSelect().
		Model(quest).
		Where("quest_id = ?", questID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("quest not found: %s", questID)
		}
		return nil, err
	}

	return quest, nil
}

func (r *questRepository) GetQuestsByType(ctx context.Context, questType string) ([]*models.QuestDefinition, error) {
	var quests []*models.QuestDefinition
	err := r.db.NewSelect().
		Model(&quests).
		Where("type = ?", questType).
		Order("tier ASC", "quest_id ASC").
		Scan(ctx)

	return quests, err
}

func (r *questRepository) GetRandomQuestsByTier(ctx context.Context, questType string, tier int, count int) ([]*models.QuestDefinition, error) {
	var quests []*models.QuestDefinition
	err := r.db.NewSelect().
		Model(&quests).
		Where("type = ? AND tier = ?", questType, tier).
		OrderExpr("RANDOM()").
		Limit(count).
		Scan(ctx)

	return quests, err
}

func (r *questRepository) GetAllQuestDefinitions(ctx context.Context) ([]*models.QuestDefinition, error) {
	var quests []*models.QuestDefinition
	err := r.db.NewSelect().
		Model(&quests).
		Order("type ASC", "tier ASC", "quest_id ASC").
		Scan(ctx)

	return quests, err
}

func (r *questRepository) CreateQuestDefinition(ctx context.Context, quest *models.QuestDefinition) error {
	quest.CreatedAt = time.Now()
	quest.UpdatedAt = time.Now()
	_, err := r.db.NewInsert().Model(quest).Exec(ctx)
	return err
}

// User progress
func (r *questRepository) GetActiveQuests(ctx context.Context, userID string) ([]*models.UserQuestProgress, error) {
	var progress []*models.UserQuestProgress
	err := r.db.NewSelect().
		Model(&progress).
		Relation("QuestDefinition").
		Where("uqp.user_id = ?", userID).
		Where("uqp.expires_at > ?", time.Now()).
		// Removed the claimed = false filter to show completed quests too
		Order("uqp.quest_id ASC").
		Scan(ctx)

	if err != nil {
		slog.Error("Failed to get active quests",
			slog.String("user_id", userID),
			slog.Any("error", err))
		return nil, err
	}

	return progress, nil
}

func (r *questRepository) GetQuestProgress(ctx context.Context, userID string, questID string) (*models.UserQuestProgress, error) {
	progress := new(models.UserQuestProgress)
	err := r.db.NewSelect().
		Model(progress).
		Relation("QuestDefinition").
		Where("user_id = ? AND quest_id = ?", userID, questID).
		Where("expires_at > ?", time.Now()).
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return progress, nil
}

func (r *questRepository) CreateQuestProgress(ctx context.Context, progress *models.UserQuestProgress) error {
	progress.CreatedAt = time.Now()
	progress.UpdatedAt = time.Now()
	progress.StartedAt = time.Now()

	_, err := r.db.NewInsert().Model(progress).Exec(ctx)
	return err
}

func (r *questRepository) UpdateQuestProgress(ctx context.Context, progress *models.UserQuestProgress) error {
	progress.UpdatedAt = time.Now()

	// Check if quest is completed
	if progress.QuestDefinition != nil && progress.CurrentProgress >= progress.QuestDefinition.RequirementCount {
		progress.Completed = true
		now := time.Now()
		progress.CompletedAt = &now
	}

	_, err := r.db.NewUpdate().
		Model(progress).
		WherePK().
		Exec(ctx)

	return err
}

func (r *questRepository) GetUnclaimedQuests(ctx context.Context, userID string) ([]*models.UserQuestProgress, error) {
	var progress []*models.UserQuestProgress
	err := r.db.NewSelect().
		Model(&progress).
		Relation("QuestDefinition").
		Where("uqp.user_id = ?", userID).
		Where("uqp.completed = ?", true).
		Where("uqp.claimed = ?", false).
		Where("uqp.expires_at > ?", time.Now()).
		Order("uqp.completed_at ASC").
		Scan(ctx)

	return progress, err
}

func (r *questRepository) GetCompletedQuestCount(ctx context.Context, userID string, questType string, since time.Time) (int, error) {
	count, err := r.db.NewSelect().
		Model((*models.UserQuestProgress)(nil)).
		Join("JOIN quest_definitions qd ON qd.quest_id = user_quest_progress.quest_id").
		Where("user_quest_progress.user_id = ?", userID).
		Where("user_quest_progress.completed = ?", true).
		Where("user_quest_progress.completed_at >= ?", since).
		Where("qd.type = ?", questType).
		Count(ctx)

	return count, err
}

func (r *questRepository) DeleteExpiredQuests(ctx context.Context) error {
	_, err := r.db.NewDelete().
		Model((*models.UserQuestProgress)(nil)).
		Where("expires_at < ?", time.Now()).
		Where("claimed = ?", false).
		Exec(ctx)

	return err
}

// Leaderboards
func (r *questRepository) GetLeaderboard(ctx context.Context, periodType string, periodStart time.Time, limit int) ([]*models.QuestLeaderboard, error) {
	var entries []*models.QuestLeaderboard
	err := r.db.NewSelect().
		Model(&entries).
		Where("period_type = ?", periodType).
		Where("period_start = ?", periodStart).
		Order("points_earned DESC", "quests_completed DESC").
		Limit(limit).
		Scan(ctx)

	return entries, err
}

func (r *questRepository) UpdateLeaderboard(ctx context.Context, entry *models.QuestLeaderboard) error {
	entry.UpdatedAt = time.Now()

	// Upsert logic
	_, err := r.db.NewInsert().
		Model(entry).
		On("CONFLICT (period_type, period_start, user_id) DO UPDATE").
		Set("quests_completed = EXCLUDED.quests_completed").
		Set("points_earned = EXCLUDED.points_earned").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)

	return err
}

func (r *questRepository) GetUserLeaderboardEntry(ctx context.Context, userID string, periodType string, periodStart time.Time) (*models.QuestLeaderboard, error) {
	entry := new(models.QuestLeaderboard)
	err := r.db.NewSelect().
		Model(entry).
		Where("user_id = ?", userID).
		Where("period_type = ?", periodType).
		Where("period_start = ?", periodStart).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return entry, nil
}

// Quest chains
func (r *questRepository) GetQuestChain(ctx context.Context, chainID string) (*models.QuestChain, error) {
	chain := new(models.QuestChain)
	err := r.db.NewSelect().
		Model(chain).
		Where("chain_id = ?", chainID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("quest chain not found: %s", chainID)
		}
		return nil, err
	}

	return chain, nil
}

func (r *questRepository) GetAllQuestChains(ctx context.Context) ([]*models.QuestChain, error) {
	var chains []*models.QuestChain
	err := r.db.NewSelect().
		Model(&chains).
		Order("chain_id ASC").
		Scan(ctx)

	return chains, err
}
