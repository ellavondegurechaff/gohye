package effects

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/disgoorg/bot-template/internal/gateways/database/models"
	"github.com/disgoorg/bot-template/internal/gateways/database/repositories"
)

type Manager struct {
	repo     repositories.EffectRepository
	userRepo repositories.UserRepository
}

func NewManager(repo repositories.EffectRepository, userRepo repositories.UserRepository) *Manager {
	return &Manager{
		repo:     repo,
		userRepo: userRepo,
	}
}

// PurchaseEffect handles the purchase of an effect item
func (m *Manager) PurchaseEffect(ctx context.Context, userID string, effectID string) error {
	// Get effect item
	item, err := m.repo.GetEffectItem(ctx, effectID)
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

	// Add to inventory
	if err := m.repo.AddToInventory(ctx, userID, effectID, 1); err != nil {
		return fmt.Errorf("failed to add to inventory: %w", err)
	}

	// Update user stats
	if err := m.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user stats: %w", err)
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
	userEffect := &models.UserEffect{
		UserID:    userID,
		EffectID:  effectID,
		Active:    true,
		ExpiresAt: time.Now().Add(time.Duration(effect.Duration) * time.Hour),
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

// ListEffectItems returns all available effect items
func (m *Manager) ListEffectItems(ctx context.Context) ([]*models.EffectItem, error) {
	return m.repo.ListEffectItems(ctx)
}

// GetEffectItem returns a specific effect item by ID
func (m *Manager) GetEffectItem(ctx context.Context, effectID string) (*models.EffectItem, error) {
	return m.repo.GetEffectItem(ctx, effectID)
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
	// Get effect item
	item, err := m.repo.GetEffectItem(ctx, effectID)
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
