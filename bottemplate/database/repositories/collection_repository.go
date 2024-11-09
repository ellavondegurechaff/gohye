package repositories

import (
	"context"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

type CollectionRepository interface {
	Create(ctx context.Context, collection *models.Collection) error
	GetByID(ctx context.Context, id string) (*models.Collection, error)
	GetAll(ctx context.Context) ([]*models.Collection, error)
	Update(ctx context.Context, collection *models.Collection) error
	Delete(ctx context.Context, id string) error
	BulkCreate(ctx context.Context, collections []*models.Collection) error
}

type collectionRepository struct {
	db *bun.DB
}

func NewCollectionRepository(db *bun.DB) CollectionRepository {
	return &collectionRepository{db: db}
}

func (r *collectionRepository) Create(ctx context.Context, collection *models.Collection) error {
	collection.CreatedAt = time.Now()
	collection.UpdatedAt = time.Now()
	_, err := r.db.NewInsert().Model(collection).Exec(ctx)
	return err
}

func (r *collectionRepository) GetByID(ctx context.Context, id string) (*models.Collection, error) {
	collection := new(models.Collection)
	err := r.db.NewSelect().Model(collection).Where("id = ?", id).Scan(ctx)
	return collection, err
}

func (r *collectionRepository) GetAll(ctx context.Context) ([]*models.Collection, error) {
	var collections []*models.Collection
	err := r.db.NewSelect().Model(&collections).Scan(ctx)
	return collections, err
}

func (r *collectionRepository) Update(ctx context.Context, collection *models.Collection) error {
	collection.UpdatedAt = time.Now()
	_, err := r.db.NewUpdate().Model(collection).WherePK().Exec(ctx)
	return err
}

func (r *collectionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.NewDelete().Model((*models.Collection)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (r *collectionRepository) BulkCreate(ctx context.Context, collections []*models.Collection) error {
	if len(collections) == 0 {
		return nil
	}

	now := time.Now()
	for _, collection := range collections {
		collection.CreatedAt = now
		collection.UpdatedAt = now
	}

	_, err := r.db.NewInsert().
		Model(&collections).
		On("CONFLICT (id) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("origin = EXCLUDED.origin").
		Set("aliases = EXCLUDED.aliases").
		Set("promo = EXCLUDED.promo").
		Set("compressed = EXCLUDED.compressed").
		Set("tags = EXCLUDED.tags").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)

	return err
}
