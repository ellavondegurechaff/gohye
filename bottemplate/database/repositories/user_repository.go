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
	GetBalance(ctx context.Context, userID string) (int64, error)
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
	slog.Debug("UserRepository.GetByDiscordID called",
		slog.String("type", "db"),
		slog.String("operation", "GetByDiscordID"),
		slog.String("discord_id", discordID))

	user := new(models.User)
	err := r.db.NewSelect().
		Model(user).
		Where("discord_id = ?", discordID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Warn("User not found in database",
				slog.String("type", "db"),
				slog.String("operation", "GetByDiscordID"),
				slog.String("discord_id", discordID),
				slog.String("error", "sql.ErrNoRows"))
		} else {
			slog.Error("Database error when getting user",
				slog.String("type", "db"),
				slog.String("operation", "GetByDiscordID"),
				slog.String("discord_id", discordID),
				slog.String("error", err.Error()))
		}
		return user, err
	}

	slog.Debug("Successfully retrieved user from database",
		slog.String("type", "db"),
		slog.String("operation", "GetByDiscordID"),
		slog.String("discord_id", discordID),
		slog.String("username", user.Username),
		slog.Int64("user_internal_id", user.ID))

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
	slog.Debug("UserRepository.GetUsers called",
		slog.String("type", "db"),
		slog.String("operation", "GetUsers"))

	var users []*models.User
	err := r.db.NewSelect().
		Model(&users).
		Order("balance DESC").
		Scan(ctx)

	if err != nil {
		slog.Error("Database error when getting all users",
			slog.String("type", "db"),
			slog.String("operation", "GetUsers"),
			slog.String("error", err.Error()))
		return nil, err
	}

	slog.Debug("Successfully retrieved all users from database",
		slog.String("type", "db"),
		slog.String("operation", "GetUsers"),
		slog.Int("user_count", len(users)))

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

func (r *userRepository) GetBalance(ctx context.Context, userID string) (int64, error) {
	var user models.User
	err := r.db.NewSelect().
		Model(&user).
		Where("id = ?", userID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	return user.Balance, nil
}
