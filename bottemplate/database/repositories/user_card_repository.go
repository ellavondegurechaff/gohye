package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/uptrace/bun"
)

type UserCardRepository interface {
	Create(ctx context.Context, userCard *models.UserCard) error
	GetByID(ctx context.Context, id int64) (*models.UserCard, error)
	GetByUserIDAndCardID(ctx context.Context, userID string, cardID int64) (*models.UserCard, error)
	GetAllByUserID(ctx context.Context, userID string) ([]*models.UserCard, error)
	Update(ctx context.Context, userCard *models.UserCard) error
	Delete(ctx context.Context, id int64) error
	UpdateAmount(ctx context.Context, id int64, amount int64) error
	UpdateExp(ctx context.Context, id int64, exp int64) error
	GetFavorites(ctx context.Context, userID string) ([]*models.UserCard, error)
	GetUserCard(ctx context.Context, userID string, cardID int64) (*models.UserCard, error)
	CleanupZeroAmountCards(ctx context.Context) error
	GetUserCardsByName(ctx context.Context, userID string, cardName string) ([]*models.UserCard, error)
	GetTotalOwnersCount(ctx context.Context, cardID int64) (int64, error)
}

type userCardRepository struct {
	db *bun.DB
}

func NewUserCardRepository(db *bun.DB) UserCardRepository {
	return &userCardRepository{db: db}
}

func (r *userCardRepository) Create(ctx context.Context, userCard *models.UserCard) error {
	userCard.CreatedAt = time.Now()
	userCard.UpdatedAt = time.Now()
	_, err := r.db.NewInsert().Model(userCard).Exec(ctx)
	return err
}

func (r *userCardRepository) GetByID(ctx context.Context, id int64) (*models.UserCard, error) {
	userCard := new(models.UserCard)
	err := r.db.NewSelect().
		Model(userCard).
		Where("id = ?", id).
		Scan(ctx)
	return userCard, err
}

func (r *userCardRepository) GetByUserIDAndCardID(ctx context.Context, userID string, cardID int64) (*models.UserCard, error) {
	var userCard models.UserCard
	err := r.db.NewSelect().
		Model(&userCard).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		Scan(ctx)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[DEBUG] Card not found in inventory - UserID: %s, CardID: %d", userID, cardID)
			return nil, nil
		}
		log.Printf("[ERROR] Database error while checking ownership: %v", err)
		return nil, fmt.Errorf("failed to get user card: %w", err)
	}

	log.Printf("[DEBUG] Card inventory check - UserID: %s, CardID: %d, Amount: %d",
		userCard.UserID, userCard.CardID, userCard.Amount)

	return &userCard, nil
}

func (r *userCardRepository) GetAllByUserID(ctx context.Context, userID string) ([]*models.UserCard, error) {
	var userCards []*models.UserCard
	err := r.db.NewSelect().
		Model(&userCards).
		Where("user_id = ? AND amount > 0", userID).
		Order("obtained DESC").
		Scan(ctx)
	return userCards, err
}

func (r *userCardRepository) Update(ctx context.Context, userCard *models.UserCard) error {
	if userCard.Amount <= 0 {
		// If amount is 0 or negative, delete the record instead of updating
		_, err := r.db.NewDelete().
			Model((*models.UserCard)(nil)).
			Where("user_id = ? AND card_id = ?", userCard.UserID, userCard.CardID).
			Exec(ctx)
		return err
	}

	userCard.UpdatedAt = time.Now()
	_, err := r.db.NewUpdate().
		Model(userCard).
		WherePK().
		Exec(ctx)
	return err
}

func (r *userCardRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.NewDelete().
		Model((*models.UserCard)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (r *userCardRepository) UpdateAmount(ctx context.Context, id int64, amount int64) error {
	_, err := r.db.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + ?", amount).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (r *userCardRepository) UpdateExp(ctx context.Context, id int64, exp int64) error {
	_, err := r.db.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("exp = exp + ?", exp).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (r *userCardRepository) GetFavorites(ctx context.Context, userID string) ([]*models.UserCard, error) {
	var userCards []*models.UserCard
	err := r.db.NewSelect().
		Model(&userCards).
		Where("user_id = ? AND favorite = true", userID).
		Order("obtained DESC").
		Scan(ctx)
	return userCards, err
}

func (r *userCardRepository) GetUserCard(ctx context.Context, userID string, cardID int64) (*models.UserCard, error) {
	userCard := new(models.UserCard)
	err := r.db.NewSelect().
		Model(userCard).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user card: %w", err)
	}

	return userCard, nil
}

func (r *userCardRepository) CleanupZeroAmountCards(ctx context.Context) error {
	result, err := r.db.NewDelete().
		Model((*models.UserCard)(nil)).
		Where("amount <= 0").
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to cleanup zero amount cards: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("[INFO] Cleaned up %d cards with zero amount", rowsAffected)
	return nil
}

func (r *userCardRepository) GetUserCardsByName(ctx context.Context, userID string, cardName string) ([]*models.UserCard, error) {
	// Get all user's cards with card details in a single JOIN query
	var userCardsWithCards []struct {
		*models.UserCard
		Card *models.Card `bun:"rel:belongs-to,join:card_id=id"`
	}
	
	err := r.db.NewSelect().
		Model(&userCardsWithCards).
		Relation("Card").
		Where("user_cards.user_id = ? AND user_cards.amount > 0", userID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get user cards with details: %w", err)
	}

	// Extract cards for weighted search
	cards := make([]*models.Card, len(userCardsWithCards))
	for i, uc := range userCardsWithCards {
		cards[i] = uc.Card
	}

	// Use weighted search on the cards
	searchResults := utils.WeightedSearch(cards, utils.ParseSearchQuery(cardName))
	if len(searchResults) == 0 {
		return nil, nil
	}

	// Create a map for quick lookup of search results
	searchMap := make(map[int64]bool)
	for _, card := range searchResults {
		searchMap[card.ID] = true
	}

	// Return the user cards corresponding to the matched cards
	var matchedUserCards []*models.UserCard
	for _, ucWithCard := range userCardsWithCards {
		if searchMap[ucWithCard.CardID] {
			matchedUserCards = append(matchedUserCards, ucWithCard.UserCard)
		}
	}

	return matchedUserCards, nil
}

// Helper function to collect card IDs
func (r *userCardRepository) GetTotalOwnersCount(ctx context.Context, cardID int64) (int64, error) {
	var count int64
	err := r.db.NewSelect().
		Model((*models.UserCard)(nil)).
		ColumnExpr("COUNT(DISTINCT user_id)").
		Where("card_id = ? AND amount > 0", cardID).
		Scan(ctx, &count)

	if err != nil {
		return 0, fmt.Errorf("failed to get total owners count: %w", err)
	}

	return count, nil
}
