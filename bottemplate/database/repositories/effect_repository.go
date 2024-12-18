package repositories

import (
	"context"
	"fmt"
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
	// Implementation for adding to inventory
	return nil
}

func (r *effectRepository) RemoveFromInventory(ctx context.Context, userID string, itemID string, amount int) error {
	// Implementation for removing from inventory
	return nil
}

func (r *effectRepository) GetInventory(ctx context.Context, userID string) ([]*models.UserInventory, error) {
	var inventory []*models.UserInventory
	err := r.db.NewSelect().
		Model(&inventory).
		Where("user_id = ?", userID).
		Scan(ctx)
	return inventory, err
}
