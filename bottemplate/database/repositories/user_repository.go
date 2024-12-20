package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByDiscordID(ctx context.Context, discordID string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, discordID string) error
	UpdateBalance(ctx context.Context, discordID string, balance int64) error
	UpdateLastDaily(ctx context.Context, discordID string) error
	GetTopUsers(ctx context.Context, limit int) ([]*models.User, error)
	GetUsers(ctx context.Context) ([]*models.User, error)
	UpdateLastWork(ctx context.Context, discordID string) error
}

type userRepository struct {
	db *bun.DB
}

func NewUserRepository(db *bun.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	_, err := r.db.NewInsert().Model(user).Exec(ctx)
	return err
}

func (r *userRepository) GetByDiscordID(ctx context.Context, discordID string) (*models.User, error) {
	user := new(models.User)
	err := r.db.NewSelect().
		Model(user).
		Where("discord_id = ?", discordID).
		Scan(ctx)
	return user, err
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()
	_, err := r.db.NewUpdate().
		Model(user).
		WherePK().
		Exec(ctx)
	return err
}

func (r *userRepository) Delete(ctx context.Context, discordID string) error {
	_, err := r.db.NewDelete().
		Model((*models.User)(nil)).
		Where("discord_id = ?", discordID).
		Exec(ctx)
	return err
}

func (r *userRepository) UpdateBalance(ctx context.Context, discordID string, balance int64) error {
	_, err := r.db.NewUpdate().
		Model((*models.User)(nil)).
		Set("balance = balance + ?", balance).
		Set("updated_at = ?", time.Now()).
		Where("discord_id = ?", discordID).
		Exec(ctx)
	return err
}

func (r *userRepository) UpdateLastDaily(ctx context.Context, discordID string) error {
	_, err := r.db.NewUpdate().
		Model((*models.User)(nil)).
		Set("last_daily = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Where("discord_id = ?", discordID).
		Exec(ctx)
	return err
}

func (r *userRepository) GetTopUsers(ctx context.Context, limit int) ([]*models.User, error) {
	var users []*models.User
	err := r.db.NewSelect().
		Model(&users).
		OrderExpr("balance DESC").
		Limit(limit).
		Scan(ctx)
	return users, err
}

func (r *userRepository) GetUsers(ctx context.Context) ([]*models.User, error) {
	var users []*models.User
	err := r.db.NewSelect().
		Model(&users).
		Order("balance DESC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return users, nil
}

func (r *userRepository) UpdateLastWork(ctx context.Context, discordID string) error {
	_, err := r.db.NewUpdate().
		Model((*models.User)(nil)).
		Set("last_work = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Where("discord_id = ?", discordID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update last_work: %w", err)
	}
	return nil
}
