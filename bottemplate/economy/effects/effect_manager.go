package effects

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
)

type Manager struct {
	repo         repositories.EffectRepository
	userRepo     repositories.UserRepository
	userCardRepo repositories.UserCardRepository
	db           *database.DB
}

func NewManager(repo repositories.EffectRepository, userRepo repositories.UserRepository, userCardRepo repositories.UserCardRepository, db *database.DB) *Manager {
	return &Manager{
		repo:         repo,
		userRepo:     userRepo,
		userCardRepo: userCardRepo,
		db:           db,
	}
}

// PurchaseEffect handles the purchase of an effect item
func (m *Manager) PurchaseEffect(ctx context.Context, userID string, effectID string) error {
	// Get effect item from static data
	item, err := m.GetEffectItem(ctx, effectID)
	if err != nil {
		return fmt.Errorf("effect not found: %w", err)
	}

	// Get user
	user, err := m.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Verify currency and balance
	switch item.Currency {
	case models.CurrencyTomato:
		if user.Balance < item.Price {
			return fmt.Errorf("insufficient balance")
		}
		user.Balance -= item.Price
	case models.CurrencyVials:
		if user.UserStats.Vials < item.Price {
			return fmt.Errorf("insufficient vials")
		}
		user.UserStats.Vials -= item.Price
	default:
		return fmt.Errorf("invalid currency")
	}

	// Update user balance
	if err := m.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}

	// For recipe items, store the recipe instead of adding to inventory
	if item.Type == models.EffectTypeRecipe && len(item.Recipe) > 0 {
		// Store recipe for user
		if err := m.StoreRecipeForUser(ctx, userID, effectID); err != nil {
			return fmt.Errorf("failed to store recipe: %w", err)
		}
	} else {
		// For non-recipe items, add directly to inventory
		if err := m.repo.AddToInventory(ctx, userID, effectID, 1); err != nil {
			return fmt.Errorf("failed to add to inventory: %w", err)
		}
	}

	return nil
}

// ActivateEffect activates an effect from inventory
func (m *Manager) ActivateEffect(ctx context.Context, userID string, effectID string) error {
	// Get effect from inventory
	inventory, err := m.repo.GetInventory(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get inventory: %w", err)
	}

	// Check if user has the effect
	hasEffect := false
	for _, item := range inventory {
		if item.ItemID == effectID && item.Amount > 0 {
			hasEffect = true
			break
		}
	}

	if !hasEffect {
		return fmt.Errorf("effect not found in inventory")
	}

	// Get effect details
	effect, err := m.repo.GetEffectItem(ctx, effectID)
	if err != nil {
		return fmt.Errorf("effect not found: %w", err)
	}

	// Create user effect
	expiresAt := time.Now().Add(time.Duration(effect.Duration) * time.Hour)
	userEffect := &models.UserEffect{
		UserID:    userID,
		EffectID:  effectID,
		Active:    true,
		ExpiresAt: &expiresAt,
	}

	// Add effect and remove from inventory
	if err := m.repo.AddUserEffect(ctx, userEffect); err != nil {
		return fmt.Errorf("failed to activate effect: %w", err)
	}

	if err := m.repo.RemoveFromInventory(ctx, userID, effectID, 1); err != nil {
		return fmt.Errorf("failed to remove from inventory: %w", err)
	}

	return nil
}

// ListEffectItems returns all available effect items from static data
func (m *Manager) ListEffectItems(ctx context.Context) ([]*models.EffectItem, error) {
	var items []*models.EffectItem
	for i := range StaticEffectItems {
		items = append(items, StaticEffectItems[i].ToEffectItem())
	}
	return items, nil
}

// GetEffectItem returns a specific effect item by ID from static data
func (m *Manager) GetEffectItem(ctx context.Context, effectID string) (*models.EffectItem, error) {
	staticItem := GetEffectItemByID(effectID)
	if staticItem == nil {
		return nil, fmt.Errorf("effect item not found: %s", effectID)
	}
	return staticItem.ToEffectItem(), nil
}

// ListUserEffects returns all effects in user's inventory with their details
func (m *Manager) ListUserEffects(ctx context.Context, userID string) ([]*models.EffectItem, error) {
	// Get user's inventory
	inventory, err := m.repo.GetInventory(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}

	// Get details for each item
	items := make([]*models.EffectItem, 0, len(inventory))
	for _, inv := range inventory {
		item, err := m.repo.GetEffectItem(ctx, inv.ItemID)
		if err != nil {
			continue // Skip invalid items
		}
		items = append(items, item)
	}

	return items, nil
}

func (m *Manager) GetUserRecipeStatus(ctx context.Context, userID string, effectID string) ([]*models.Card, error) {
	// Get stored recipe
	recipe, err := m.repo.GetUserRecipe(ctx, userID, effectID)
	if err != nil {
		// If no stored recipe exists, create one
		if err := m.StoreRecipeForUser(ctx, userID, effectID); err != nil {
			return nil, fmt.Errorf("failed to store recipe: %w", err)
		}
		recipe, err = m.repo.GetUserRecipe(ctx, userID, effectID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stored recipe: %w", err)
		}
	}

	// Get card details for each stored card ID
	cards := make([]*models.Card, len(recipe.CardIDs))
	for i, cardID := range recipe.CardIDs {
		card, err := m.repo.GetCard(ctx, cardID)
		if err != nil {
			log.Printf("[ERROR] Failed to get card %d: %v", cardID, err)
			continue
		}
		cards[i] = card
	}

	return cards, nil
}

func (m *Manager) GetRandomCardForRecipe(ctx context.Context, userID string, stars int64) (*models.Card, error) {
	log.Printf("[DEBUG] Getting random card for recipe - UserID: %s, Stars: %d", userID, stars)

	card, err := m.repo.GetRandomCardForRecipe(ctx, userID, stars)
	if err != nil {
		log.Printf("[ERROR] Failed to get random card: %v", err)
		return nil, fmt.Errorf("failed to get recipe card: %w", err)
	}

	if card != nil {
		log.Printf("[DEBUG] Found card - ID: %d, Name: %s, Level: %d", card.ID, card.Name, card.Level)
	}

	return card, nil
}

// StoreRecipeForUser stores the specific recipe cards for a user's effect item
func (m *Manager) StoreRecipeForUser(ctx context.Context, userID string, effectID string) error {
	// Get effect item from static data
	item, err := m.GetEffectItem(ctx, effectID)
	if err != nil {
		return fmt.Errorf("effect not found: %w", err)
	}

	// Get recipe cards for each star requirement
	var cardIDs []int64
	for _, stars := range item.Recipe {
		card, err := m.repo.GetRandomCardForRecipe(ctx, userID, stars)
		if err != nil {
			return fmt.Errorf("failed to get recipe card: %w", err)
		}
		if card == nil {
			return fmt.Errorf("no available cards found for %d stars", stars)
		}
		cardIDs = append(cardIDs, card.ID)
	}

	// Store the recipe
	if err := m.repo.StoreUserRecipe(ctx, userID, effectID, cardIDs); err != nil {
		return fmt.Errorf("failed to store recipe: %w", err)
	}

	return nil
}

// CraftEffect crafts an effect from recipe cards
func (m *Manager) CraftEffect(ctx context.Context, userID string, effectID string) error {
	// Get effect item from static data
	item, err := m.GetEffectItem(ctx, effectID)
	if err != nil {
		return fmt.Errorf("effect not found: %w", err)
	}

	// Check if this is a recipe type effect
	if item.Type != models.EffectTypeRecipe {
		return fmt.Errorf("this effect cannot be crafted")
	}

	// Get stored recipe for user
	recipe, err := m.repo.GetUserRecipe(ctx, userID, effectID)
	if err != nil {
		return fmt.Errorf("no recipe found for this effect. Purchase it from the shop first")
	}

	// Verify user has all required cards
	if err := m.verifyUserHasRecipeCards(ctx, userID, recipe.CardIDs); err != nil {
		return fmt.Errorf("missing required cards: %w", err)
	}

	// Remove cards from user's collection
	if err := m.removeCardsFromUser(ctx, userID, recipe.CardIDs); err != nil {
		return fmt.Errorf("failed to remove cards: %w", err)
	}

	// Add crafted effect to inventory
	if err := m.repo.AddToInventory(ctx, userID, effectID, 1); err != nil {
		// Rollback: add cards back if inventory add fails
		m.addCardsToUser(ctx, userID, recipe.CardIDs) // Best effort rollback
		return fmt.Errorf("failed to add crafted effect to inventory: %w", err)
	}

	// Remove the used recipe
	if err := m.repo.DeleteUserRecipe(ctx, userID, effectID); err != nil {
		// Log warning but don't fail the craft
		log.Printf("Warning: failed to remove recipe after crafting: %v", err)
	}

	return nil
}

// verifyUserHasRecipeCards checks if user owns all required recipe cards
func (m *Manager) verifyUserHasRecipeCards(ctx context.Context, userID string, cardIDs []int64) error {
	user, err := m.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Convert user cards to map for O(1) lookup
	userCardMap := make(map[string]bool)
	for _, cardID := range user.Cards {
		userCardMap[cardID] = true
	}

	// Check each required card
	for _, cardID := range cardIDs {
		cardIDStr := fmt.Sprintf("%d", cardID)
		if !userCardMap[cardIDStr] {
			card, err := m.repo.GetCard(ctx, cardID)
			cardName := "Unknown Card"
			if err == nil && card != nil {
				cardName = card.Name
			}
			return fmt.Errorf("missing card: %s (ID: %d)", cardName, cardID)
		}
	}

	return nil
}

// removeCardsFromUser removes specific cards from user's collection
func (m *Manager) removeCardsFromUser(ctx context.Context, userID string, cardIDs []int64) error {
	user, err := m.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Remove each card
	for _, cardID := range cardIDs {
		cardIDStr := fmt.Sprintf("%d", cardID)
		for i, userCardID := range user.Cards {
			if userCardID == cardIDStr {
				// Remove card from slice
				user.Cards = append(user.Cards[:i], user.Cards[i+1:]...)
				break
			}
		}
	}

	// Update user
	return m.userRepo.Update(ctx, user)
}

// addCardsToUser adds cards back to user (for rollback)
func (m *Manager) addCardsToUser(ctx context.Context, userID string, cardIDs []int64) error {
	user, err := m.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return err
	}

	// Add each card back
	for _, cardID := range cardIDs {
		cardIDStr := fmt.Sprintf("%d", cardID)
		user.Cards = append(user.Cards, cardIDStr)
	}

	// Update user
	return m.userRepo.Update(ctx, user)
}

// CanCraftEffect checks if user can craft a specific effect
func (m *Manager) CanCraftEffect(ctx context.Context, userID string, effectID string) (bool, []string, error) {
	// Get stored recipe for user
	recipe, err := m.repo.GetUserRecipe(ctx, userID, effectID)
	if err != nil {
		return false, nil, fmt.Errorf("no recipe found for this effect")
	}

	// Check if user has all required cards
	if err := m.verifyUserHasRecipeCards(ctx, userID, recipe.CardIDs); err != nil {
		return false, []string{err.Error()}, nil
	}

	return true, nil, nil
}

// UseActiveEffect uses an active effect from inventory
func (m *Manager) UseActiveEffect(ctx context.Context, userID string, effectID string, args string) (string, error) {
	// Check if user has the effect in inventory
	inventory, err := m.repo.GetInventory(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get inventory: %w", err)
	}

	hasEffect := false
	for _, item := range inventory {
		if item.ItemID == effectID && item.Amount > 0 {
			hasEffect = true
			break
		}
	}

	if !hasEffect {
		return "", fmt.Errorf("effect not found in inventory")
	}

	// Get effect details
	effect, err := m.repo.GetEffectItem(ctx, effectID)
	if err != nil {
		return "", fmt.Errorf("effect not found: %w", err)
	}

	// Check cooldown
	if effect.Cooldown > 0 {
		cooldownEnds, err := m.GetEffectCooldown(ctx, userID, effectID)
		if err == nil && cooldownEnds != nil && time.Now().Before(*cooldownEnds) {
			remaining := time.Until(*cooldownEnds)
			return "", fmt.Errorf("effect is on cooldown for %v", remaining.Round(time.Minute))
		}
	}

	// Execute effect based on type
	result, used, err := m.executeActiveEffect(ctx, userID, effectID, args)
	if err != nil {
		return "", err
	}

	if !used {
		return result, nil
	}

	// Remove from inventory
	if err := m.repo.RemoveFromInventory(ctx, userID, effectID, 1); err != nil {
		return "", fmt.Errorf("failed to remove from inventory: %w", err)
	}

	// Set cooldown if effect was used (apply spellcard reduction if active)
	if effect.Cooldown > 0 {
		cooldownHours := effect.Cooldown
		
		// Check for spellcard passive effect (40% cooldown reduction)
		// Simple check without creating circular dependency
		activeEffects, err := m.repo.GetActiveUserEffects(ctx, userID)
		if err == nil {
			for _, activeEffect := range activeEffects {
				if activeEffect.EffectID == "spellcard" {
					cooldownHours = int(float64(cooldownHours) * 0.6) // 40% reduction
					log.Printf("Applied spellcard effect: reduced cooldown from %d to %d hours", effect.Cooldown, cooldownHours)
					break
				}
			}
		}
		
		cooldownEnd := time.Now().Add(time.Duration(cooldownHours) * time.Hour)
		if err := m.SetEffectCooldown(ctx, userID, effectID, cooldownEnd); err != nil {
			// Log error but don't fail the whole operation
			fmt.Printf("Warning: failed to set cooldown: %v\n", err)
		}
	}

	return result, nil
}

// executeActiveEffect executes specific active effect logic
func (m *Manager) executeActiveEffect(ctx context.Context, userID string, effectID string, args string) (string, bool, error) {
	switch effectID {
	case "claimrecall":
		return m.executeClaimRecall(ctx, userID)
	case "spaceunity":
		return m.executeSpaceUnity(ctx, userID, args)
	case "judgeday":
		return m.executeJudgeDay(ctx, userID, args)
	default:
		return fmt.Sprintf("Active effect %s not implemented yet", effectID), false, nil
	}
}

// executeClaimRecall implements the claim recall effect
func (m *Manager) executeClaimRecall(ctx context.Context, userID string) (string, bool, error) {
	// Get user
	user, err := m.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return "", false, fmt.Errorf("user not found: %w", err)
	}

	// Check if user has claimed more than 4 cards
	if user.DailyStats.Claims < 5 {
		return "you can only use Claim Recall when you have claimed more than 4 cards!", false, nil
	}

	// Reduce claim count by 4
	user.DailyStats.Claims -= 4

	// Update user
	if err := m.userRepo.Update(ctx, user); err != nil {
		return "", false, fmt.Errorf("failed to update user: %w", err)
	}

	newCost := user.DailyStats.Claims * 50
	return fmt.Sprintf("claim cost has been reset to **%d**", newCost), true, nil
}

// executeSpaceUnity implements the space unity effect
func (m *Manager) executeSpaceUnity(ctx context.Context, userID string, args string) (string, bool, error) {
	if args == "" {
		return "Please specify collection ID (e.g., 'twice' or 'blackpink')", false, nil
	}

	// Remove leading dash if present
	args = strings.TrimPrefix(args, "-")

	// Get collection by ID or alias
	var collection models.Collection
	err := m.db.BunDB().NewSelect().
		Model(&collection).
		Where("id = ? OR ? = ANY(aliases)", args, args).
		Scan(ctx)

	if err != nil {
		return fmt.Sprintf("Collection '%s' not found", args), false, nil
	}

	// Check restrictions (based on legacy system)
	if collection.Promo {
		return "Cannot use this effect on promo collections", false, nil
	}

	// Get user's existing cards
	existingCards, err := m.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return "", false, fmt.Errorf("failed to get user cards: %w", err)
	}

	// Create map of owned card IDs for quick lookup
	ownedCardIDs := make(map[int64]bool)
	for _, uc := range existingCards {
		if uc.Amount > 0 {
			ownedCardIDs[uc.CardID] = true
		}
	}

	// Get all cards from the collection that user doesn't own (level < 4)
	var availableCards []*models.Card
	err = m.db.BunDB().NewSelect().
		Model(&availableCards).
		Where("col_id = ? AND level < 4", collection.ID).
		Scan(ctx)

	if err != nil {
		return "", false, fmt.Errorf("failed to get collection cards: %w", err)
	}

	// Filter out owned cards
	var uniqueCards []*models.Card
	for _, card := range availableCards {
		if !ownedCardIDs[card.ID] {
			uniqueCards = append(uniqueCards, card)
		}
	}

	if len(uniqueCards) == 0 {
		return fmt.Sprintf("Cannot fetch unique card from **%s** collection (you own them all!)", collection.Name), false, nil
	}

	// Select random card
	selectedCard := uniqueCards[rand.Intn(len(uniqueCards))]

	// Add card to user's inventory
	newUserCard := &models.UserCard{
		UserID: userID,
		CardID: selectedCard.ID,
		Amount: 1,
	}
	if err := m.userCardRepo.Create(ctx, newUserCard); err != nil {
		return "", false, fmt.Errorf("failed to add card to inventory: %w", err)
	}

	return fmt.Sprintf("You got **%s** (%s) from %s collection!", selectedCard.Name, strings.Repeat("⭐", selectedCard.Level), collection.Name), true, nil
}

// executeJudgeDay implements the judge day effect
func (m *Manager) executeJudgeDay(ctx context.Context, userID string, args string) (string, bool, error) {
	if args == "" {
		return "Please specify effect ID (e.g., 'claimrecall' or 'spaceunity -twice')", false, nil
	}

	// Parse effect ID and remaining args
	parts := strings.SplitN(args, " ", 2)
	effectID := parts[0]
	remainingArgs := ""
	if len(parts) > 1 {
		remainingArgs = parts[1]
	}

	// Check if effect exists and is active (not passive)
	staticEffect := GetEffectItemByID(effectID)
	if staticEffect == nil {
		return fmt.Sprintf("Effect '%s' not found or not usable", effectID), false, nil
	}

	if staticEffect.Passive {
		return fmt.Sprintf("Effect '%s' is passive and cannot be used with Judgment Day", effectID), false, nil
	}

	// Exclusion list (effects that cannot be used with judgeday)
	excludedEffects := []string{"judgeday", "walpurgisnight"}
	for _, excluded := range excludedEffects {
		if effectID == excluded {
			return "You cannot use that effect with Judgment Day", false, nil
		}
	}

	// Execute the target effect directly (bypass inventory check for judgeday)
	result, _, err := m.executeActiveEffect(ctx, userID, effectID, remainingArgs)
	if err != nil {
		return "", false, err
	}

	// Return with Judgment Day prefix
	return fmt.Sprintf("[Judgment Day] %s", result), true, nil
}

// GetEffectCooldown gets the cooldown end time for an effect
func (m *Manager) GetEffectCooldown(ctx context.Context, userID string, effectID string) (*time.Time, error) {
	return m.repo.GetEffectCooldown(ctx, userID, effectID)
}

// SetEffectCooldown sets the cooldown end time for an effect
func (m *Manager) SetEffectCooldown(ctx context.Context, userID string, effectID string, cooldownEnd time.Time) error {
	return m.repo.SetEffectCooldown(ctx, userID, effectID, cooldownEnd)
}
