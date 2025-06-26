package utils

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

// TransactionOptions configures transaction behavior
type TransactionOptions struct {
	IsolationLevel sql.IsolationLevel
	Timeout        time.Duration
}

// EconomicTransactionManager provides standardized transaction utilities for economic operations
type EconomicTransactionManager struct {
	db *bun.DB
}

// NewEconomicTransactionManager creates a new transaction manager
func NewEconomicTransactionManager(db *bun.DB) *EconomicTransactionManager {
	return &EconomicTransactionManager{db: db}
}

// StandardTransactionOptions returns default transaction options
func StandardTransactionOptions() *TransactionOptions {
	return &TransactionOptions{
		IsolationLevel: sql.LevelReadCommitted,
		Timeout:        DefaultTxTimeout,
	}
}

// SerializableTransactionOptions returns serializable isolation level options for critical operations
func SerializableTransactionOptions() *TransactionOptions {
	return &TransactionOptions{
		IsolationLevel: sql.LevelSerializable,
		Timeout:        DefaultTxTimeout,
	}
}

// WithTransaction executes a function within a database transaction
func (etm *EconomicTransactionManager) WithTransaction(ctx context.Context, opts *TransactionOptions, fn func(context.Context, bun.Tx) error) error {
	if opts == nil {
		opts = StandardTransactionOptions()
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Begin transaction
	tx, err := etm.db.BeginTx(timeoutCtx, &sql.TxOptions{
		Isolation: opts.IsolationLevel,
	})
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute function
	if err := fn(timeoutCtx, tx); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CardOperationOptions configures card inventory operations
type CardOperationOptions struct {
	UserID string
	CardID int64
	Amount int64
}

// BalanceOperationOptions configures balance operations
type BalanceOperationOptions struct {
	UserID string
	Amount int64
	MinimumBalance int64 // Validation threshold
}

// AddCardToInventory adds cards to user inventory with UPSERT logic
func (etm *EconomicTransactionManager) AddCardToInventory(ctx context.Context, tx bun.Tx, opts CardOperationOptions) error {
	// Try to update existing card first
	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + ?", opts.Amount).
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", opts.UserID, opts.CardID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update card amount: %w", err)
	}

	// If no existing card, insert new one
	if affected, _ := result.RowsAffected(); affected == 0 {
		_, err = tx.NewInsert().
			Model(&models.UserCard{
				UserID:    opts.UserID,
				CardID:    opts.CardID,
				Amount:    opts.Amount,
				Exp:       0,
				Obtained:  time.Now(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to add new card: %w", err)
		}
	}

	return nil
}

// RemoveCardFromInventory removes cards from user inventory
func (etm *EconomicTransactionManager) RemoveCardFromInventory(ctx context.Context, tx bun.Tx, opts CardOperationOptions) error {
	// Get current card state for validation
	var userCard models.UserCard
	err := tx.NewSelect().
		Model(&userCard).
		Where("user_id = ? AND card_id = ?", opts.UserID, opts.CardID).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("card not found in inventory")
		}
		return fmt.Errorf("failed to get user card: %w", err)
	}

	// Check if user has enough cards
	if userCard.Amount < opts.Amount {
		return fmt.Errorf("insufficient cards in inventory (has %d, needs %d)", userCard.Amount, opts.Amount)
	}

	if userCard.Amount == opts.Amount {
		// Delete the record if removing all cards
		result, err := tx.NewDelete().
			Model((*models.UserCard)(nil)).
			Where("user_id = ? AND card_id = ?", opts.UserID, opts.CardID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete card: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return fmt.Errorf("card not found in user's inventory")
		}
	} else {
		// Decrease amount
		result, err := tx.NewUpdate().
			Model((*models.UserCard)(nil)).
			Set("amount = amount - ?", opts.Amount).
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND card_id = ? AND amount >= ?", opts.UserID, opts.CardID, opts.Amount).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update card amount: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return fmt.Errorf("failed to remove cards from inventory")
		}
	}

	return nil
}

// ValidateAndUpdateBalance validates user balance and updates it
func (etm *EconomicTransactionManager) ValidateAndUpdateBalance(ctx context.Context, tx bun.Tx, opts BalanceOperationOptions) error {
	// Lock and get current balance
	var user models.User
	err := tx.NewSelect().
		Model(&user).
		Column("balance").
		Where("discord_id = ?", opts.UserID).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user balance: %w", err)
	}

	// Validate minimum balance requirement for deductions
	if opts.Amount < 0 && user.Balance < -opts.Amount {
		return fmt.Errorf("insufficient balance (has %d, needs %d)", user.Balance, -opts.Amount)
	}

	// Apply minimum balance constraint
	if opts.MinimumBalance > 0 && user.Balance+opts.Amount < opts.MinimumBalance {
		return fmt.Errorf("operation would violate minimum balance constraint")
	}

	// Update balance
	result, err := tx.NewUpdate().
		Model((*models.User)(nil)).
		Set("balance = balance + ?", opts.Amount).
		Where("discord_id = ?", opts.UserID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("user not found when updating balance")
	}

	return nil
}

// TransferCard transfers a card from one user to another
func (etm *EconomicTransactionManager) TransferCard(ctx context.Context, tx bun.Tx, fromUserID, toUserID string, cardID int64, amount int64) error {
	// Remove from source
	if err := etm.RemoveCardFromInventory(ctx, tx, CardOperationOptions{
		UserID: fromUserID,
		CardID: cardID,
		Amount: amount,
	}); err != nil {
		return fmt.Errorf("failed to remove card from source: %w", err)
	}

	// Add to destination
	if err := etm.AddCardToInventory(ctx, tx, CardOperationOptions{
		UserID: toUserID,
		CardID: cardID,
		Amount: amount,
	}); err != nil {
		return fmt.Errorf("failed to add card to destination: %w", err)
	}

	return nil
}

// TransferBalance transfers balance from one user to another
func (etm *EconomicTransactionManager) TransferBalance(ctx context.Context, tx bun.Tx, fromUserID, toUserID string, amount int64) error {
	// Deduct from source
	if err := etm.ValidateAndUpdateBalance(ctx, tx, BalanceOperationOptions{
		UserID: fromUserID,
		Amount: -amount,
	}); err != nil {
		return fmt.Errorf("failed to deduct from source: %w", err)
	}

	// Add to destination
	if err := etm.ValidateAndUpdateBalance(ctx, tx, BalanceOperationOptions{
		UserID: toUserID,
		Amount: amount,
	}); err != nil {
		return fmt.Errorf("failed to add to destination: %w", err)
	}

	return nil
}

// GetDB returns the underlying database connection
func (etm *EconomicTransactionManager) GetDB() *bun.DB {
	return etm.db
}