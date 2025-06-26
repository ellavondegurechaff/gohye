package forge

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/uptrace/bun"
)

// Use centralized economic constants from utils package

type ForgeManager struct {
	db        *database.DB
	priceCalc *economy.PriceCalculator
	mu        sync.RWMutex
	txManager *utils.EconomicTransactionManager
}

func NewForgeManager(db *database.DB, priceCalc *economy.PriceCalculator) *ForgeManager {
	return &ForgeManager{
		db:        db,
		priceCalc: priceCalc,
		txManager: utils.NewEconomicTransactionManager(db.BunDB()),
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
	forgeCost := int64(math.Max(float64(avgPrice)*utils.ForgeCostMultiplier, float64(utils.MinForgeCost)))

	return forgeCost, nil
}

func (fm *ForgeManager) ForgeCards(ctx context.Context, userID int64, card1ID, card2ID int64) (*models.Card, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	userIDStr := strconv.FormatInt(userID, 10)
	var newCard *models.Card

	err := fm.txManager.WithTransaction(ctx, utils.StandardTransactionOptions(), func(ctx context.Context, tx bun.Tx) error {
		// Get both cards
		var card1, card2 models.Card
		err := tx.NewSelect().Model(&card1).Where("id = ?", card1ID).Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to get card 1: %w", err)
		}

		err = tx.NewSelect().Model(&card2).Where("id = ?", card2ID).Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to get card 2: %w", err)
		}

		// Calculate forge cost
		forgeCost, err := fm.CalculateForgeCost(ctx, &card1, &card2)
		if err != nil {
			return err
		}

		// Deduct balance using standardized method
		if err := fm.txManager.ValidateAndUpdateBalance(ctx, tx, utils.BalanceOperationOptions{
			UserID: userIDStr,
			Amount: -forgeCost,
		}); err != nil {
			return fmt.Errorf("insufficient balance to forge cards: %w", err)
		}

		// Remove the forged cards using standardized method
		if err := fm.txManager.RemoveCardFromInventory(ctx, tx, utils.CardOperationOptions{
			UserID: userIDStr,
			CardID: card1ID,
			Amount: 1,
		}); err != nil {
			return fmt.Errorf("failed to remove card 1: %w", err)
		}

		if err := fm.txManager.RemoveCardFromInventory(ctx, tx, utils.CardOperationOptions{
			UserID: userIDStr,
			CardID: card2ID,
			Amount: 1,
		}); err != nil {
			return fmt.Errorf("failed to remove card 2: %w", err)
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
			return fmt.Errorf("failed to get possible cards: %w", err)
		}

		if len(possibleCards) == 0 {
			return fmt.Errorf("no possible cards found for forging result")
		}

		// Select random card
		newCard = possibleCards[rand.Intn(len(possibleCards))]

		// Add new card to user's inventory using standardized method
		if err := fm.txManager.AddCardToInventory(ctx, tx, utils.CardOperationOptions{
			UserID: userIDStr,
			CardID: newCard.ID,
			Amount: 1,
		}); err != nil {
			return fmt.Errorf("failed to add new card: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return newCard, nil
}

// Helper methods removed - now using standardized transaction utilities
