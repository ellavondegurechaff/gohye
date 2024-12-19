package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/disgoorg/bot-template/internal/gateways/database/models"
	"github.com/uptrace/bun"
)

type UserCardRepository interface {
	Create(ctx context.Context, userCard *models.UserCard) error
	GetByID(ctx context.Context, id int64) (*models.UserCard, error)
	GetByUserIDAndCardID(ctx context.Context, userID string, cardID int64) (*models.UserCard, error)
	GetAllByUserID(ctx context.Context, userID string) ([]*models.UserCard, error)
	Update(ctx context.Context, userCard *models.UserCard) error
	Delete(ctx context.Context, id int64) error
	UpdateAmount(ctx context.Context, id int64, amount int64) error
	UpdateExp(ctx context.Context, id int64, exp int64) error
	GetFavorites(ctx context.Context, userID string) ([]*models.UserCard, error)
	GetUserCard(ctx context.Context, userID string, cardID int64) (*models.UserCard, error)
}

type userCardRepository struct {
	db *bun.DB
}

func NewUserCardRepository(db *bun.DB) UserCardRepository {
	return &userCardRepository{db: db}
}

func (r *userCardRepository) Create(ctx context.Context, userCard *models.UserCard) error {
	userCard.CreatedAt = time.Now()
	userCard.UpdatedAt = time.Now()
	_, err := r.db.NewInsert().Model(userCard).Exec(ctx)
	return err
}

func (r *userCardRepository) GetByID(ctx context.Context, id int64) (*models.UserCard, error) {
	userCard := new(models.UserCard)
	err := r.db.NewSelect().
		Model(userCard).
		Where("id = ?", id).
		Scan(ctx)
	return userCard, err
}

func (r *userCardRepository) GetByUserIDAndCardID(ctx context.Context, userID string, cardID int64) (*models.UserCard, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	log.Printf("[GoHYE] [DEBUG] Checking card ownership - UserID: %s, CardID: %d", userID, cardID)

	var userCard models.UserCard
	err := r.db.NewSelect().
		Model(&userCard).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[GoHYE] [DEBUG] No card found in user_cards table - UserID: %s, CardID: %d", userID, cardID)
			return nil, nil
		}
		log.Printf("[GoHYE] [ERROR] Database error while checking ownership: %v", err)
		return nil, fmt.Errorf("failed to get user card: %w", err)
	}

	if userCard.Amount <= 0 {
		log.Printf("[GoHYE] [DEBUG] Attempting to fix card amount - UserID: %s, CardID: %d", userID, cardID)

		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to start transaction: %w", err)
		}
		defer tx.Rollback()

		_, err = tx.NewUpdate().
			Model(&userCard).
			Set("amount = ?", 1).
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND card_id = ?", userID, cardID).
			Exec(ctx)

		if err != nil {
			log.Printf("[GoHYE] [ERROR] Failed to update card amount: %v", err)
			return nil, fmt.Errorf("failed to update card amount: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}

		userCard.Amount = 1
		log.Printf("[GoHYE] [DEBUG] Successfully fixed card amount - UserID: %s, CardID: %d, New Amount: %d",
			userID, cardID, userCard.Amount)
	}

	log.Printf("[GoHYE] [DEBUG] Card ownership check result: UserID: %s, CardID: %d, Amount: %d",
		userCard.UserID, userCard.CardID, userCard.Amount)

	return &userCard, nil
}

func (r *userCardRepository) GetAllByUserID(ctx context.Context, userID string) ([]*models.UserCard, error) {
	var userCards []*models.UserCard
	err := r.db.NewSelect().
		Model(&userCards).
		Where("user_id = ?", userID).
		Order("obtained DESC").
		Scan(ctx)
	return userCards, err
}

func (r *userCardRepository) Update(ctx context.Context, userCard *models.UserCard) error {
	userCard.UpdatedAt = time.Now()
	_, err := r.db.NewUpdate().
		Model(userCard).
		WherePK().
		Exec(ctx)
	return err
}

func (r *userCardRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.NewDelete().
		Model((*models.UserCard)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (r *userCardRepository) UpdateAmount(ctx context.Context, id int64, amount int64) error {
	_, err := r.db.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + ?", amount).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (r *userCardRepository) UpdateExp(ctx context.Context, id int64, exp int64) error {
	_, err := r.db.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("exp = exp + ?", exp).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (r *userCardRepository) GetFavorites(ctx context.Context, userID string) ([]*models.UserCard, error) {
	var userCards []*models.UserCard
	err := r.db.NewSelect().
		Model(&userCards).
		Where("user_id = ? AND favorite = true", userID).
		Order("obtained DESC").
		Scan(ctx)
	return userCards, err
}

func (r *userCardRepository) GetUserCard(ctx context.Context, userID string, cardID int64) (*models.UserCard, error) {
	userCard := new(models.UserCard)
	err := r.db.NewSelect().
		Model(userCard).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user card: %w", err)
	}

	return userCard, nil
}
