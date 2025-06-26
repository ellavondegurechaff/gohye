package vials

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy"
	"github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/uptrace/bun"
)

// Use centralized economic constants from utils package

type VialManager struct {
	db        *database.DB
	priceCalc *economy.PriceCalculator
	mu        sync.RWMutex
	txManager *utils.EconomicTransactionManager
}

func NewVialManager(db *database.DB, priceCalc *economy.PriceCalculator) *VialManager {
	return &VialManager{
		db:        db,
		priceCalc: priceCalc,
		txManager: utils.NewEconomicTransactionManager(db.BunDB()),
	}
}

// GetVials returns the current vial balance for a user
func (vm *VialManager) GetVials(ctx context.Context, userID int64) (int64, error) {
	userIDStr := strconv.FormatInt(userID, 10)

	var user models.User
	err := vm.db.BunDB().NewSelect().
		Model(&user).
		Column("user_stats").
		Where("discord_id = ?", userIDStr).
		Scan(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to get vials: %w", err)
	}
	return user.UserStats.Vials, nil
}

// AddVials adds vials to a user's balance
func (vm *VialManager) AddVials(ctx context.Context, userID int64, amount int64) error {
	userIDStr := strconv.FormatInt(userID, 10)

	_, err := vm.db.BunDB().NewUpdate().
		Model((*models.User)(nil)).
		Set("user_stats = jsonb_set(user_stats, '{vials}', (COALESCE((user_stats->>'vials')::bigint, 0) + ?)::text::jsonb)", amount).
		Where("discord_id = ?", userIDStr).
		Exec(ctx)

	return err
}

// CalculateVialYield calculates how many vials a card would yield
func (vm *VialManager) CalculateVialYield(ctx context.Context, card *models.Card) (int64, error) {
	// Get current card price
	price, err := vm.priceCalc.GetLatestPrice(ctx, card.ID)
	if err != nil {
		return 0, fmt.Errorf("failed to get card price: %w", err)
	}

	// Calculate vial yield based on card level
	var vialRate float64
	switch card.Level {
	case 1:
		vialRate = utils.VialRate1Star
	case 2:
		vialRate = utils.VialRate2Star
	case 3:
		vialRate = utils.VialRate3Star
	default:
		return 0, fmt.Errorf("invalid card level for liquefying: %d", card.Level)
	}

	vials := int64(math.Floor(float64(price) * vialRate))
	return vials, nil
}

// CalculateVialYieldWithEffects calculates vial yield and applies effect bonuses
func (vm *VialManager) CalculateVialYieldWithEffects(ctx context.Context, card *models.Card, userID string, effectIntegrator interface{}) (int64, error) {
	baseVials, err := vm.CalculateVialYield(ctx, card)
	if err != nil {
		return 0, err
	}

	// Apply effect bonuses if integrator is available
	if integrator, ok := effectIntegrator.(interface {
		ApplyLiquefyBonus(ctx context.Context, userID string, baseVials int64, cardLevel int) int64
	}); ok && integrator != nil {
		finalVials := integrator.ApplyLiquefyBonus(ctx, userID, baseVials, card.Level)
		return finalVials, nil
	}

	return baseVials, nil
}

// findCardByName is a helper function to find a card by name
func (vm *VialManager) findCardByName(ctx context.Context, cardName string) (*models.Card, error) {
	var card models.Card

	// Clean and prepare the search term
	cleanedName := strings.TrimSpace(strings.ToLower(cardName))

	// Try exact match first
	err := vm.db.BunDB().NewSelect().
		Model(&card).
		Where("LOWER(name) = LOWER(?)", cleanedName).
		Scan(ctx)

	if err != nil {
		// Try prefix match
		err = vm.db.BunDB().NewSelect().
			Model(&card).
			Where("LOWER(name) LIKE LOWER(?)", cleanedName+"%").
			Order("LENGTH(name)").
			Limit(1).
			Scan(ctx)

		if err != nil {
			// Try contains match
			err = vm.db.BunDB().NewSelect().
				Model(&card).
				Where("LOWER(name) LIKE LOWER(?)", "%"+cleanedName+"%").
				Order("LENGTH(name)").
				Limit(1).
				Scan(ctx)

			if err != nil {
				// Try word boundary match
				words := strings.Fields(cleanedName)
				if len(words) > 0 {
					mainWord := words[0]
					err = vm.db.BunDB().NewSelect().
						Model(&card).
						Where("LOWER(name) LIKE LOWER(?)", mainWord+"%").
						Order("LENGTH(name)").
						Limit(1).
						Scan(ctx)

					if err != nil {
						// Try fuzzy match as last resort
						sanitizedName := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(cleanedName, "%")
						err = vm.db.BunDB().NewSelect().
							Model(&card).
							Where("LOWER(REGEXP_REPLACE(name, '[^a-zA-Z0-9]+', '', 'g')) LIKE LOWER(?)", "%"+sanitizedName+"%").
							Order("LENGTH(name)").
							Limit(1).
							Scan(ctx)

						if err != nil {
							return nil, fmt.Errorf("card not found: %s", cardName)
						}
					}
				}
			}
		}
	}

	return &card, nil
}

// LiquefyCard converts a card into vials
func (vm *VialManager) LiquefyCard(ctx context.Context, userID int64, cardNameOrID interface{}) (int64, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	userIDStr := strconv.FormatInt(userID, 10)
	var vials int64

	err := vm.txManager.WithTransaction(ctx, utils.StandardTransactionOptions(), func(ctx context.Context, tx bun.Tx) error {
		var card models.Card

		// Handle both card ID and card name inputs
		switch v := cardNameOrID.(type) {
		case int64:
			// Existing ID-based logic
			err := tx.NewSelect().
				Model(&card).
				Where("id = ?", v).
				Scan(ctx)
			if err != nil {
				return fmt.Errorf("failed to get card: %w", err)
			}
		case string:
			// New name-based search
			foundCard, err := vm.findCardByName(ctx, v)
			if err != nil {
				return err
			}
			card = *foundCard
		default:
			return fmt.Errorf("invalid card identifier type")
		}

		// Verify card level is valid for liquefying
		if card.Level > 3 {
			return fmt.Errorf("cannot liquefy cards above 3 stars")
		}

		// Calculate vial yield
		var err error
		vials, err = vm.CalculateVialYield(ctx, &card)
		if err != nil {
			return err
		}

		// Remove card from inventory using standardized method
		if err := vm.txManager.RemoveCardFromInventory(ctx, tx, utils.CardOperationOptions{
			UserID: userIDStr,
			CardID: card.ID,
			Amount: 1,
		}); err != nil {
			return fmt.Errorf("failed to remove card from inventory: %w", err)
		}

		// Add vials to user's balance using direct JSONB update (preserving existing logic)
		_, err = tx.NewUpdate().
			Model((*models.User)(nil)).
			Set("user_stats = jsonb_set(user_stats, '{vials}', (COALESCE((user_stats->>'vials')::bigint, 0) + ?)::text::jsonb)", vials).
			Where("discord_id = ?", userIDStr).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to add vials: %w", err)
		}

		// Update user stats based on card level
		statField := fmt.Sprintf("liquefy%d", card.Level)
		_, err = tx.NewUpdate().
			Model((*models.User)(nil)).
			Set("user_stats = jsonb_set(user_stats, '{"+statField+"}', (COALESCE((user_stats->'"+statField+"')::bigint, 0) + 1)::text::jsonb)").
			Where("discord_id = ?", userIDStr).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update stats: %w", err)
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return vials, nil
}
