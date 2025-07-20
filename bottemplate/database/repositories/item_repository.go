package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

type ItemRepository interface {
	// Item operations
	GetByID(ctx context.Context, id string) (*models.Item, error)
	GetAll(ctx context.Context) ([]*models.Item, error)
	GetByType(ctx context.Context, itemType string) ([]*models.Item, error)

	// User item operations
	GetUserItems(ctx context.Context, userID string) ([]*models.UserItem, error)
	GetUserItem(ctx context.Context, userID, itemID string) (*models.UserItem, error)
	AddUserItem(ctx context.Context, userID, itemID string, quantity int) error
	UpdateUserItemQuantity(ctx context.Context, userID, itemID string, quantity int) error
	RemoveUserItem(ctx context.Context, userID, itemID string, quantity int) error
	HasRequiredItems(ctx context.Context, userID string, requirements map[string]int) (bool, error)
	ConsumeItems(ctx context.Context, userID string, requirements map[string]int) error
}

type itemRepository struct {
	db *bun.DB
}

func NewItemRepository(db *bun.DB) ItemRepository {
	return &itemRepository{db: db}
}

// Item operations

func (r *itemRepository) GetByID(ctx context.Context, id string) (*models.Item, error) {
	var item models.Item
	err := r.db.NewSelect().
		Model(&item).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *itemRepository) GetAll(ctx context.Context) ([]*models.Item, error) {
	var items []*models.Item
	err := r.db.NewSelect().
		Model(&items).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *itemRepository) GetByType(ctx context.Context, itemType string) ([]*models.Item, error) {
	var items []*models.Item
	err := r.db.NewSelect().
		Model(&items).
		Where("type = ?", itemType).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// User item operations

func (r *itemRepository) GetUserItems(ctx context.Context, userID string) ([]*models.UserItem, error) {
	var userItems []*models.UserItem
	err := r.db.NewSelect().
		Model(&userItems).
		Where("user_id = ?", userID).
		Where("quantity > 0").
		Relation("Item").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return userItems, nil
}

func (r *itemRepository) GetUserItem(ctx context.Context, userID, itemID string) (*models.UserItem, error) {
	var userItem models.UserItem
	err := r.db.NewSelect().
		Model(&userItem).
		Where("user_id = ? AND item_id = ?", userID, itemID).
		Relation("Item").
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &userItem, nil
}

func (r *itemRepository) AddUserItem(ctx context.Context, userID, itemID string, quantity int) error {
	// First check if user already has this item
	existingItem, err := r.GetUserItem(ctx, userID, itemID)
	if err != nil {
		return err
	}

	if existingItem != nil {
		// Update existing quantity
		newQuantity := existingItem.Quantity + quantity
		return r.UpdateUserItemQuantity(ctx, userID, itemID, newQuantity)
	}

	// Create new user item
	userItem := &models.UserItem{
		UserID:   userID,
		ItemID:   itemID,
		Quantity: quantity,
	}

	_, err = r.db.NewInsert().
		Model(userItem).
		Exec(ctx)
	return err
}

func (r *itemRepository) UpdateUserItemQuantity(ctx context.Context, userID, itemID string, quantity int) error {
	_, err := r.db.NewUpdate().
		Model(&models.UserItem{}).
		Set("quantity = ?", quantity).
		Set("updated_at = CURRENT_TIMESTAMP").
		Where("user_id = ? AND item_id = ?", userID, itemID).
		Exec(ctx)
	return err
}

func (r *itemRepository) RemoveUserItem(ctx context.Context, userID, itemID string, quantity int) error {
	// Get current item
	userItem, err := r.GetUserItem(ctx, userID, itemID)
	if err != nil {
		return err
	}
	if userItem == nil {
		return fmt.Errorf("user does not have item %s", itemID)
	}

	newQuantity := userItem.Quantity - quantity
	if newQuantity < 0 {
		return fmt.Errorf("insufficient quantity: has %d, trying to remove %d", userItem.Quantity, quantity)
	}

	if newQuantity == 0 {
		// Delete the record if quantity becomes 0
		_, err = r.db.NewDelete().
			Model(&models.UserItem{}).
			Where("user_id = ? AND item_id = ?", userID, itemID).
			Exec(ctx)
		return err
	}

	// Update with new quantity
	return r.UpdateUserItemQuantity(ctx, userID, itemID, newQuantity)
}

func (r *itemRepository) HasRequiredItems(ctx context.Context, userID string, requirements map[string]int) (bool, error) {
	for itemID, requiredQty := range requirements {
		userItem, err := r.GetUserItem(ctx, userID, itemID)
		if err != nil {
			return false, err
		}
		if userItem == nil || userItem.Quantity < requiredQty {
			return false, nil
		}
	}
	return true, nil
}

func (r *itemRepository) ConsumeItems(ctx context.Context, userID string, requirements map[string]int) error {
	// First verify user has all required items
	hasItems, err := r.HasRequiredItems(ctx, userID, requirements)
	if err != nil {
		return err
	}
	if !hasItems {
		return fmt.Errorf("insufficient items")
	}

	// Use transaction to ensure atomicity
	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for itemID, quantity := range requirements {
			err := r.RemoveUserItem(ctx, userID, itemID, quantity)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
