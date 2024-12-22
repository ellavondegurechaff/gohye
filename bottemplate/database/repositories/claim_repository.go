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

var (
	ErrClaimExists  = errors.New("claim already exists")
	ErrInvalidClaim = errors.New("invalid claim")
)

type ClaimRepository interface {
	GetUserClaimsInPeriod(ctx context.Context, userID string, since time.Time) (int, error)
	UpdateClaimStats(ctx context.Context, tx bun.Tx, userID string, claimCost int64, successful bool, totalClaims int) error
	GetClaimStats(ctx context.Context, userID string) (*models.ClaimStats, error)
	GetClaimCost(ctx context.Context, userID string) (int64, error)
	GetClaimInfo(ctx context.Context, userID string) (*ClaimInfo, error)
	GetBasePrice() int64
	ResetDailyClaims(ctx context.Context, tx bun.Tx, userID string) error
}

type claimRepository struct {
	db        *bun.DB
	basePrice int64
}

type ClaimInfo struct {
	TotalSpent      int64
	Balance         int64
	RemainingClaims int
	NextClaimCost   int64
}

func NewClaimRepository(db *bun.DB) ClaimRepository {
	return &claimRepository{
		db:        db,
		basePrice: 700,
	}
}

func (r *claimRepository) GetUserClaimsInPeriod(ctx context.Context, userID string, since time.Time) (int, error) {
	var stats models.ClaimStats
	err := r.db.NewSelect().
		Model(&stats).
		Where("user_id = ?", userID).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	var user models.User
	err = r.db.NewSelect().
		Model(&user).
		Column("last_daily").
		Where("discord_id = ?", userID).
		Scan(ctx)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	return stats.DailyClaims, nil
}

func (r *claimRepository) UpdateClaimStats(ctx context.Context, tx bun.Tx, userID string, claimCost int64, successful bool, totalClaims int) error {
	now := time.Now()

	// First try to update existing record
	result, err := tx.NewUpdate().
		Model(&models.ClaimStats{}).
		Set("total_spent = COALESCE(total_spent, 0) + ?", claimCost).
		Set("total_claims = COALESCE(total_claims, 0) + ?", totalClaims).
		Set("daily_claims = CASE "+
			"WHEN DATE(last_claim_date) = CURRENT_DATE THEN daily_claims + ? "+
			"ELSE ? END", totalClaims, totalClaims).
		Set("last_claim_date = ?", now).
		Set("last_claim_at = ?", now).
		Set("updated_at = ?", now).
		Where("user_id = ?", userID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update stats: %w", err)
	}

	// Check if we need to insert instead (no existing record)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If no rows were affected, insert new record
	if rowsAffected == 0 {
		stats := &models.ClaimStats{
			UserID:        userID,
			TotalSpent:    claimCost,
			TotalClaims:   totalClaims,
			DailyClaims:   totalClaims,
			LastClaimDate: now,
			LastClaimAt:   now,
			UpdatedAt:     now,
		}

		_, err = tx.NewInsert().Model(stats).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create stats: %w", err)
		}
	}

	return nil
}

func (r *claimRepository) GetClaimStats(ctx context.Context, userID string) (*models.ClaimStats, error) {
	stats := new(models.ClaimStats)
	err := r.db.NewSelect().
		Model(stats).
		Where("user_id = ?", userID).
		Scan(ctx)

	// If no stats exist yet, return empty stats instead of error
	if errors.Is(err, sql.ErrNoRows) {
		return &models.ClaimStats{
			UserID:      userID,
			TotalSpent:  0,
			TotalClaims: 0,
			DailyClaims: 0,
			LastClaimAt: time.Now(),
			UpdatedAt:   time.Now(),
		}, nil
	}

	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *claimRepository) GetClaimCost(ctx context.Context, userID string) (int64, error) {
	claimCount, err := r.GetUserClaimsInPeriod(ctx, userID, time.Now().Add(-24*time.Hour))
	if err != nil {
		return 0, fmt.Errorf("failed to get claim count: %w", err)
	}

	// Base price multiplied by number of claims today + 1
	return r.basePrice * (int64(claimCount) + 1), nil
}

func (r *claimRepository) GetClaimInfo(ctx context.Context, userID string) (*ClaimInfo, error) {
	var user models.User
	err := r.db.NewSelect().
		Model(&user).
		Where("discord_id = ?", userID).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		// New user gets starting balance
		user = models.User{
			DiscordID: userID,
			Balance:   500, // Starting balance
		}
		// Create user record
		_, err = r.db.NewInsert().Model(&user).Exec(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create new user: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to get user balance: %w", err)
	}

	stats, err := r.GetClaimStats(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get claim stats: %w", err)
	}

	// Get daily claims count with logging
	dailyClaims := 0
	if stats != nil && time.Now().Format("2006-01-02") == stats.LastClaimDate.Format("2006-01-02") {
		dailyClaims = stats.DailyClaims
	}

	slog.Info("Claim info calculation",
		"user_id", userID,
		"daily_claims", dailyClaims,
		"balance", user.Balance)

	// Calculate next claim cost based on daily claims
	nextClaimCost := r.basePrice * (int64(dailyClaims) + 1)

	slog.Info("Next claim cost calculation",
		"base_price", r.basePrice,
		"daily_claims", dailyClaims,
		"next_claim_cost", nextClaimCost)

	possibleClaims := user.Balance / nextClaimCost
	if possibleClaims < 0 {
		possibleClaims = 0
	}

	return &ClaimInfo{
		TotalSpent:      stats.TotalSpent,
		Balance:         user.Balance,
		RemainingClaims: int(possibleClaims),
		NextClaimCost:   nextClaimCost,
	}, nil
}

func (r *claimRepository) calculateClaimCost() int64 {
	return r.basePrice
}

func (r *claimRepository) GetBasePrice() int64 {
	return r.basePrice
}

func (r *claimRepository) ResetDailyClaims(ctx context.Context, tx bun.Tx, userID string) error {
	_, err := tx.NewUpdate().
		Model(&models.ClaimStats{}).
		Set("daily_claims = 0").
		Where("user_id = ?", userID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to reset daily claims: %w", err)
	}
	return nil
}
