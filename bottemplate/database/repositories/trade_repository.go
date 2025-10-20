package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

type TradeRepository interface {
	DB() *bun.DB
	Create(ctx context.Context, trade *models.Trade) error
	GetByID(ctx context.Context, id int64) (*models.Trade, error)
	GetByTradeID(ctx context.Context, tradeID string) (*models.Trade, error)
	GetUserTrades(ctx context.Context, userID string, status models.TradeStatus) ([]*models.Trade, error)
	GetAllUserTrades(ctx context.Context, userID string) ([]*models.Trade, error)
	UpdateStatus(ctx context.Context, tradeID int64, status models.TradeStatus) error
	ExecuteTrade(ctx context.Context, tradeID int64) error
	GetPendingTradesBetweenUsers(ctx context.Context, user1ID, user2ID string) ([]*models.Trade, error)
	ExpireOldTrades(ctx context.Context) error
	TradeIDExists(ctx context.Context, tradeID string) (bool, error)
	GetTradeWithCards(ctx context.Context, tradeID int64) (*models.Trade, error)
}

type tradeRepository struct {
	db *bun.DB
}

func NewTradeRepository(db *bun.DB) TradeRepository {
	return &tradeRepository{db: db}
}

func (r *tradeRepository) DB() *bun.DB {
	return r.db
}

func (r *tradeRepository) Create(ctx context.Context, trade *models.Trade) error {
	trade.CreatedAt = time.Now()
	trade.UpdatedAt = time.Now()
	trade.Status = models.TradePending
	trade.ExpiresAt = time.Now().Add(7 * 24 * time.Hour) // 7 days

	_, err := r.db.NewInsert().Model(trade).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create trade: %w", err)
	}
	return nil
}

func (r *tradeRepository) GetByID(ctx context.Context, id int64) (*models.Trade, error) {
	trade := new(models.Trade)
	err := r.db.NewSelect().
		Model(trade).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("trade not found")
		}
		return nil, fmt.Errorf("failed to get trade: %w", err)
	}
	return trade, nil
}

func (r *tradeRepository) GetByTradeID(ctx context.Context, tradeID string) (*models.Trade, error) {
	trade := new(models.Trade)
	err := r.db.NewSelect().
		Model(trade).
		Where("trade_id = ?", tradeID).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("trade not found")
		}
		return nil, fmt.Errorf("failed to get trade: %w", err)
	}
	return trade, nil
}

func (r *tradeRepository) GetUserTrades(ctx context.Context, userID string, status models.TradeStatus) ([]*models.Trade, error) {
	var trades []*models.Trade
	err := r.db.NewSelect().
		Model(&trades).
		Where("(offerer_id = ? OR target_id = ?) AND status = ?", userID, userID, status).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get user trades: %w", err)
	}
	return trades, nil
}

func (r *tradeRepository) GetAllUserTrades(ctx context.Context, userID string) ([]*models.Trade, error) {
	var trades []*models.Trade
	err := r.db.NewSelect().
		Model(&trades).
		Where("offerer_id = ? OR target_id = ?", userID, userID).
		Order("created_at DESC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get all user trades: %w", err)
	}
	return trades, nil
}

func (r *tradeRepository) UpdateStatus(ctx context.Context, tradeID int64, status models.TradeStatus) error {
	_, err := r.db.NewUpdate().
		Model((*models.Trade)(nil)).
		Set("status = ?", status).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", tradeID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update trade status: %w", err)
	}
	return nil
}

func (r *tradeRepository) ExecuteTrade(ctx context.Context, tradeID int64) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock and get trade details
	trade := new(models.Trade)
	err = tx.NewSelect().
		Model(trade).
		Where("id = ? AND status = ?", tradeID, models.TradePending).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("trade not found or not pending")
		}
		return fmt.Errorf("failed to get trade: %w", err)
	}

	// Check if trade has expired
	if time.Now().After(trade.ExpiresAt) {
		_, err = tx.NewUpdate().
			Model(trade).
			Set("status = ?", models.TradeExpired).
			Set("updated_at = ?", time.Now()).
			Where("id = ?", tradeID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark trade as expired: %w", err)
		}
		return fmt.Errorf("trade has expired")
	}

	// Verify both users still own their cards
	var offererCard models.UserCard
	err = tx.NewSelect().
		Model(&offererCard).
		Where("user_id = ? AND card_id = ? AND amount > 0", trade.OffererID, trade.OffererCardID).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("offerer no longer owns the card")
		}
		return fmt.Errorf("failed to verify offerer's card: %w", err)
	}

	var targetCard models.UserCard
	err = tx.NewSelect().
		Model(&targetCard).
		Where("user_id = ? AND card_id = ? AND amount > 0", trade.TargetID, trade.TargetCardID).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("target user no longer owns the card")
		}
		return fmt.Errorf("failed to verify target's card: %w", err)
	}

	// Execute the card transfer
	// Remove card from offerer
	_, err = tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount - 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", trade.OffererID, trade.OffererCardID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove card from offerer: %w", err)
	}

	// Remove card from target
	_, err = tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount - 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", trade.TargetID, trade.TargetCardID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove card from target: %w", err)
	}

	// Give offerer's card to target
	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", trade.TargetID, trade.OffererCardID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to give card to target: %w", err)
	}

	// If target doesn't have the card, create new entry
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for target: %w", err)
	}

	if affected == 0 {
		_, err = tx.NewInsert().
			Model(&models.UserCard{
				UserID:    trade.TargetID,
				CardID:    trade.OffererCardID,
				Amount:    1,
				Obtained:  time.Now(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to create new card entry for target: %w", err)
		}
	}

	// Give target's card to offerer
	result, err = tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", trade.OffererID, trade.TargetCardID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to give card to offerer: %w", err)
	}

	// If offerer doesn't have the card, create new entry
	affected, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for offerer: %w", err)
	}

	if affected == 0 {
		_, err = tx.NewInsert().
			Model(&models.UserCard{
				UserID:    trade.OffererID,
				CardID:    trade.TargetCardID,
				Amount:    1,
				Obtained:  time.Now(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to create new card entry for offerer: %w", err)
		}
	}

	// Update trade status to accepted
	_, err = tx.NewUpdate().
		Model(trade).
		Set("status = ?", models.TradeAccepted).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", tradeID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update trade status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit trade transaction: %w", err)
	}

	slog.Info("Trade executed successfully",
		slog.Int64("trade_id", tradeID),
		slog.String("trade_uuid", trade.TradeID),
		slog.String("offerer_id", trade.OffererID),
		slog.String("target_id", trade.TargetID))

	return nil
}

func (r *tradeRepository) GetPendingTradesBetweenUsers(ctx context.Context, user1ID, user2ID string) ([]*models.Trade, error) {
	var trades []*models.Trade
	err := r.db.NewSelect().
		Model(&trades).
		Where("((offerer_id = ? AND target_id = ?) OR (offerer_id = ? AND target_id = ?)) AND status = ?",
			user1ID, user2ID, user2ID, user1ID, models.TradePending).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get pending trades between users: %w", err)
	}
	return trades, nil
}

func (r *tradeRepository) ExpireOldTrades(ctx context.Context) error {
	_, err := r.db.NewUpdate().
		Model((*models.Trade)(nil)).
		Set("status = ?", models.TradeExpired).
		Set("updated_at = ?", time.Now()).
		Where("status = ? AND expires_at <= ?", models.TradePending, time.Now()).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to expire old trades: %w", err)
	}
	return nil
}

func (r *tradeRepository) TradeIDExists(ctx context.Context, tradeID string) (bool, error) {
	exists, err := r.db.NewSelect().
		Model((*models.Trade)(nil)).
		Where("trade_id = ?", tradeID).
		Exists(ctx)

	return exists, err
}

func (r *tradeRepository) GetTradeWithCards(ctx context.Context, tradeID int64) (*models.Trade, error) {
	trade := new(models.Trade)
	err := r.db.NewSelect().
		Model(trade).
		Relation("OffererCard").
		Relation("TargetCard").
		Where("t.id = ?", tradeID).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("trade not found")
		}
		return nil, fmt.Errorf("failed to get trade with cards: %w", err)
	}
	return trade, nil
}