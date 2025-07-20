package utils

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

// TransactionManager provides common transaction patterns for economic operations
type TransactionManager struct {
	db bun.IDB
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(db bun.IDB) *TransactionManager {
	return &TransactionManager{db: db}
}

// UpdateUserBalance updates a user's balance with the specified amount (can be negative)
func (tm *TransactionManager) UpdateUserBalance(ctx context.Context, tx bun.Tx, userID string, amount int64) error {
	var operation string
	var value int64

	if amount >= 0 {
		operation = "balance = balance + ?"
		value = amount
	} else {
		operation = "balance = balance - ?"
		value = -amount
	}

	result, err := tx.NewUpdate().
		Model((*models.User)(nil)).
		Set(operation, value).
		Where("discord_id = ?", userID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	return nil
}

// VerifyUserBalance checks if a user has sufficient balance
func (tm *TransactionManager) VerifyUserBalance(ctx context.Context, tx bun.Tx, userID string, requiredAmount int64) (*models.User, error) {
	var user models.User
	err := tx.NewSelect().
		Model(&user).
		Column("balance").
		Where("discord_id = ?", userID).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get user balance: %w", err)
	}

	if user.Balance < requiredAmount {
		return nil, fmt.Errorf("insufficient balance (required: %d, has: %d)", requiredAmount, user.Balance)
	}

	return &user, nil
}

// UpdateCardAmount updates the amount of a specific card for a user
func (tm *TransactionManager) UpdateCardAmount(ctx context.Context, tx bun.Tx, userID string, cardID int64, amount int64) error {
	userIDStr := userID
	if _, err := strconv.ParseInt(userID, 10, 64); err == nil {
		// userID is numeric, use as-is
	} else {
		// userID is string, use as-is
	}

	var operation string
	var value int64

	if amount >= 0 {
		operation = "amount = amount + ?"
		value = amount
	} else {
		operation = "amount = amount - ?"
		value = -amount
	}

	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set(operation, value).
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", userIDStr, cardID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update card amount: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("card not found in user inventory")
	}

	return nil
}

// UpsertUserCard inserts a new card or updates existing card amount
func (tm *TransactionManager) UpsertUserCard(ctx context.Context, tx bun.Tx, userID string, cardID int64, amount int64) error {
	userIDStr := userID

	_, err := tx.NewInsert().
		Model(&models.UserCard{
			UserID:    userIDStr,
			CardID:    cardID,
			Amount:    amount,
			Obtained:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}).
		On("CONFLICT (user_id, card_id) DO UPDATE").
		Set("amount = user_cards.amount + ?", amount).
		Set("updated_at = ?", time.Now()).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to upsert user card: %w", err)
	}

	return nil
}

// RemoveUserCard removes a card from user's inventory (decrements amount or deletes)
func (tm *TransactionManager) RemoveUserCard(ctx context.Context, tx bun.Tx, userID string, cardID int64) error {
	userIDStr := userID

	// First try to decrement amount
	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount - 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ? AND amount > 1", userIDStr, cardID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update card amount: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	// If no rows were affected, try to delete the record (amount was 1)
	if rowsAffected == 0 {
		result, err = tx.NewDelete().
			Model((*models.UserCard)(nil)).
			Where("user_id = ? AND card_id = ? AND amount = 1", userIDStr, cardID).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to delete card: %w", err)
		}

		rowsAffected, err = result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		if rowsAffected == 0 {
			return fmt.Errorf("card not found in user inventory or insufficient amount")
		}
	}

	return nil
}

// VerifyCardOwnership checks if a user owns a specific card with sufficient amount
func (tm *TransactionManager) VerifyCardOwnership(ctx context.Context, tx bun.Tx, userID string, cardID int64, requiredAmount int64) (*models.UserCard, error) {
	var userCard models.UserCard
	err := tx.NewSelect().
		Model(&userCard).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		For("UPDATE").
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("card not found in inventory")
		}
		return nil, fmt.Errorf("failed to get user card: %w", err)
	}

	if userCard.Amount < requiredAmount {
		return nil, fmt.Errorf("insufficient card amount (required: %d, has: %d)", requiredAmount, userCard.Amount)
	}

	return &userCard, nil
}

// WithTransaction executes a function within a database transaction
func (tm *TransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context, tx bun.Tx) error) error {
	tx, err := tm.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := fn(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// WithTimeoutTransaction executes a function within a transaction with a timeout
func (tm *TransactionManager) WithTimeoutTransaction(ctx context.Context, timeout time.Duration, fn func(ctx context.Context, tx bun.Tx) error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return tm.WithTransaction(timeoutCtx, fn)
}
