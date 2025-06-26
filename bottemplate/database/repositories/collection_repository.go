package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/config"
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
	SearchCollections(ctx context.Context, search string) ([]*models.Collection, error)
	GetCollectionProgress(ctx context.Context, collectionID string, limit int) ([]*models.CollectionProgressResult, error)
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
		Set("fragments = EXCLUDED.fragments").
		Set("tags = EXCLUDED.tags").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)

	return err
}

func (r *collectionRepository) SearchCollections(ctx context.Context, search string) ([]*models.Collection, error) {
	var collections []*models.Collection
	query := r.db.NewSelect().
		Model(&collections)

	if search != "" {
		// First add the WHERE clause for filtering with improved matching
		query = query.Where(`
			LOWER(id) LIKE LOWER(?) OR 
			LOWER(name) LIKE LOWER(?) OR
			LOWER(id) = LOWER(?) OR 
			LOWER(name) = LOWER(?)`,
			"%"+search+"%", "%"+search+"%",
			search, search)

		// Use OrderExpr instead of Order for SQL expressions
		query = query.OrderExpr(`CASE 
			WHEN LOWER(id) = LOWER(?) THEN 0
			WHEN LOWER(name) = LOWER(?) THEN 1
			WHEN LOWER(id) LIKE LOWER(?) THEN 2
			WHEN LOWER(name) LIKE LOWER(?) THEN 3
			ELSE 4 
		END ASC`, search, search, "%"+search+"%", "%"+search+"%")
	}

	// Add final ordering and limit
	err := query.
		Order("name ASC").
		Limit(25).
		Scan(ctx)

	return collections, err
}

// GetCollectionProgress returns leaderboard data for a collection using SQL aggregation
// Equivalent to the MongoDB aggregation pipeline in the JavaScript version
func (r *collectionRepository) GetCollectionProgress(ctx context.Context, collectionID string, limit int) ([]*models.CollectionProgressResult, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	// First, get the collection to check if it's a fragment collection
	collection, err := r.GetByID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	// Get all cards for this collection using existing method
	var cards []*models.Card
	err = r.db.NewSelect().
		Model(&cards).
		Where("col_id = ?", collectionID).
		Scan(ctx)
	
	if err != nil {
		return nil, fmt.Errorf("failed to get collection cards: %w", err)
	}

	// Filter cards based on collection type (same logic as CalculateProgress)
	var filteredCardIDs []int64
	for _, card := range cards {
		if collection.Fragments {
			if card.Level == 1 {
				filteredCardIDs = append(filteredCardIDs, card.ID)
			}
		} else {
			if card.Level < 5 {
				filteredCardIDs = append(filteredCardIDs, card.ID)
			}
		}
	}
	
	if len(filteredCardIDs) == 0 {
		return nil, fmt.Errorf("no eligible cards found for collection %s", collectionID)
	}

	// Build the aggregation query equivalent to MongoDB pipeline
	totalCards := len(filteredCardIDs)
	var results []*models.CollectionProgressResult
	
	query := `
		SELECT 
			u.discord_id,
			u.username,
			COUNT(DISTINCT uc.card_id) as owned_cards,
			ROUND((COUNT(DISTINCT uc.card_id)::decimal / ?) * 100, 2) as progress
		FROM user_cards uc
		JOIN users u ON uc.user_id = u.discord_id
		WHERE uc.card_id IN (?) 
		  AND uc.amount > 0
		GROUP BY u.discord_id, u.username
		ORDER BY progress DESC, owned_cards DESC
		LIMIT ?
	`
	
	err = r.db.NewRaw(query, totalCards, bun.In(filteredCardIDs), limit).Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection progress: %w", err)
	}

	return results, nil
}
