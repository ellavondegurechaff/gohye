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
	botutils "github.com/disgoorg/bot-template/bottemplate/utils"
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

// CalculateForgeCostWithEffects calculates forge cost and applies effect discounts
func (fm *ForgeManager) CalculateForgeCostWithEffects(ctx context.Context, card1, card2 *models.Card, userID string, effectIntegrator interface{}) (int64, error) {
	baseCost, err := fm.CalculateForgeCost(ctx, card1, card2)
	if err != nil {
		return 0, err
	}

	// Apply effect discounts if integrator is available
	if integrator, ok := effectIntegrator.(interface {
		ApplyForgeDiscount(ctx context.Context, userID string, baseCost int) int
	}); ok && integrator != nil {
		finalCost := integrator.ApplyForgeDiscount(ctx, userID, int(baseCost))
		return int64(finalCost), nil
	}

	return baseCost, nil
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

		// Get new card of same level with proper filtering
		var possibleCards []*models.Card
		query := tx.NewSelect().
			Model((*models.Card)(nil)).
			Where("level = ?", card1.Level).
			Where("id != ? AND id != ?", card1.ID, card2.ID) // Exclude input cards

		err = query.Scan(ctx, &possibleCards)
		if err != nil {
			return fmt.Errorf("failed to get possible cards: %w", err)
		}

		// Apply sophisticated filtering based on input card characteristics
		filteredCards := filterForgeOutputCards(possibleCards, &card1, &card2)

		if len(filteredCards) == 0 {
			return fmt.Errorf("no eligible cards found for forging result")
		}

		// Select random card from filtered results
		newCard = filteredCards[rand.Intn(len(filteredCards))]

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

// filterForgeOutputCards applies sophisticated filtering for forge output based on input card characteristics
// This implements the logic from CommandReference.js to ensure proper card type consistency
// Special album logic: album + album = album, album + normal = normal, normal + normal = normal
func filterForgeOutputCards(possibleCards []*models.Card, card1, card2 *models.Card) []*models.Card {
	var filteredCards []*models.Card

	// Get collection info for input cards
	col1Info, col1Exists := botutils.GetCollectionInfo(card1.ColID)
	col2Info, col2Exists := botutils.GetCollectionInfo(card2.ColID)

	// Determine if both cards are from the same collection
	sameCollection := card1.ColID == card2.ColID

	// Determine if both cards are promo
	bothPromo := (col1Exists && col1Info.IsPromo) && (col2Exists && col2Info.IsPromo)

	// Special handling for album collections
	isCard1Album := card1.ColID == "bgalbums" || card1.ColID == "ggalbums"
	isCard2Album := card2.ColID == "bgalbums" || card2.ColID == "ggalbums"
	bothAlbums := isCard1Album && isCard2Album

	// Determine if both cards have same tags (group consistency)
	sameTags := hasSameTags(card1.Tags, card2.Tags)

	for _, card := range possibleCards {
		// Get collection info for potential output card
		colInfo, exists := botutils.GetCollectionInfo(card.ColID)
		isOutputAlbum := card.ColID == "bgalbums" || card.ColID == "ggalbums"

		// 1. Exclude forge-excluded collections (fragments, album, liveauction, jackpot, birthdays, limited)
		if exists && colInfo.IsForgeExcluded {
			continue
		}

		// 2. Special album collection logic
		if bothAlbums {
			// If both inputs are albums, output must be album
			if !isOutputAlbum {
				continue // Skip non-album cards when both inputs are albums
			}
		} else if isCard1Album || isCard2Album {
			// If only one input is album, output should NOT be album (normal forge logic)
			if isOutputAlbum {
				continue // Skip album cards when only one input is album
			}
			// Apply normal promo logic for non-album output
			if exists && colInfo.IsPromo {
				continue
			}
		} else {
			// Regular promo collection logic for non-album cases
			if sameCollection && bothPromo {
				// If both input cards are from the same promo collection, result must be promo
				if !exists || !colInfo.IsPromo {
					continue
				}
			} else {
				// For normal forging, ensure result is not from a promo collection
				if exists && colInfo.IsPromo {
					continue
				}
			}
		}

		// 3. Tag consistency (group type consistency: GG→GG, BG→BG)
		if sameTags {
			// If both input cards have same tags, output should have same tags
			if !hasSameTags(card.Tags, card1.Tags) {
				continue
			}
		} else {
			// If input cards have different tags, only allow standard group tags
			if !hasStandardGroupTags(card.Tags) {
				continue
			}
		}

		// 4. If both cards are from same collection, restrict to that collection (collection bonus)
		if sameCollection {
			if card.ColID != card1.ColID {
				continue
			}
		}

		// Card passed all filters
		filteredCards = append(filteredCards, card)
	}

	return filteredCards
}

// hasSameTags checks if two tag slices contain the same group tags
func hasSameTags(tags1, tags2 []string) bool {
	if len(tags1) == 0 && len(tags2) == 0 {
		return true
	}

	// Extract group tags only (girlgroups, boygroups)
	group1 := getGroupTag(tags1)
	group2 := getGroupTag(tags2)

	return group1 != "" && group1 == group2
}

// getGroupTag extracts the group tag (girlgroups or boygroups) from a tag slice
func getGroupTag(tags []string) string {
	for _, tag := range tags {
		if tag == "girlgroups" || tag == "boygroups" {
			return tag
		}
	}
	return ""
}

// hasStandardGroupTags checks if the card has standard group tags (girlgroups or boygroups)
func hasStandardGroupTags(tags []string) bool {
	groupTag := getGroupTag(tags)
	return groupTag == "girlgroups" || groupTag == "boygroups"
}

// Helper methods removed - now using standardized transaction utilities
