package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
)

const (
	maxBatchSize = 1000
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
	Search(ctx context.Context, filters SearchFilters, offset, limit int) ([]*models.Card, int, error)
	UpdateUserCard(ctx context.Context, userCard *models.UserCard) error
	DeleteUserCard(ctx context.Context, id int64) error
	GetUserCard(ctx context.Context, userID string, cardID int64) (*models.UserCard, error)
	GetAllByUserID(ctx context.Context, userID string) ([]*models.UserCard, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*models.Card, error)
	GetByQuery(ctx context.Context, query string) (*models.Card, error)
	GetLastCardID(ctx context.Context) (int64, error)
	BatchCreateWithTransaction(ctx context.Context, tx bun.Tx, cards []*models.Card) error
	GetCardCount(ctx context.Context) (int64, error)

	// Enhanced search methods for optimization
	GetByNameOrIDFuzzy(ctx context.Context, query string) (*models.Card, error)
	SearchByNameFuzzy(ctx context.Context, query string, limit int) ([]*models.Card, error)
	SearchAdminMode(ctx context.Context, query string, filters SearchFilters) ([]*models.Card, error)
}

type cardRepository struct {
	db    *bun.DB
	cache *sync.Map
}

func NewCardRepository(db *bun.DB) CardRepository {
	return &cardRepository{
		db:    db,
		cache: &sync.Map{},
	}
}

func (r *cardRepository) Create(ctx context.Context, card *models.Card) error {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	card.CreatedAt = time.Now()
	card.UpdatedAt = time.Now()

	_, err := r.db.NewInsert().
		Model(card).
		Returning("id").
		Exec(ctx)

	return err
}

func (r *cardRepository) GetByID(ctx context.Context, id int64) (*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	card := new(models.Card)
	err := r.db.NewSelect().
		Model(card).
		Where("id = ?", id).
		Scan(ctx)

	return card, err
}

func (r *cardRepository) GetByName(ctx context.Context, name string) ([]*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	cacheKey := fmt.Sprintf("name:%s", name)
	if cached, ok := r.getFromCache(cacheKey); ok {
		return cached.([]*models.Card), nil
	}

	var cards []*models.Card
	err := r.db.NewSelect().
		Model(&cards).
		Where("LOWER(name) = LOWER(?)", name).
		Scan(ctx)

	if err == nil {
		r.setCache(cacheKey, cards, config.CacheExpiration)
	}

	return cards, err
}

func (r *cardRepository) GetAll(ctx context.Context) ([]*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	var cards []*models.Card
	err := r.db.NewSelect().
		Model(&cards).
		Order("id ASC").
		Scan(ctx)

	return cards, err
}

func (r *cardRepository) GetByCollectionID(ctx context.Context, colID string) ([]*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	cacheKey := fmt.Sprintf("collection:%s", colID)
	if cached, ok := r.getFromCache(cacheKey); ok {
		return cached.([]*models.Card), nil
	}

	var cards []*models.Card
	err := r.db.NewSelect().
		Model(&cards).
		Where("col_id = ?", colID).
		Order("id ASC").
		Scan(ctx)

	if err == nil {
		r.setCache(cacheKey, cards, config.CacheExpiration)
	}

	return cards, err
}

func (r *cardRepository) Update(ctx context.Context, card *models.Card) error {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	card.UpdatedAt = time.Now()

	_, err := r.db.NewUpdate().
		Model(card).
		WherePK().
		Exec(ctx)

	if err == nil {
		r.invalidateCache(card.ID)
	}

	return err
}

func (r *cardRepository) Delete(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	_, err := r.db.NewDelete().
		Model((*models.Card)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	if err == nil {
		r.invalidateCache(id)
	}

	return err
}

func (r *cardRepository) GetByTag(ctx context.Context, tag string) ([]*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	cacheKey := fmt.Sprintf("tag:%s", tag)
	if cached, ok := r.getFromCache(cacheKey); ok {
		return cached.([]*models.Card), nil
	}

	var cards []*models.Card
	err := r.db.NewSelect().
		Model(&cards).
		Where("? = ANY(tags)", tag).
		Order("id ASC").
		Scan(ctx)

	if err == nil {
		r.setCache(cacheKey, cards, config.CacheExpiration)
	}

	return cards, err
}

func (r *cardRepository) BulkCreate(ctx context.Context, cards []*models.Card) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	if len(cards) == 0 {
		return 0, nil
	}

	now := time.Now()
	totalCreated := 0

	// Process in batches to avoid overwhelming the database
	for i := 0; i < len(cards); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(cards) {
			end = len(cards)
		}
		batch := cards[i:end]

		for _, card := range batch {
			card.CreatedAt = now
			card.UpdatedAt = now
		}

		res, err := r.db.NewInsert().
			Model(&batch).
			On("CONFLICT (id) DO UPDATE").
			Set("name = EXCLUDED.name").
			Set("level = EXCLUDED.level").
			Set("animated = EXCLUDED.animated").
			Set("col_id = EXCLUDED.col_id").
			Set("tags = EXCLUDED.tags").
			Set("updated_at = EXCLUDED.updated_at").
			Exec(ctx)

		if err != nil {
			return totalCreated, err
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return totalCreated, err
		}

		totalCreated += int(affected)
	}

	return totalCreated, nil
}

func (r *cardRepository) GetByLevel(ctx context.Context, level int) ([]*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	cacheKey := fmt.Sprintf("level:%d", level)
	if cached, ok := r.getFromCache(cacheKey); ok {
		return cached.([]*models.Card), nil
	}

	var cards []*models.Card
	err := r.db.NewSelect().
		Model(&cards).
		Where("level = ?", level).
		Order("id ASC").
		Scan(ctx)

	if err == nil {
		r.setCache(cacheKey, cards, config.CacheExpiration)
	}

	return cards, err
}

func (r *cardRepository) GetAnimated(ctx context.Context) ([]*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	if cached, ok := r.getFromCache("animated"); ok {
		return cached.([]*models.Card), nil
	}

	var cards []*models.Card
	err := r.db.NewSelect().
		Model(&cards).
		Where("animated = true").
		Order("id ASC").
		Scan(ctx)

	if err == nil {
		r.setCache("animated", cards, config.CacheExpiration)
	}

	return cards, err
}

func (r *cardRepository) SafeDelete(ctx context.Context, cardID int64) (*models.DeletionReport, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	report := &models.DeletionReport{
		CardID:           cardID,
		UserCardsDeleted: 0,
		CardDeleted:      false,
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return report, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Get card details before deletion
	card := new(models.Card)
	err = tx.NewSelect().
		Model(card).
		Where("id = ?", cardID).
		Scan(ctx)

	if err != nil {
		return report, fmt.Errorf("card not found: %w", err)
	}

	// Delete user_cards entries
	result, err := tx.NewDelete().
		Model((*models.UserCard)(nil)).
		Where("card_id = ?", cardID).
		Exec(ctx)

	if err != nil {
		return report, fmt.Errorf("failed to delete user cards: %w", err)
	}

	affected, _ := result.RowsAffected()
	report.UserCardsDeleted = int(affected)

	// Delete the card
	result, err = tx.NewDelete().
		Model((*models.Card)(nil)).
		Where("id = ?", cardID).
		Exec(ctx)

	if err != nil {
		return report, fmt.Errorf("failed to delete card: %w", err)
	}

	cardAffected, _ := result.RowsAffected()
	report.CardDeleted = cardAffected > 0

	// Delete image if card was deleted
	if report.CardDeleted {
		// Note: Image deletion should be handled by the service layer
		// when calling this repository method, not within the repository itself
		r.invalidateCache(cardID)
	}

	if err = tx.Commit(); err != nil {
		return report, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return report, nil
}

// First, let's improve the cache key generation
func generateCacheKey(filters SearchFilters, offset, limit int) string {
	return fmt.Sprintf("search:name=%s:id=%d:level=%d:col=%s:type=%s:animated=%v:offset=%d:limit=%d",
		filters.Name,
		filters.ID,
		filters.Level,
		filters.Collection,
		filters.Type,
		filters.Animated,
		offset,
		limit,
	)
}

func (r *cardRepository) Search(ctx context.Context, filters SearchFilters, offset, limit int) ([]*models.Card, int, error) {
	fmt.Printf("\n=== Search Query Debug ===\n")
	fmt.Printf("Filters: %+v\n", filters)
	fmt.Printf("Offset: %d, Limit: %d\n", offset, limit)

	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	cacheKey := generateCacheKey(filters, offset, limit)
	fmt.Printf("Cache key: %s\n", cacheKey)

	if cached, ok := r.getFromCache(cacheKey); ok {
		fmt.Printf("Cache hit! Returning cached results\n")
		results := cached.(map[string]interface{})
		return results["cards"].([]*models.Card), results["count"].(int), nil
	}

	countCacheKey := fmt.Sprintf("count:%s:%s:%d:%v",
		filters.Collection,
		filters.Type,
		filters.Level,
		filters.Animated,
	)

	var count int
	if cachedCount, ok := r.getFromCache(countCacheKey); ok {
		count = cachedCount.(int)
		fmt.Printf("Count cache hit! Count: %d\n", count)
	} else {
		countQuery := r.db.NewSelect().Model((*models.Card)(nil))

		// Apply filters to count query
		if filters.Name != "" {
			countQuery = countQuery.Where("LOWER(name) LIKE LOWER(?)", "%"+filters.Name+"%")
		}
		if filters.ID != 0 {
			countQuery = countQuery.Where("id = ?", filters.ID)
		}
		if filters.Level != 0 {
			countQuery = countQuery.Where("level = ?", filters.Level)
		}
		if filters.Collection != "" {
			countQuery = countQuery.Where("LOWER(col_id) LIKE LOWER(?)", "%"+filters.Collection+"%")
		}
		if filters.Type != "" {
			countQuery = countQuery.Where("? = ANY(tags)", filters.Type)
		}
		if filters.Animated {
			countQuery = countQuery.Where("animated = true")
		}

		var err error
		count, err = countQuery.Count(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to count results: %w", err)
		}
		r.setCache(countCacheKey, count, config.CacheExpiration*2)
	}

	// Create and execute the main query
	query := r.db.NewSelect().Model((*models.Card)(nil))

	// Improve name search logic
	if filters.Name != "" {
		// Use a simple LIKE query to avoid duplicates
		query = query.Where("LOWER(name) LIKE LOWER(?)", "%"+filters.Name+"%")
	}

	// Apply filters to main query
	if filters.ID != 0 {
		query = query.Where("id = ?", filters.ID)
	}
	if filters.Level != 0 {
		query = query.Where("level = ?", filters.Level)
	}
	if filters.Collection != "" {
		query = query.Where("LOWER(col_id) LIKE LOWER(?)", "%"+filters.Collection+"%")
	}
	if filters.Type != "" {
		query = query.Where("? = ANY(tags)", filters.Type)
	}
	if filters.Animated {
		query = query.Where("animated = true")
	}

	// Apply pagination and ordering
	query = query.Order("id ASC").
		Limit(limit).
		Offset(offset)

	// Execute the query
	var cards []*models.Card
	err := query.Scan(ctx, &cards)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch results: %w", err)
	}

	// Cache the results
	cacheData := map[string]interface{}{
		"cards": cards,
		"count": count,
	}
	r.setCache(cacheKey, cacheData, config.CacheExpiration)
	fmt.Printf("Results cached with key: %s\n", cacheKey)

	fmt.Printf("=== End Search Debug ===\n\n")
	return cards, count, nil
}

// Improve cache entry structure
type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
	key       string
}

// Improve cache helper methods
func (r *cardRepository) getFromCache(key string) (interface{}, bool) {
	value, ok := r.cache.Load(key)
	if !ok {
		fmt.Printf("Cache miss for key: %s\n", key)
		return nil, false
	}

	entry := value.(cacheEntry)
	if time.Now().After(entry.expiresAt) {
		fmt.Printf("Cache expired for key: %s\n", key)
		r.cache.Delete(key)
		return nil, false
	}

	fmt.Printf("Cache hit for key: %s\n", key)
	return entry.data, true
}

func (r *cardRepository) setCache(key string, value interface{}, duration time.Duration) {
	entry := cacheEntry{
		data:      value,
		expiresAt: time.Now().Add(duration),
		key:       key,
	}
	r.cache.Store(key, entry)
	fmt.Printf("Cached value for key: %s (expires: %s)\n", key, entry.expiresAt)
}

// Add this method for cache invalidation
func (r *cardRepository) invalidateCache(cardID int64) {
	r.cache.Delete(fmt.Sprintf("card:%d", cardID))
	// Also delete any collection or level caches that might contain this card
	r.cache.Range(func(key, _ interface{}) bool {
		keyStr := key.(string)
		if strings.HasPrefix(keyStr, "search:") ||
			strings.HasPrefix(keyStr, "collection:") ||
			strings.HasPrefix(keyStr, "level:") {
			r.cache.Delete(key)
		}
		return true
	})
	fmt.Printf("Invalidated cache for card ID: %d\n", cardID)
}

func (r *cardRepository) UpdateUserCard(ctx context.Context, userCard *models.UserCard) error {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	userCard.UpdatedAt = time.Now()
	_, err := r.db.NewUpdate().
		Model(userCard).
		WherePK().
		Exec(ctx)

	if err == nil {
		r.invalidateCache(userCard.CardID)
	}

	return err
}

func (r *cardRepository) DeleteUserCard(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	_, err := r.db.NewDelete().
		Model((*models.UserCard)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

func (r *cardRepository) GetUserCard(ctx context.Context, userID string, cardID int64) (*models.UserCard, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	userCard := new(models.UserCard)
	err := r.db.NewRaw(`
		SELECT * FROM user_cards 
		WHERE user_id = ? AND card_id = ? 
		LIMIT 1
	`, userID, cardID).Scan(ctx, userCard)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("you don't own card #%d", cardID)
		}
		return nil, fmt.Errorf("database error: %v", err)
	}

	return userCard, nil
}

func (r *cardRepository) GetAllByUserID(ctx context.Context, userID string) ([]*models.UserCard, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	userCards := make([]*models.UserCard, 0)

    err := r.db.NewSelect().
        Model(&userCards).
        Where("user_id = ?", userID).
        Where("amount > 0").
        Order("level DESC").
        Order("id ASC").
        Scan(ctx)

	if err != nil {
		fmt.Printf("Database error: %v\n", err)
		return nil, fmt.Errorf("failed to fetch user cards: %w", err)
	}

	return userCards, nil
}

func (r *cardRepository) GetByIDs(ctx context.Context, ids []int64) ([]*models.Card, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var cards []*models.Card
	err := r.db.NewSelect().
		Model(&cards).
		Where("id IN (?)", bun.In(ids)).
		Scan(ctx)

	return cards, err
}

func (r *cardRepository) GetByQuery(ctx context.Context, query string) (*models.Card, error) {
	card := new(models.Card)
	err := r.db.NewSelect().
		Model(card).
		Where("LOWER(name) LIKE LOWER(?)", "%"+query+"%").
		WhereOr("id::text = ?", query).
		Limit(1).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no card found matching query: %s", query)
		}
		return nil, err
	}

	return card, nil
}

func (r *cardRepository) GetLastCardID(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	var maxID int64
	err := r.db.NewSelect().
		Model((*models.Card)(nil)).
		ColumnExpr("COALESCE(MAX(id), 0)").
		Scan(ctx, &maxID)
	return maxID, err
}

func (r *cardRepository) BatchCreateWithTransaction(ctx context.Context, tx bun.Tx, cards []*models.Card) error {
	if len(cards) == 0 {
		return nil
	}

	now := time.Now()
	for _, card := range cards {
		card.CreatedAt = now
		card.UpdatedAt = now
	}

	_, err := tx.NewInsert().
		Model(&cards).
		Exec(ctx)

	return err
}

// GetCardCount returns the total number of cards for performance
func (r *cardRepository) GetCardCount(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	count, err := r.db.NewSelect().
		Model((*models.Card)(nil)).
		Count(ctx)

	return int64(count), err
}

// GetByNameOrIDFuzzy finds a single card by name or ID with fuzzy matching - optimized for price stats
func (r *cardRepository) GetByNameOrIDFuzzy(ctx context.Context, query string) (*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	card := new(models.Card)

	// Try exact ID match first
	err := r.db.NewSelect().
		Model(card).
		Where("id::text = ?", query).
		Scan(ctx)

	if err == nil {
		return card, nil
	}

	// Try exact name match (case insensitive)
	err = r.db.NewSelect().
		Model(card).
		Where("LOWER(name) = LOWER(?)", query).
		Scan(ctx)

	if err == nil {
		return card, nil
	}

	// Try fuzzy name match with LIKE
	err = r.db.NewSelect().
		Model(card).
		Where("LOWER(name) LIKE LOWER(?)", "%"+query+"%").
		OrderExpr("LENGTH(name)"). // Prefer shorter matches
		Limit(1).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no card found matching query: %s", query)
		}
		return nil, err
	}

	return card, nil
}

// SearchByNameFuzzy performs fuzzy search by name with limit - optimized for admin commands
func (r *cardRepository) SearchByNameFuzzy(ctx context.Context, query string, limit int) ([]*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	if limit <= 0 {
		limit = 10 // Default reasonable limit
	}

	var cards []*models.Card
	err := r.db.NewSelect().
		Model(&cards).
		Where("LOWER(name) LIKE LOWER(?)", "%"+query+"%").
		OrderExpr("LENGTH(name)").Order("name"). // Prefer shorter, then alphabetical
		Limit(limit).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to search cards: %w", err)
	}

	return cards, nil
}

// SearchAdminMode performs admin search without promo/exclusion filters
func (r *cardRepository) SearchAdminMode(ctx context.Context, query string, filters SearchFilters) ([]*models.Card, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	// Start with base query
	selectQuery := r.db.NewSelect().Model((*models.Card)(nil))

	// Apply name filter if provided
	if query != "" {
		selectQuery = selectQuery.Where("LOWER(name) LIKE LOWER(?)", "%"+query+"%")
	}

	// Apply other filters
	if filters.Level != 0 {
		selectQuery = selectQuery.Where("level = ?", filters.Level)
	}

	if filters.Collection != "" {
		selectQuery = selectQuery.Where("LOWER(col_id) LIKE LOWER(?)", "%"+filters.Collection+"%")
	}

	if filters.Animated {
		selectQuery = selectQuery.Where("animated = true")
	}

	// Admin mode: no promo/exclusion filtering, order by relevance
	selectQuery = selectQuery.OrderExpr("LENGTH(name)").Order("level DESC", "name")

	var cards []*models.Card
	err := selectQuery.Scan(ctx, &cards)

	if err != nil {
		return nil, fmt.Errorf("failed to perform admin search: %w", err)
	}

	return cards, nil
}
