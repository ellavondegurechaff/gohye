package forge

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/uptrace/bun"
)

const (
	// Forge cost is based on average price of both cards
	ForgeCostMultiplier = 0.15 // 15% of combined card value
	MinForgeCost        = 1000 // Minimum forge cost
)

type ForgeManager struct {
	db        *database.DB
	priceCalc *economy.PriceCalculator
	mu        sync.RWMutex
}

func NewForgeManager(db *database.DB, priceCalc *economy.PriceCalculator) *ForgeManager {
	return &ForgeManager{
		db:        db,
		priceCalc: priceCalc,
	}
}

func (fm *ForgeManager) CalculateForgeCost(ctx context.Context, card1, card2 *models.Card) (int64, error) {
	price1, err := fm.priceCalc.GetLatestPrice(ctx, card1.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get price for card 1: %w", err)
	}

	price2, err := fm.priceCalc.GetLatestPrice(ctx, card2.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get price for card 2: %w", err)
	}

	avgPrice := (price1 + price2) / 2
	forgeCost := int64(math.Max(float64(avgPrice)*ForgeCostMultiplier, float64(MinForgeCost)))

	return forgeCost, nil
}

func (fm *ForgeManager) ForgeCards(ctx context.Context, userID int64, card1ID, card2ID int64) (*models.Card, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	tx, err := fm.db.BunDB().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get both cards
	var card1, card2 models.Card
	err = tx.NewSelect().Model(&card1).Where("id = ?", card1ID).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get card 1: %w", err)
	}

	err = tx.NewSelect().Model(&card2).Where("id = ?", card2ID).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get card 2: %w", err)
	}

	// Calculate forge cost
	forgeCost, err := fm.CalculateForgeCost(ctx, &card1, &card2)
	if err != nil {
		return nil, err
	}

	// Verify user has enough balance
	var user models.User
	err = tx.NewSelect().
		Model(&user).
		Column("balance").
		Where("discord_id = ?", strconv.FormatInt(userID, 10)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user balance: %w", err)
	}

	if user.Balance < forgeCost {
		return nil, fmt.Errorf("insufficient balance to forge cards")
	}

	// Deduct balance
	_, err = tx.NewUpdate().
		Model((*models.User)(nil)).
		Set("balance = balance - ?", forgeCost).
		Where("discord_id = ?", strconv.FormatInt(userID, 10)).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update balance: %w", err)
	}

	// Remove the forged cards
	err = fm.removeCards(ctx, tx, userID, []int64{card1ID, card2ID})
	if err != nil {
		return nil, err
	}

	// Get new card of same level
	var possibleCards []*models.Card
	query := tx.NewSelect().
		Model((*models.Card)(nil)).
		Where("level = ?", card1.Level)

	// If both cards are from same collection, restrict to that collection
	if card1.ColID == card2.ColID {
		query.Where("col_id = ?", card1.ColID)
	}

	err = query.Scan(ctx, &possibleCards)
	if err != nil {
		return nil, fmt.Errorf("failed to get possible cards: %w", err)
	}

	if len(possibleCards) == 0 {
		return nil, fmt.Errorf("no possible cards found for forging result")
	}

	// Select random card
	newCard := possibleCards[rand.Intn(len(possibleCards))]

	// Add new card to user's inventory
	err = fm.addCard(ctx, tx, userID, newCard.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return newCard, nil
}

func (fm *ForgeManager) removeCards(ctx context.Context, tx bun.Tx, userID int64, cardIDs []int64) error {
	userIDStr := strconv.FormatInt(userID, 10)

	for _, cardID := range cardIDs {
		result, err := tx.NewDelete().
			Model((*models.UserCard)(nil)).
			Where("user_id = ? AND card_id = ?", userIDStr, cardID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to remove card %d: %w", cardID, err)
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			return fmt.Errorf("card %d not found in inventory", cardID)
		}
	}
	return nil
}

func (fm *ForgeManager) addCard(ctx context.Context, tx bun.Tx, userID int64, cardID int64) error {
	userIDStr := strconv.FormatInt(userID, 10)

	// Try to update existing card first
	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + 1").
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", userIDStr, cardID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update card amount: %w", err)
	}

	// If no existing card, insert new one
	if affected, _ := result.RowsAffected(); affected == 0 {
		_, err = tx.NewInsert().
			Model(&models.UserCard{
				UserID:    userIDStr,
				CardID:    cardID,
				Amount:    1,
				Exp:       0,
				Obtained:  time.Now(),
				UpdatedAt: time.Now(),
			}).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to add new card: %w", err)
		}
	}
	return nil
}
