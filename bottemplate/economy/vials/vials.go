package vials

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy"
)

const (
	// Vial conversion rates based on card rarity
	VialRate1Star = 0.05 // 5% of card price
	VialRate2Star = 0.08 // 8% of card price
	VialRate3Star = 0.12 // 12% of card price
)

type VialManager struct {
	db        *database.DB
	priceCalc *economy.PriceCalculator
	mu        sync.RWMutex
}

func NewVialManager(db *database.DB, priceCalc *economy.PriceCalculator) *VialManager {
	return &VialManager{
		db:        db,
		priceCalc: priceCalc,
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
		vialRate = VialRate1Star
	case 2:
		vialRate = VialRate2Star
	case 3:
		vialRate = VialRate3Star
	default:
		return 0, fmt.Errorf("invalid card level for liquefying: %d", card.Level)
	}

	vials := int64(math.Floor(float64(price) * vialRate))
	return vials, nil
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

	// Start transaction
	tx, err := vm.db.BunDB().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var card models.Card

	// Handle both card ID and card name inputs
	switch v := cardNameOrID.(type) {
	case int64:
		// Existing ID-based logic
		err = tx.NewSelect().
			Model(&card).
			Where("id = ?", v).
			Scan(ctx)
	case string:
		// New name-based search
		foundCard, err := vm.findCardByName(ctx, v)
		if err != nil {
			return 0, err
		}
		card = *foundCard
	default:
		return 0, fmt.Errorf("invalid card identifier type")
	}

	if err != nil {
		return 0, fmt.Errorf("failed to get card: %w", err)
	}

	// Verify card level is valid for liquefying
	if card.Level > 3 {
		return 0, fmt.Errorf("cannot liquefy cards above 3 stars")
	}

	// Calculate vial yield
	vials, err := vm.CalculateVialYield(ctx, &card)
	if err != nil {
		return 0, err
	}

	// First verify the user owns the card
	var userCard models.UserCard
	err = tx.NewSelect().
		Model(&userCard).
		Where("user_id = ? AND card_id = ?", userIDStr, card.ID).
		Scan(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get user card: %w", err)
	}

	// Handle card removal based on amount
	if userCard.Amount <= 1 {
		// Delete the record if amount is 0 or 1
		result, err := tx.NewDelete().
			Model((*models.UserCard)(nil)).
			Where("user_id = ? AND card_id = ?", userIDStr, card.ID).
			Exec(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to delete card: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil || rowsAffected == 0 {
			return 0, fmt.Errorf("card not found in user's inventory")
		}
	} else {
		// Decrement amount if greater than 1
		result, err := tx.NewUpdate().
			Model((*models.UserCard)(nil)).
			Set("amount = amount - 1").
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND card_id = ?", userIDStr, card.ID).
			Exec(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to update card amount: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil || rowsAffected == 0 {
			return 0, fmt.Errorf("card not found in user's inventory")
		}
	}

	// Add vials to user's balance
	err = vm.AddVials(ctx, userID, vials)
	if err != nil {
		return 0, fmt.Errorf("failed to add vials: %w", err)
	}

	// Update user stats based on card level
	statField := fmt.Sprintf("liquefy%d", card.Level)
	_, err = tx.NewUpdate().
		Model((*models.User)(nil)).
		Set("user_stats = jsonb_set(user_stats, '{"+statField+"}', (COALESCE((user_stats->'"+statField+"')::bigint, 0) + 1)::text::jsonb)").
		Where("discord_id = ?", userIDStr).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to update stats: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return vials, nil
}
