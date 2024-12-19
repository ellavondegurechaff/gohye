package repositories

import (
	"context"
	"time"

	"github.com/disgoorg/bot-template/internal/gateways/database/models"
	"github.com/uptrace/bun"
)

type WishlistRepository interface {
	Add(ctx context.Context, userID string, cardID int64) error
	Remove(ctx context.Context, userID string, cardID int64) error
	GetByUserID(ctx context.Context, userID string) ([]*models.Wishlist, error)
	RemoveMany(ctx context.Context, userID string, cardIDs []int64) error
	AddMany(ctx context.Context, userID string, cardIDs []int64) error
	Exists(ctx context.Context, userID string, cardID int64) (bool, error)
}

type wishlistRepository struct {
	db *bun.DB
}

func NewWishlistRepository(db *bun.DB) WishlistRepository {
	return &wishlistRepository{db: db}
}

func (r *wishlistRepository) Add(ctx context.Context, userID string, cardID int64) error {
	wishlist := &models.Wishlist{
		UserID:    userID,
		CardID:    cardID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := r.db.NewInsert().Model(wishlist).Exec(ctx)
	return err
}

func (r *wishlistRepository) Remove(ctx context.Context, userID string, cardID int64) error {
	_, err := r.db.NewDelete().
		Model((*models.Wishlist)(nil)).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		Exec(ctx)
	return err
}

func (r *wishlistRepository) GetByUserID(ctx context.Context, userID string) ([]*models.Wishlist, error) {
	var wishlists []*models.Wishlist
	err := r.db.NewSelect().
		Model(&wishlists).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Scan(ctx)
	return wishlists, err
}

func (r *wishlistRepository) RemoveMany(ctx context.Context, userID string, cardIDs []int64) error {
	_, err := r.db.NewDelete().
		Model((*models.Wishlist)(nil)).
		Where("user_id = ? AND card_id IN (?)", userID, bun.In(cardIDs)).
		Exec(ctx)
	return err
}

func (r *wishlistRepository) AddMany(ctx context.Context, userID string, cardIDs []int64) error {
	wishlists := make([]*models.Wishlist, len(cardIDs))
	now := time.Now()
	for i, cardID := range cardIDs {
		wishlists[i] = &models.Wishlist{
			UserID:    userID,
			CardID:    cardID,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	_, err := r.db.NewInsert().Model(&wishlists).Exec(ctx)
	return err
}

func (r *wishlistRepository) Exists(ctx context.Context, userID string, cardID int64) (bool, error) {
	exists, err := r.db.NewSelect().
		Model((*models.Wishlist)(nil)).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		Exists(ctx)
	return exists, err
}
