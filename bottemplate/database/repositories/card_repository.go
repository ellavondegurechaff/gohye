package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/uptrace/bun"
)

type CardRepository interface {
	Create(ctx context.Context, card *models.Card) error
	GetByID(ctx context.Context, id int64) (*models.Card, error)
	GetByName(ctx context.Context, name string) ([]*models.Card, error)
	GetAll(ctx context.Context) ([]*models.Card, error)
	GetByCollectionID(ctx context.Context, colID string) ([]*models.Card, error)
	Update(ctx context.Context, card *models.Card) error
	Delete(ctx context.Context, id int64) error
	GetByTag(ctx context.Context, tag string) ([]*models.Card, error)
	BulkCreate(ctx context.Context, cards []*models.Card) (int, error)
	GetByLevel(ctx context.Context, level int) ([]*models.Card, error)
	GetAnimated(ctx context.Context) ([]*models.Card, error)
	SafeDelete(ctx context.Context, cardID int64) (*models.DeletionReport, error)
}

type cardRepository struct {
	db            *bun.DB
	spacesService *services.SpacesService
}

func NewCardRepository(db *bun.DB, spacesService *services.SpacesService) CardRepository {
	return &cardRepository{
		db:            db,
		spacesService: spacesService,
	}
}

func (r *cardRepository) Create(ctx context.Context, card *models.Card) error {
	card.CreatedAt = time.Now()
	card.UpdatedAt = time.Now()
	_, err := r.db.NewInsert().Model(card).Exec(ctx)
	return err
}

func (r *cardRepository) GetByID(ctx context.Context, id int64) (*models.Card, error) {
	card := new(models.Card)
	err := r.db.NewSelect().Model(card).Where("id = ?", id).Scan(ctx)
	return card, err
}

func (r *cardRepository) GetByName(ctx context.Context, name string) ([]*models.Card, error) {
	var cards []*models.Card
	err := r.db.NewSelect().Model(&cards).Where("name = ?", name).Scan(ctx)
	return cards, err
}

func (r *cardRepository) GetAll(ctx context.Context) ([]*models.Card, error) {
	var cards []*models.Card
	err := r.db.NewSelect().Model(&cards).Scan(ctx)
	return cards, err
}

func (r *cardRepository) GetByCollectionID(ctx context.Context, colID string) ([]*models.Card, error) {
	var cards []*models.Card
	err := r.db.NewSelect().Model(&cards).Where("col_id = ?", colID).Scan(ctx)
	return cards, err
}

func (r *cardRepository) Update(ctx context.Context, card *models.Card) error {
	card.UpdatedAt = time.Now()
	_, err := r.db.NewUpdate().Model(card).WherePK().Exec(ctx)
	return err
}

func (r *cardRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.NewDelete().Model((*models.Card)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (r *cardRepository) GetByTag(ctx context.Context, tag string) ([]*models.Card, error) {
	var cards []*models.Card
	err := r.db.NewSelect().Model(&cards).Where("? = ANY(tags)", tag).Scan(ctx)
	return cards, err
}

func (r *cardRepository) BulkCreate(ctx context.Context, cards []*models.Card) (int, error) {
	if len(cards) == 0 {
		return 0, nil
	}

	now := time.Now()
	for _, card := range cards {
		card.CreatedAt = now
		card.UpdatedAt = now
	}

	// Using ON CONFLICT (id) DO UPDATE to handle duplicates
	res, err := r.db.NewInsert().
		Model(&cards).
		On("CONFLICT (id) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("level = EXCLUDED.level").
		Set("animated = EXCLUDED.animated").
		Set("col_id = EXCLUDED.col_id").
		Set("tags = EXCLUDED.tags").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)

	if err != nil {
		return 0, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return len(cards), nil // Return the input length if we can't get affected rows
	}

	return int(affected), nil
}

func (r *cardRepository) GetByLevel(ctx context.Context, level int) ([]*models.Card, error) {
	var cards []*models.Card
	err := r.db.NewSelect().Model(&cards).Where("level = ?", level).Scan(ctx)
	return cards, err
}

func (r *cardRepository) GetAnimated(ctx context.Context) ([]*models.Card, error) {
	var cards []*models.Card
	err := r.db.NewSelect().Model(&cards).Where("animated = true").Scan(ctx)
	return cards, err
}

func (r *cardRepository) SafeDelete(ctx context.Context, cardID int64) (*models.DeletionReport, error) {
	report := &models.DeletionReport{
		CardID:           cardID,
		UserCardsDeleted: 0,
		CardDeleted:      false,
	}

	// Start a transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return report, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	// 1. First verify the card exists
	card := new(models.Card)
	err = tx.NewSelect().
		Model(card).
		Where("id = ?", cardID).
		Scan(ctx)

	if err != nil {
		return report, fmt.Errorf("card not found: %w", err)
	}

	// 2. Delete all user_cards entries for this card
	result, err := tx.NewDelete().
		Model((*models.UserCard)(nil)).
		Where("card_id = ?", cardID).
		Exec(ctx)

	if err != nil {
		return report, fmt.Errorf("failed to delete user cards: %w", err)
	}

	affected, _ := result.RowsAffected()
	report.UserCardsDeleted = int(affected)

	// 3. Delete the card itself
	result, err = tx.NewDelete().
		Model((*models.Card)(nil)).
		Where("id = ?", cardID).
		Exec(ctx)

	if err != nil {
		return report, fmt.Errorf("failed to delete card: %w", err)
	}

	cardAffected, _ := result.RowsAffected()
	report.CardDeleted = cardAffected > 0

	// 4. If card was deleted successfully, delete the image from Spaces
	if report.CardDeleted {
		err = r.spacesService.DeleteCardImage(ctx, card.ColID, card.Name, card.Level)
		if err != nil {
			// Log the error but don't fail the transaction
			fmt.Printf("Warning: Failed to delete image for card %s: %v\n", card.Name, err)
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return report, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return report, nil
}
