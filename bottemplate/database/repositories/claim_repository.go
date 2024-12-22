package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/uptrace/bun"
)

var (
	ErrClaimExists  = errors.New("claim already exists")
	ErrInvalidClaim = errors.New("invalid claim")
)

type ClaimRepository interface {
	CreateClaim(ctx context.Context, cardID int64, userID string) (*models.Claim, error)
	GetUserClaims(ctx context.Context, userID string) ([]*models.Claim, error)
	DeleteExpiredClaims(ctx context.Context) (int64, error)
	GetUserClaimsInPeriod(ctx context.Context, userID string, since time.Time) (int, error)
	CreateClaimTx(ctx context.Context, tx bun.Tx, cardID int64, userID string) (*models.Claim, error)
	ValidateClaimEligibility(ctx context.Context, userID string) error
	StartCleanupRoutine(ctx context.Context)
	UpdateClaimStats(ctx context.Context, tx bun.Tx, userID string, claimCost int64, successful bool) error
	GetClaimStats(ctx context.Context, userID string) (*models.ClaimStats, error)
	GetClaimCost(ctx context.Context, userID string) (int64, error)
	GetClaimInfo(ctx context.Context, userID string) (*ClaimInfo, error)
	GetBasePrice() int64
}

type claimRepository struct {
	db        *bun.DB
	cache     sync.Map
	claimTTL  time.Duration
	basePrice int64
}

type claimCacheEntry struct {
	claims    []*models.Claim
	expiresAt time.Time
}

type ClaimInfo struct {
	TotalSpent      int64
	Balance         int64
	RemainingClaims int
	NextClaimCost   int64
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func NewClaimRepository(db *bun.DB) ClaimRepository {
	return &claimRepository{
		db:        db,
		claimTTL:  24 * time.Hour,
		basePrice: 700,
	}
}

func (r *claimRepository) CreateClaim(ctx context.Context, cardID int64, userID string) (*models.Claim, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	claim, err := r.CreateClaimTx(ctx, tx, cardID, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return claim, nil
}

func (r *claimRepository) CreateClaimTx(ctx context.Context, tx bun.Tx, cardID int64, userID string) (*models.Claim, error) {
	if err := r.ValidateClaimEligibility(ctx, userID); err != nil {
		return nil, err
	}

	// First try to update existing UserCard
	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	// If no rows were affected, create new UserCard
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		// Create new UserCard entry for first-time acquisition
		userCard := &models.UserCard{
			UserID:    userID,
			CardID:    cardID,
			Amount:    1,
			Obtained:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err = tx.NewInsert().Model(userCard).Exec(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Create claim record for cooldown tracking
	claim := &models.Claim{
		CardID:    cardID,
		UserID:    userID,
		ClaimedAt: time.Now(),
		Expires:   time.Now().Add(r.claimTTL),
	}

	_, err = tx.NewInsert().Model(claim).Exec(ctx)
	if err != nil {
		return nil, err
	}

	r.invalidateCache(userID)
	return claim, nil
}

func (r *claimRepository) ValidateClaimEligibility(ctx context.Context, userID string) error {
	// Only check if the user exists and has enough balance
	return nil
}

func (r *claimRepository) GetUserClaimsInPeriod(ctx context.Context, userID string, since time.Time) (int, error) {
	count, err := r.db.NewSelect().
		Model((*models.Claim)(nil)).
		Where("user_id = ?", userID).
		Where("claimed_at > ?", since).
		Where("claimed_at <= ?", time.Now()).
		Count(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to count claims: %w", err)
	}

	return count, nil
}

func (r *claimRepository) GetUserClaims(ctx context.Context, userID string) ([]*models.Claim, error) {
	if cached, ok := r.getCached(userID); ok {
		return cached, nil
	}

	var claims []*models.Claim
	err := r.db.NewSelect().
		Model(&claims).
		Where("user_id = ?", userID).
		Where("expires > ?", time.Now()).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	r.setCached(userID, claims)
	return claims, nil
}

func (r *claimRepository) DeleteExpiredClaims(ctx context.Context) (int64, error) {
	result, err := r.db.NewDelete().
		Model((*models.Claim)(nil)).
		Where("expires <= ?", time.Now()).
		Exec(ctx)

	if err != nil {
		return 0, err
	}

	r.cache.Range(func(key, _ interface{}) bool {
		r.cache.Delete(key)
		return true
	})

	return result.RowsAffected()
}

func (r *claimRepository) getCached(userID string) ([]*models.Claim, bool) {
	if entry, ok := r.cache.Load(userID); ok {
		cached := entry.(claimCacheEntry)
		if time.Now().Before(cached.expiresAt) {
			return cached.claims, true
		}
		r.cache.Delete(userID)
	}
	return nil, false
}

func (r *claimRepository) setCached(userID string, claims []*models.Claim) {
	r.cache.Store(userID, claimCacheEntry{
		claims:    claims,
		expiresAt: time.Now().Add(utils.CacheExpiration),
	})
}

func (r *claimRepository) invalidateCache(userID string) {
	r.cache.Delete(userID)
}

func (r *claimRepository) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := r.DeleteExpiredClaims(ctx); err != nil {
					log.Printf("[ERROR] Failed to cleanup expired claims: %v", err)
				}
			}
		}
	}()
}

func (r *claimRepository) UpdateClaimStats(ctx context.Context, tx bun.Tx, userID string, claimCost int64, successful bool) error {
	// Verify the claim cost is correct
	count, err := r.GetUserClaimsInPeriod(ctx, userID, time.Now().Add(-r.claimTTL))
	if err != nil {
		return fmt.Errorf("failed to get user claims: %w", err)
	}

	expectedCost := r.calculateClaimCost(count)
	if claimCost != expectedCost {
		claimCost = expectedCost
	}

	// First try to update existing stats
	result, err := tx.NewUpdate().
		Model((*models.ClaimStats)(nil)).
		Set("total_spent = COALESCE(total_spent, 0) + ?", claimCost).
		Set("total_claims = COALESCE(total_claims, 0) + 1").
		Set("successful_claims = COALESCE(successful_claims, 0) + ?", boolToInt(successful)).
		Set("daily_claims = CASE WHEN DATE(last_claim_at) = CURRENT_DATE THEN COALESCE(daily_claims, 0) + 1 ELSE 1 END").
		Set("weekly_claims = CASE WHEN DATE(last_claim_at) >= DATE(CURRENT_DATE - INTERVAL '7 days') THEN COALESCE(weekly_claims, 0) + 1 ELSE 1 END").
		Set("last_claim_at = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Where("user_id = ?", userID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update stats: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		stats := &models.ClaimStats{
			UserID:           userID,
			TotalSpent:       claimCost,
			TotalClaims:      1,
			SuccessfulClaims: boolToInt(successful),
			LastClaimAt:      time.Now(),
			DailyClaims:      1,
			WeeklyClaims:     1,
			UpdatedAt:        time.Now(),
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
			UserID:       userID,
			TotalSpent:   0,
			TotalClaims:  0,
			DailyClaims:  0,
			WeeklyClaims: 0,
			LastClaimAt:  time.Now(),
			UpdatedAt:    time.Now(),
		}, nil
	}

	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *claimRepository) GetClaimCost(ctx context.Context, userID string) (int64, error) {
	count, err := r.GetUserClaimsInPeriod(ctx, userID, time.Now().Add(-r.claimTTL))
	if err != nil {
		return 0, err
	}

	return r.calculateClaimCost(count), nil
}

func (r *claimRepository) GetClaimInfo(ctx context.Context, userID string) (*ClaimInfo, error) {
	var user models.User
	err := r.db.NewSelect().
		Model(&user).
		Where("discord_id = ?", userID).
		Scan(ctx)

	if errors.Is(err, sql.ErrNoRows) {
		user.Balance = 500
		user.DiscordID = userID
	} else if err != nil {
		return nil, fmt.Errorf("failed to get user balance: %w", err)
	}

	var stats models.ClaimStats
	err = r.db.NewSelect().
		Model(&stats).
		Where("user_id = ?", userID).
		Scan(ctx)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get claim stats: %w", err)
	}

	count, err := r.GetUserClaimsInPeriod(ctx, userID, time.Now().Add(-r.claimTTL))
	if err != nil {
		return nil, fmt.Errorf("failed to get user claims: %w", err)
	}

	nextClaimCost := r.calculateClaimCost(count)

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

func (r *claimRepository) calculateClaimCost(claimCount int) int64 {
	return r.basePrice * (int64(claimCount) + 1)
}

func (r *claimRepository) GetBasePrice() int64 {
	return r.basePrice
}
