package repositories

import (
	"context"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByDiscordID(ctx context.Context, discordID string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, discordID string) error
	UpdateExp(ctx context.Context, discordID string, exp int64) error
	UpdateLastDaily(ctx context.Context, discordID string) error
	GetTopUsers(ctx context.Context, limit int) ([]*models.User, error)
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

func (r *userRepository) UpdateExp(ctx context.Context, discordID string, exp int64) error {
	_, err := r.db.NewUpdate().
		Model((*models.User)(nil)).
		Set("exp = exp + ?", exp).
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
		OrderExpr("exp DESC").
		Limit(limit).
		Scan(ctx)
	return users, err
}
