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

	// Tier progression methods
	UpdateEffectProgress(ctx context.Context, userID string, effectID string, increment int) error
	UpgradeEffectTier(ctx context.Context, userID string, effectID string) error
	GetUserEffectsByTier(ctx context.Context, userID string) ([]*models.UserEffect, error)

	// Cooldown methods
	GetEffectCooldown(ctx context.Context, userID string, effectID string) (*time.Time, error)
	SetEffectCooldown(ctx context.Context, userID string, effectID string, cooldownEnd time.Time) error
}

type effectRepository struct {
	*BaseRepository
}

func NewEffectRepository(db *bun.DB) EffectRepository {
	return &effectRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Implementation of interface methods
func (r *effectRepository) CreateEffectItem(ctx context.Context, item *models.EffectItem) error {
	// Validate required fields
	if err := r.ValidateRequired(map[string]interface{}{
		"id":          item.ID,
		"name":        item.Name,
		"description": item.Description,
		"type":        item.Type,
	}); err != nil {
		return err
	}

	// Set timestamps
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	_, err := r.ExecWithTimeout(ctx, "create", "effect_item", func(ctx context.Context) (sql.Result, error) {
		return r.GetDB().NewInsert().Model(item).Exec(ctx)
	})

	return err
}

func (r *effectRepository) GetEffectItem(ctx context.Context, id string) (*models.EffectItem, error) {
	item := new(models.EffectItem)
	err := r.SelectOneWithTimeout(ctx, "get", "effect_item", id, func(ctx context.Context) error {
		return r.GetDB().NewSelect().Model(item).Where("id = ?", id).Scan(ctx)
	})

	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *effectRepository) ListEffectItems(ctx context.Context) ([]*models.EffectItem, error) {
	var items []*models.EffectItem
	err := r.SelectWithTimeout(ctx, "list", "effect_items", func(ctx context.Context) error {
		return r.GetDB().NewSelect().Model(&items).Order("name ASC").Scan(ctx)
	})
	return items, err
}

func (r *effectRepository) AddUserEffect(ctx context.Context, effect *models.UserEffect) error {
	// Validate required fields
	if err := r.ValidateRequired(map[string]interface{}{
		"user_id":   effect.UserID,
		"effect_id": effect.EffectID,
	}); err != nil {
		return err
	}

	// Set timestamps
	effect.CreatedAt = time.Now()
	effect.UpdatedAt = time.Now()

	_, err := r.ExecWithTimeout(ctx, "add", "user_effect", func(ctx context.Context) (sql.Result, error) {
		return r.GetDB().NewInsert().Model(effect).Exec(ctx)
	})

	return err
}

func (r *effectRepository) GetUserEffect(ctx context.Context, userID string, effectID string) (*models.UserEffect, error) {
	effect := new(models.UserEffect)
	err := r.SelectOneWithTimeout(ctx, "get", "user_effect", fmt.Sprintf("%s:%s", userID, effectID), func(ctx context.Context) error {
		return r.GetDB().NewSelect().
			Model(effect).
			Where("user_id = ? AND effect_id = ?", userID, effectID).
			Scan(ctx)
	})

	if err != nil {
		return nil, err
	}
	return effect, nil
}

func (r *effectRepository) GetActiveUserEffects(ctx context.Context, userID string) ([]*models.UserEffect, error) {
	var effects []*models.UserEffect
	err := r.SelectWithTimeout(ctx, "get_active", "user_effects", func(ctx context.Context) error {
		return r.GetDB().NewSelect().
			Model(&effects).
			Where("user_id = ? AND active = true AND expires_at > ?", userID, time.Now()).
			Order("expires_at ASC").
			Scan(ctx)
	})
	return effects, err
}

func (r *effectRepository) UpdateUserEffect(ctx context.Context, effect *models.UserEffect) error {
	// Set update timestamp
	effect.UpdatedAt = time.Now()

	_, err := r.ExecWithTimeout(ctx, "update", "user_effect", func(ctx context.Context) (sql.Result, error) {
		return r.GetDB().NewUpdate().Model(effect).WherePK().Exec(ctx)
	})

	return err
}

func (r *effectRepository) DeactivateExpiredEffects(ctx context.Context) error {
	_, err := r.ExecWithTimeout(ctx, "deactivate_expired", "user_effects", func(ctx context.Context) (sql.Result, error) {
		return r.GetDB().NewUpdate().
			Model((*models.UserEffect)(nil)).
			Set("active = false").
			Set("updated_at = ?", time.Now()).
			Where("active = true AND expires_at <= ?", time.Now()).
			Exec(ctx)
	})

	return err
}

func (r *effectRepository) AddToInventory(ctx context.Context, userID string, itemID string, amount int) error {
	// Validate required fields
	if err := r.ValidateRequired(map[string]interface{}{
		"user_id": userID,
		"item_id": itemID,
	}); err != nil {
		return err
	}

	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	return r.Transaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Try to update existing inventory item first
		result, err := tx.NewUpdate().
			Model((*models.UserInventory)(nil)).
			Set("amount = amount + ?", amount).
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND item_id = ?", userID, itemID).
			Exec(ctx)
		if err != nil {
			return r.HandleError("update_inventory", "user_inventory", err)
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
				return r.HandleError("add_inventory", "user_inventory", err)
			}
		}

		return nil
	})
}

func (r *effectRepository) RemoveFromInventory(ctx context.Context, userID string, itemID string, amount int) error {
	// Validate required fields
	if err := r.ValidateRequired(map[string]interface{}{
		"user_id": userID,
		"item_id": itemID,
	}); err != nil {
		return err
	}

	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	return r.Transaction(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get current amount
		var inventory models.UserInventory
		err := tx.NewSelect().
			Model(&inventory).
			Where("user_id = ? AND item_id = ?", userID, itemID).
			Scan(ctx)
		if err != nil {
			return r.HandleError("get_inventory", "user_inventory", err)
		}

		if inventory.Amount < amount {
			return fmt.Errorf("insufficient items in inventory: have %d, need %d", inventory.Amount, amount)
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
			return r.HandleError("update_inventory", "user_inventory", err)
		}

		return nil
	})
}

func (r *effectRepository) GetInventory(ctx context.Context, userID string) ([]*models.UserInventory, error) {
	var inventory []*models.UserInventory
	err := r.SelectWithTimeout(ctx, "get_inventory", "user_inventory", func(ctx context.Context) error {
		return r.GetDB().NewSelect().
			Model(&inventory).
			Column("user_id", "item_id", "amount", "created_at", "updated_at").
			Where("user_id = ?", userID).
			Scan(ctx)
	})
	return inventory, err
}

func (r *effectRepository) GetRandomCardForRecipe(ctx context.Context, userID string, stars int64) (*models.Card, error) {
	var card models.Card

	slog.Debug("Executing GetRandomCardForRecipe",
		slog.String("user_id", userID),
		slog.Int64("stars", stars))

	// Collections to exclude from recipe requirements (event/special cards)
	// Based on legacy JavaScript logic + additional exclusions requested
	excludedCollections := []string{
		// Legacy JavaScript exclusions (from item.js:530-536)
		"lottery",   // Lottery cards (legacy)
		"jackpot",   // Jackpot cards (legacy)
		"fragments", // Fragment cards (legacy)
		"albums",    // Album cards (legacy - "album" in JS)

		// Additional exclusions requested by user
		"signed",      // Signed cards
		"liveauction", // Live auction cards
		"birthdays",   // Birthday cards
		"limited",     // Limited cards
	}

	query := r.db.NewSelect().
		Model(&card).
		Where("level = ?", stars).
		Where("NOT EXISTS (SELECT 1 FROM user_cards uc WHERE uc.user_id = ? AND uc.card_id = c.id)", userID)

	// Exclude specific event/special collections
	for _, excludedCol := range excludedCollections {
		query = query.Where("col_id != ?", excludedCol)
	}

	// Also exclude promo collections by checking the collections table
	query = query.Where("NOT EXISTS (SELECT 1 FROM collections col WHERE col.id = c.col_id AND col.promo = true)")

	query = query.OrderExpr("RANDOM()").Limit(1)

	slog.Debug("Query parameters",
		slog.Int64("level", stars),
		slog.String("user_id", userID),
		slog.Any("excluded", excludedCollections))

	err := r.SelectOneWithTimeout(ctx, "get_random_card", "card", fmt.Sprintf("%s:%d", userID, stars), func(ctx context.Context) error {
		return query.Scan(ctx, &card)
	})

	if err != nil {
		if IsNotFound(err) {
			slog.Debug("No uncollected non-event cards found", slog.Int64("stars", stars))
			return nil, fmt.Errorf("no uncollected non-event cards found with %d stars", stars)
		}
		return nil, err
	}

	slog.Debug("Found card",
		slog.Int64("id", card.ID),
		slog.String("name", card.Name),
		slog.Int64("level", int64(card.Level)),
		slog.String("collection", card.ColID))
	return &card, nil
}

func (r *effectRepository) StoreUserRecipe(ctx context.Context, userID string, itemID string, cardIDs []int64) error {
	// Validate required fields
	if err := r.ValidateRequired(map[string]interface{}{
		"user_id": userID,
		"item_id": itemID,
	}); err != nil {
		return err
	}

	if len(cardIDs) == 0 {
		return fmt.Errorf("card_ids cannot be empty")
	}

	recipe := &models.UserRecipe{
		UserID:    userID,
		ItemID:    itemID,
		CardIDs:   cardIDs,
		UpdatedAt: time.Now(),
	}

	_, err := r.ExecWithTimeout(ctx, "store_recipe", "user_recipe", func(ctx context.Context) (sql.Result, error) {
		return r.GetDB().NewInsert().
			Model(recipe).
			On("CONFLICT (user_id, item_id) DO UPDATE").
			Set("card_ids = EXCLUDED.card_ids").
			Set("updated_at = EXCLUDED.updated_at").
			Exec(ctx)
	})

	return err
}

func (r *effectRepository) GetUserRecipe(ctx context.Context, userID string, itemID string) (*models.UserRecipe, error) {
	recipe := new(models.UserRecipe)
	err := r.SelectOneWithTimeout(ctx, "get_recipe", "user_recipe", fmt.Sprintf("%s:%s", userID, itemID), func(ctx context.Context) error {
		return r.GetDB().NewSelect().
			Model(recipe).
			Where("user_id = ? AND item_id = ?", userID, itemID).
			Scan(ctx)
	})

	if err != nil {
		return nil, err
	}
	return recipe, nil
}

func (r *effectRepository) GetCard(ctx context.Context, cardID int64) (*models.Card, error) {
	var card models.Card
	err := r.SelectOneWithTimeout(ctx, "get", "card", cardID, func(ctx context.Context) error {
		return r.GetDB().NewSelect().
			Model(&card).
			Where("id = ?", cardID).
			Scan(ctx)
	})
	if err != nil {
		return nil, err
	}
	return &card, nil
}

// GetEffectCooldown gets the cooldown end time for a specific user effect
func (r *effectRepository) GetEffectCooldown(ctx context.Context, userID string, effectID string) (*time.Time, error) {
	var userEffect models.UserEffect
	err := r.SelectOneWithTimeout(ctx, "get_cooldown", "user_effect", fmt.Sprintf("%s:%s", userID, effectID), func(ctx context.Context) error {
		return r.GetDB().NewSelect().
			Model(&userEffect).
			Where("user_id = ? AND effect_id = ?", userID, effectID).
			Order("created_at DESC").
			Limit(1).
			Scan(ctx)
	})

	if err != nil {
		if IsNotFound(err) {
			return nil, nil // No cooldown found
		}
		return nil, err
	}

	return userEffect.CooldownEndsAt, nil
}

// SetEffectCooldown sets or updates the cooldown for a specific user effect
func (r *effectRepository) SetEffectCooldown(ctx context.Context, userID string, effectID string, cooldownEnd time.Time) error {
	// Validate required fields
	if err := r.ValidateRequired(map[string]interface{}{
		"user_id":   userID,
		"effect_id": effectID,
	}); err != nil {
		return err
	}

	// First try to update existing record
	_, err := r.ExecWithTimeout(ctx, "update_cooldown", "user_effect", func(ctx context.Context) (sql.Result, error) {
		return r.GetDB().NewUpdate().
			Model((*models.UserEffect)(nil)).
			Set("cooldown_ends_at = ?", cooldownEnd).
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND effect_id = ?", userID, effectID).
			Exec(ctx)
	})

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

		_, err = r.ExecWithTimeout(ctx, "create_cooldown", "user_effect", func(ctx context.Context) (sql.Result, error) {
			return r.GetDB().NewInsert().Model(userEffect).Exec(ctx)
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// Tier Progress methods
func (r *effectRepository) UpdateEffectProgress(ctx context.Context, userID string, effectID string, increment int) error {
	// Get current effect
	effect, err := r.GetUserEffect(ctx, userID, effectID)
	if err != nil {
		return fmt.Errorf("failed to get user effect: %w", err)
	}

	// Update progress
	effect.Progress += increment
	effect.UpdatedAt = time.Now()

	_, err = r.ExecWithTimeout(ctx, "update_progress", "user_effect", func(ctx context.Context) (sql.Result, error) {
		return r.GetDB().NewUpdate().
			Model(effect).
			Column("progress", "updated_at").
			Where("id = ?", effect.ID).
			Exec(ctx)
	})

	return err
}

func (r *effectRepository) UpgradeEffectTier(ctx context.Context, userID string, effectID string) error {
	// Get current effect
	effect, err := r.GetUserEffect(ctx, userID, effectID)
	if err != nil {
		return fmt.Errorf("failed to get user effect: %w", err)
	}

	// Increment tier and reset progress
	effect.Tier++
	effect.Progress = 0
	effect.UpdatedAt = time.Now()

	_, err = r.ExecWithTimeout(ctx, "upgrade_tier", "user_effect", func(ctx context.Context) (sql.Result, error) {
		return r.GetDB().NewUpdate().
			Model(effect).
			Column("tier", "progress", "updated_at").
			Where("id = ?", effect.ID).
			Exec(ctx)
	})

	return err
}

func (r *effectRepository) GetUserEffectsByTier(ctx context.Context, userID string) ([]*models.UserEffect, error) {
	var effects []*models.UserEffect
	err := r.SelectWithTimeout(ctx, "get_by_tier", "user_effects", func(ctx context.Context) error {
		return r.GetDB().NewSelect().
			Model(&effects).
			Where("user_id = ?", userID).
			Where("is_recipe = false").
			Where("active = true").
			Order("tier DESC", "effect_id ASC").
			Scan(ctx)
	})
	return effects, err
}

// DeleteUserRecipe deletes a user's recipe after crafting
func (r *effectRepository) DeleteUserRecipe(ctx context.Context, userID string, itemID string) error {
	_, err := r.ExecWithTimeout(ctx, "delete_recipe", "user_recipe", func(ctx context.Context) (sql.Result, error) {
		return r.GetDB().NewDelete().
			Model((*models.UserRecipe)(nil)).
			Where("user_id = ? AND item_id = ?", userID, itemID).
			Exec(ctx)
	})

	return err
}
