package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

type EffectRepository interface {
	// Effect Items
	CreateEffectItem(ctx context.Context, item *models.EffectItem) error
	GetEffectItem(ctx context.Context, id string) (*models.EffectItem, error)
	ListEffectItems(ctx context.Context) ([]*models.EffectItem, error)

	// User Effects
	AddUserEffect(ctx context.Context, effect *models.UserEffect) error
	GetUserEffect(ctx context.Context, userID string, effectID string) (*models.UserEffect, error)
	GetActiveUserEffects(ctx context.Context, userID string) ([]*models.UserEffect, error)
	UpdateUserEffect(ctx context.Context, effect *models.UserEffect) error
	DeactivateExpiredEffects(ctx context.Context) error

	// Inventory
	AddToInventory(ctx context.Context, userID string, itemID string, amount int) error
	RemoveFromInventory(ctx context.Context, userID string, itemID string, amount int) error
	GetInventory(ctx context.Context, userID string) ([]*models.UserInventory, error)

	// Random Card for Recipe
	GetRandomCardForRecipe(ctx context.Context, userID string, stars int64) (*models.Card, error)

	// Recipe methods
	StoreUserRecipe(ctx context.Context, userID string, itemID string, cardIDs []int64) error
	GetUserRecipe(ctx context.Context, userID string, itemID string) (*models.UserRecipe, error)
	DeleteUserRecipe(ctx context.Context, userID string, itemID string) error

	// Card methods
	GetCard(ctx context.Context, cardID int64) (*models.Card, error)

	// Cooldown methods
	GetEffectCooldown(ctx context.Context, userID string, effectID string) (*time.Time, error)
	SetEffectCooldown(ctx context.Context, userID string, effectID string, cooldownEnd time.Time) error
}

type effectRepository struct {
	db *bun.DB
}

func NewEffectRepository(db *bun.DB) EffectRepository {
	return &effectRepository{db: db}
}

// Implementation of interface methods
func (r *effectRepository) CreateEffectItem(ctx context.Context, item *models.EffectItem) error {
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()
	_, err := r.db.NewInsert().Model(item).Exec(ctx)
	return err
}

func (r *effectRepository) GetEffectItem(ctx context.Context, id string) (*models.EffectItem, error) {
	item := new(models.EffectItem)
	err := r.db.NewSelect().Model(item).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get effect item: %w", err)
	}
	return item, nil
}

func (r *effectRepository) ListEffectItems(ctx context.Context) ([]*models.EffectItem, error) {
	var items []*models.EffectItem
	err := r.db.NewSelect().Model(&items).Order("name ASC").Scan(ctx)
	return items, err
}

func (r *effectRepository) AddUserEffect(ctx context.Context, effect *models.UserEffect) error {
	effect.CreatedAt = time.Now()
	effect.UpdatedAt = time.Now()
	_, err := r.db.NewInsert().Model(effect).Exec(ctx)
	return err
}

func (r *effectRepository) GetUserEffect(ctx context.Context, userID string, effectID string) (*models.UserEffect, error) {
	effect := new(models.UserEffect)
	err := r.db.NewSelect().
		Model(effect).
		Where("user_id = ? AND effect_id = ?", userID, effectID).
		Scan(ctx)
	return effect, err
}

func (r *effectRepository) GetActiveUserEffects(ctx context.Context, userID string) ([]*models.UserEffect, error) {
	var effects []*models.UserEffect
	err := r.db.NewSelect().
		Model(&effects).
		Where("user_id = ? AND active = true AND expires_at > ?", userID, time.Now()).
		Order("expires_at ASC").
		Scan(ctx)
	return effects, err
}

func (r *effectRepository) UpdateUserEffect(ctx context.Context, effect *models.UserEffect) error {
	effect.UpdatedAt = time.Now()
	_, err := r.db.NewUpdate().Model(effect).WherePK().Exec(ctx)
	return err
}

func (r *effectRepository) DeactivateExpiredEffects(ctx context.Context) error {
	_, err := r.db.NewUpdate().
		Model((*models.UserEffect)(nil)).
		Set("active = false").
		Where("active = true AND expires_at <= ?", time.Now()).
		Exec(ctx)
	return err
}

func (r *effectRepository) AddToInventory(ctx context.Context, userID string, itemID string, amount int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Try to update existing inventory item first
	result, err := tx.NewUpdate().
		Model((*models.UserInventory)(nil)).
		Set("amount = amount + ?", amount).
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND item_id = ?", userID, itemID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update inventory: %w", err)
	}

	// If no existing item, insert new one
	if affected, _ := result.RowsAffected(); affected == 0 {
		_, err = tx.NewInsert().
			Model(&models.UserInventory{
				UserID:    userID,
				ItemID:    itemID,
				Amount:    amount,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to add new inventory item: %w", err)
		}
	}

	return tx.Commit()
}

func (r *effectRepository) RemoveFromInventory(ctx context.Context, userID string, itemID string, amount int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current amount
	var inventory models.UserInventory
	err = tx.NewSelect().
		Model(&inventory).
		Where("user_id = ? AND item_id = ?", userID, itemID).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to get inventory: %w", err)
	}

	if inventory.Amount < amount {
		return fmt.Errorf("insufficient items in inventory")
	}

	if inventory.Amount == amount {
		// Delete the entry if removing all
		_, err = tx.NewDelete().
			Model((*models.UserInventory)(nil)).
			Where("user_id = ? AND item_id = ?", userID, itemID).
			Exec(ctx)
	} else {
		// Decrease amount
		_, err = tx.NewUpdate().
			Model((*models.UserInventory)(nil)).
			Set("amount = amount - ?", amount).
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND item_id = ?", userID, itemID).
			Exec(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to update inventory: %w", err)
	}

	return tx.Commit()
}

func (r *effectRepository) GetInventory(ctx context.Context, userID string) ([]*models.UserInventory, error) {
	var inventory []*models.UserInventory
	err := r.db.NewSelect().
		Model(&inventory).
		Column("user_id", "item_id", "amount", "created_at", "updated_at").
		Where("user_id = ?", userID).
		Scan(ctx)
	return inventory, err
}

func (r *effectRepository) GetRandomCardForRecipe(ctx context.Context, userID string, stars int64) (*models.Card, error) {
	var card models.Card

	log.Printf("[DEBUG] Executing GetRandomCardForRecipe - UserID: %s, Stars: %d", userID, stars)

	query := r.db.NewSelect().
		Model(&card).
		Where("level = ?", stars).
		Where("NOT EXISTS (SELECT 1 FROM user_cards uc WHERE uc.user_id = ? AND uc.card_id = cards.id)", userID).
		OrderExpr("RANDOM()").
		Limit(1)

	log.Printf("[DEBUG] Query parameters - Level: %d, UserID: %s", stars, userID)

	err := query.Scan(ctx, &card)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[DEBUG] No uncollected cards found with %d stars", stars)
			return nil, fmt.Errorf("no uncollected cards found with %d stars", stars)
		}
		log.Printf("[ERROR] Database error: %v", err)
		return nil, fmt.Errorf("failed to get random card: %w", err)
	}

	log.Printf("[DEBUG] Found card - ID: %d, Name: %s, Level: %d", card.ID, card.Name, card.Level)
	return &card, nil
}

func (r *effectRepository) StoreUserRecipe(ctx context.Context, userID string, itemID string, cardIDs []int64) error {
	recipe := &models.UserRecipe{
		UserID:    userID,
		ItemID:    itemID,
		CardIDs:   cardIDs,
		UpdatedAt: time.Now(),
	}

	_, err := r.db.NewInsert().
		Model(recipe).
		On("CONFLICT (user_id, item_id) DO UPDATE").
		Set("card_ids = EXCLUDED.card_ids").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)

	return err
}

func (r *effectRepository) GetUserRecipe(ctx context.Context, userID string, itemID string) (*models.UserRecipe, error) {
	recipe := new(models.UserRecipe)
	err := r.db.NewSelect().
		Model(recipe).
		Where("user_id = ? AND item_id = ?", userID, itemID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}
	return recipe, nil
}

func (r *effectRepository) GetCard(ctx context.Context, cardID int64) (*models.Card, error) {
	var card models.Card
	err := r.db.NewSelect().
		Model(&card).
		Where("id = ?", cardID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get card: %w", err)
	}
	return &card, nil
}

// GetEffectCooldown gets the cooldown end time for a specific user effect
func (r *effectRepository) GetEffectCooldown(ctx context.Context, userID string, effectID string) (*time.Time, error) {
	var userEffect models.UserEffect
	err := r.db.NewSelect().
		Model(&userEffect).
		Where("user_id = ? AND effect_id = ?", userID, effectID).
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No cooldown found
		}
		return nil, fmt.Errorf("failed to get effect cooldown: %w", err)
	}

	return userEffect.CooldownEndsAt, nil
}

// SetEffectCooldown sets or updates the cooldown for a specific user effect
func (r *effectRepository) SetEffectCooldown(ctx context.Context, userID string, effectID string, cooldownEnd time.Time) error {
	// First try to update existing record
	_, err := r.db.NewUpdate().
		Model((*models.UserEffect)(nil)).
		Set("cooldown_ends_at = ?", cooldownEnd).
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND effect_id = ?", userID, effectID).
		Exec(ctx)

	if err != nil {
		// If update fails, create new record
		userEffect := &models.UserEffect{
			UserID:         userID,
			EffectID:       effectID,
			Active:         false,
			CooldownEndsAt: &cooldownEnd,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		_, err = r.db.NewInsert().
			Model(userEffect).
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("failed to set effect cooldown: %w", err)
		}
	}

	return nil
}

// DeleteUserRecipe deletes a user's recipe after crafting
func (r *effectRepository) DeleteUserRecipe(ctx context.Context, userID string, itemID string) error {
	_, err := r.db.NewDelete().
		Model((*models.UserRecipe)(nil)).
		Where("user_id = ? AND item_id = ?", userID, itemID).
		Exec(ctx)
	
	return err
}
