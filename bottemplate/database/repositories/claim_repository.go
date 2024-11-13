package repositories

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/uptrace/bun"
)

var (
	ErrClaimLimitExceeded = errors.New("claim limit exceeded")
	ErrClaimExists        = errors.New("claim already exists")
	ErrInvalidClaim       = errors.New("invalid claim")
)

type ClaimRepository interface {
	CreateClaim(ctx context.Context, cardID int64, userID string) (*models.Claim, error)
	GetUserClaims(ctx context.Context, userID string) ([]*models.Claim, error)
	DeleteExpiredClaims(ctx context.Context) (int64, error)
	GetUserClaimsInPeriod(ctx context.Context, userID string, since time.Time) (int, error)
	CreateClaimTx(ctx context.Context, tx bun.Tx, cardID int64, userID string) (*models.Claim, error)
	ValidateClaimEligibility(ctx context.Context, userID string) error
	StartCleanupRoutine(ctx context.Context)
	UpdateClaimStats(ctx context.Context, tx bun.Tx, userID string, successful bool) error
	GetClaimStats(ctx context.Context, userID string) (*models.ClaimStats, error)
}

type claimRepository struct {
	db        *bun.DB
	cache     sync.Map
	maxClaims int
	claimTTL  time.Duration
}

type claimCacheEntry struct {
	claims    []*models.Claim
	expiresAt time.Time
}

func NewClaimRepository(db *bun.DB) ClaimRepository {
	return &claimRepository{
		db:        db,
		maxClaims: 3,
		claimTTL:  24 * time.Hour,
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
	count, err := r.GetUserClaimsInPeriod(ctx, userID, time.Now().Add(-r.claimTTL))
	if err != nil {
		return err
	}

	if count >= r.maxClaims {
		// return ErrClaimLimitExceeded
	}

	return nil
}

func (r *claimRepository) GetUserClaimsInPeriod(ctx context.Context, userID string, since time.Time) (int, error) {
	count, err := r.db.NewSelect().
		Model((*models.Claim)(nil)).
		Where("user_id = ? AND claimed_at > ?", userID, since).
		Count(ctx)

	return count, err
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

func (r *claimRepository) UpdateClaimStats(ctx context.Context, tx bun.Tx, userID string, successful bool) error {
	stats := &models.ClaimStats{
		UserID:      userID,
		UpdatedAt:   time.Now(),
		LastClaimAt: time.Now(),
	}

	_, err := tx.NewInsert().
		Model(stats).
		On("CONFLICT (user_id) DO UPDATE").
		Set("total_claims = claim_stats.total_claims + 1").
		Set("successful_claims = claim_stats.successful_claims + CASE WHEN ? THEN 1 ELSE 0 END", successful).
		Set("daily_claims = CASE WHEN DATE(last_claim_at) = CURRENT_DATE THEN daily_claims + 1 ELSE 1 END").
		Set("weekly_claims = CASE WHEN DATE(last_claim_at) >= DATE(CURRENT_DATE - INTERVAL '7 days') THEN weekly_claims + 1 ELSE 1 END").
		Set("last_claim_at = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Exec(ctx)

	return err
}

func (r *claimRepository) GetClaimStats(ctx context.Context, userID string) (*models.ClaimStats, error) {
	stats := new(models.ClaimStats)
	err := r.db.NewSelect().
		Model(stats).
		Where("user_id = ?", userID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}
	return stats, nil
}
