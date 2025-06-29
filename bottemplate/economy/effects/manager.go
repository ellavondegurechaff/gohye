package effects

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
)

// Manager is the modernized effect manager using the registry system
type Manager struct {
	registry       *EffectRegistry
	deps           *EffectDependencies
	repo           repositories.EffectRepository
	userRepo       repositories.UserRepository
	userCardRepo   repositories.UserCardRepository
	collectionRepo repositories.CollectionRepository
	cardRepo       repositories.CardRepository
	db             *database.DB

	// Metrics and monitoring
	metricsEnabled bool
	eventHandlers  []func(EffectEvent)
}

// NewManager creates a new modernized effect manager
func NewManager(
	repo repositories.EffectRepository,
	userRepo repositories.UserRepository,
	userCardRepo repositories.UserCardRepository,
	cardRepo repositories.CardRepository,
	collectionRepo repositories.CollectionRepository,
	db *database.DB,
) *Manager {
	deps := &EffectDependencies{
		UserRepo:       userRepo,
		CardRepo:       cardRepo,
		UserCardRepo:   userCardRepo,
		EffectRepo:     repo,
		CollectionRepo: collectionRepo,
		Database:       db.BunDB(),
	}

	registry := NewEffectRegistry(deps)

	manager := &Manager{
		registry:       registry,
		deps:           deps,
		repo:           repo,
		userRepo:       userRepo,
		userCardRepo:   userCardRepo,
		collectionRepo: collectionRepo,
		cardRepo:       cardRepo,
		db:             db,
		metricsEnabled: true,
		eventHandlers:  make([]func(EffectEvent), 0),
	}

	return manager
}

// RegisterEffect registers an effect handler with the manager
func (m *Manager) RegisterEffect(handler EffectHandler) error {
	return m.registry.RegisterEffect(handler)
}

// GetRegistry returns the effect registry for external registration
func (m *Manager) GetRegistry() *EffectRegistry {
	return m.registry
}

// GetDependencies returns the effect dependencies for external registration
func (m *Manager) GetDependencies() *EffectDependencies {
	return m.deps
}

// PurchaseEffect handles the purchase of an effect item (recipe)
func (m *Manager) PurchaseEffect(ctx context.Context, userID string, effectID string) error {
	// Get effect metadata to check if it exists
	_, err := m.registry.GetEffect(effectID)
	if err != nil {
		return fmt.Errorf("effect not found: %w", err)
	}

	// Get static effect item data for pricing
	staticItem := GetEffectItemByID(effectID)
	if staticItem == nil {
		return fmt.Errorf("effect item data not found: %s", effectID)
	}

	// Get user for balance checking
	user, err := m.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Check and deduct currency
	switch staticItem.Currency {
	case models.CurrencyTomato:
		if user.Balance < staticItem.Price {
			return fmt.Errorf("insufficient balance")
		}
		user.Balance -= staticItem.Price
	case models.CurrencyVials:
		if user.UserStats.Vials < staticItem.Price {
			return fmt.Errorf("insufficient vials")
		}
		user.UserStats.Vials -= staticItem.Price
	default:
		return fmt.Errorf("invalid currency")
	}

	// Update user balance
	if err := m.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user balance: %w", err)
	}

	// Store recipe for crafting
	if err := m.StoreRecipeForUser(ctx, userID, effectID); err != nil {
		return fmt.Errorf("failed to store recipe: %w", err)
	}

	slog.Info("Effect purchased",
		slog.String("user_id", userID),
		slog.String("effect_id", effectID),
		slog.Int64("price", staticItem.Price),
		slog.String("currency", staticItem.Currency))

	return nil
}

// CraftEffect crafts an effect from recipe cards
func (m *Manager) CraftEffect(ctx context.Context, userID string, effectID string) error {
	// Get effect handler
	handler, err := m.registry.GetEffect(effectID)
	if err != nil {
		return fmt.Errorf("effect not found: %w", err)
	}

	// Get static effect item data
	staticItem := GetEffectItemByID(effectID)
	if staticItem == nil {
		return fmt.Errorf("effect item data not found: %s", effectID)
	}

	if staticItem.Type != models.EffectTypeRecipe {
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

	// Create or update user effect
	if err := m.createUserEffect(ctx, userID, effectID, handler.GetMetadata()); err != nil {
		// Rollback: add cards back
		m.addCardsToUser(ctx, userID, recipe.CardIDs)
		return fmt.Errorf("failed to create user effect: %w", err)
	}

	// Remove the used recipe
	if err := m.repo.DeleteUserRecipe(ctx, userID, effectID); err != nil {
		slog.Warn("Failed to remove recipe after crafting", slog.Any("error", err))
	}

	slog.Info("Effect crafted successfully",
		slog.String("user_id", userID),
		slog.String("effect_id", effectID))

	return nil
}

// UseActiveEffect uses an active effect
func (m *Manager) UseActiveEffect(ctx context.Context, userID string, effectID string, args string) (string, error) {
	// Get active effect handler
	handler, err := m.registry.GetActiveEffect(effectID)
	if err != nil {
		return "", fmt.Errorf("active effect not found: %w", err)
	}

	// Check if user has this effect
	userEffect, err := m.repo.GetUserEffect(ctx, userID, effectID)
	if err != nil || userEffect == nil {
		return "", fmt.Errorf("effect not found in your collection")
	}

	// Check cooldown
	canExecute, reason := m.registry.CanExecuteEffect(ctx, effectID, userID)
	if !canExecute {
		return "", fmt.Errorf(reason)
	}

	// Check cooldown from database
	if cooldownEnds, err := m.repo.GetEffectCooldown(ctx, userID, effectID); err == nil && cooldownEnds != nil && time.Now().Before(*cooldownEnds) {
		remaining := time.Until(*cooldownEnds)
		return "", fmt.Errorf("effect is on cooldown for %v", remaining.Round(time.Minute))
	}

	// Execute the effect
	metadata := handler.GetMetadata()
	params := EffectParams{
		UserID:    userID,
		Arguments: args,
		Context:   make(map[string]interface{}),
		Metadata:  &metadata,
	}

	result, err := m.registry.ExecuteEffect(ctx, effectID, params)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return result.Message, nil
	}

	// Store card data in context if the effect returned card information
	if result.Data != nil {
		if resultDataMap, ok := ctx.Value("effect_result_data").(map[string]interface{}); ok {
			// Copy relevant card data to context for UI display
			for key, value := range result.Data {
				if key == "card_id" || key == "card_name" || key == "card_level" || key == "collection_id" {
					resultDataMap[key] = value
				}
			}
		}
	}

	// Effect was used successfully - decrease uses and handle lifecycle
	userEffect.Uses--
	userEffect.UpdatedAt = time.Now()
	
	// If no uses left, deactivate the effect
	if userEffect.Uses <= 0 {
		userEffect.Active = false
		slog.Info("Effect exhausted and deactivated",
			slog.String("user_id", userID),
			slog.String("effect_id", effectID))
	}

	// Update user effect
	if err := m.repo.UpdateUserEffect(ctx, userEffect); err != nil {
		slog.Error("Failed to update effect after use", slog.Any("error", err))
	}

	// Set cooldown if effect was used successfully
	metadata = handler.GetMetadata()
	if metadata.Cooldown > 0 {
		cooldownEnd := time.Now().Add(metadata.Cooldown)
		if err := m.repo.SetEffectCooldown(ctx, userID, effectID, cooldownEnd); err != nil {
			slog.Warn("Failed to set effect cooldown", slog.Any("error", err))
		}
	}

	// Emit events if any
	for _, event := range result.Events {
		m.emitEvent(event)
	}

	return result.Message, nil
}

// GetActiveUserEffects returns all active effects for a user (for integrator use)
func (m *Manager) GetActiveUserEffects(ctx context.Context, userID string) ([]*models.UserEffect, error) {
	return m.repo.GetActiveUserEffects(ctx, userID)
}

// DeactivateExpiredEffects deactivates expired effects (for integrator use)
func (m *Manager) DeactivateExpiredEffects(ctx context.Context) error {
	return m.repo.DeactivateExpiredEffects(ctx)
}

// ActivatePassiveEffect activates a passive effect for a user
func (m *Manager) ActivatePassiveEffect(ctx context.Context, userID string, effectID string) error {
	// Get the user effect
	userEffect, err := m.repo.GetUserEffect(ctx, userID, effectID)
	if err != nil {
		return fmt.Errorf("effect not found in user collection: %w", err)
	}

	// Get static effect data
	staticEffect := GetEffectItemByID(effectID)
	if staticEffect == nil {
		return fmt.Errorf("effect definition not found: %s", effectID)
	}

	if !staticEffect.Passive {
		return fmt.Errorf("cannot activate non-passive effect")
	}

	// Set activation timestamp and expiration
	now := time.Now()
	expiry := now.Add(time.Duration(staticEffect.Duration*24) * time.Hour) // Duration is in days for passive effects
	
	userEffect.Active = true
	userEffect.ExpiresAt = &expiry
	userEffect.Notified = false
	userEffect.UpdatedAt = now

	return m.repo.UpdateUserEffect(ctx, userEffect)
}

// DeactivatePassiveEffect deactivates a passive effect for a user
func (m *Manager) DeactivatePassiveEffect(ctx context.Context, userID string, effectID string) error {
	userEffect, err := m.repo.GetUserEffect(ctx, userID, effectID)
	if err != nil {
		return fmt.Errorf("effect not found: %w", err)
	}

	userEffect.Active = false
	userEffect.UpdatedAt = time.Now()

	return m.repo.UpdateUserEffect(ctx, userEffect)
}

// IsEffectExpired checks if an effect has expired
func (m *Manager) IsEffectExpired(ctx context.Context, userID string, effectID string) (bool, error) {
	userEffect, err := m.repo.GetUserEffect(ctx, userID, effectID)
	if err != nil {
		return true, err
	}

	// Check if passive effect has expired
	if userEffect.ExpiresAt != nil && time.Now().After(*userEffect.ExpiresAt) {
		return true, nil
	}

	// Check if active effect has no uses left
	staticEffect := GetEffectItemByID(effectID)
	if staticEffect != nil && !staticEffect.Passive && userEffect.Uses <= 0 {
		return true, nil
	}

	return false, nil
}

// RefreshEffectStatus refreshes the status of all user effects (expires/deactivates as needed)
func (m *Manager) RefreshEffectStatus(ctx context.Context, userID string) error {
	// Get all user effects
	effects, err := m.getAllUserEffects(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user effects: %w", err)
	}

	now := time.Now()
	var updatedEffects []*models.UserEffect

	for _, effect := range effects {
		needsUpdate := false

		// Check if passive effect has expired
		if effect.ExpiresAt != nil && now.After(*effect.ExpiresAt) && effect.Active {
			effect.Active = false
			needsUpdate = true
			slog.Info("Deactivated expired passive effect",
				slog.String("user_id", userID),
				slog.String("effect_id", effect.EffectID))
		}

		// Check if active effect has no uses left
		staticEffect := GetEffectItemByID(effect.EffectID)
		if staticEffect != nil && !staticEffect.Passive && effect.Uses <= 0 && effect.Active {
			effect.Active = false
			needsUpdate = true
			slog.Info("Deactivated exhausted active effect",
				slog.String("user_id", userID),
				slog.String("effect_id", effect.EffectID))
		}

		if needsUpdate {
			effect.UpdatedAt = now
			updatedEffects = append(updatedEffects, effect)
		}
	}

	// Update all modified effects
	for _, effect := range updatedEffects {
		if err := m.repo.UpdateUserEffect(ctx, effect); err != nil {
			slog.Error("Failed to update effect status",
				slog.String("effect_id", effect.EffectID),
				slog.Any("error", err))
		}
	}

	return nil
}

// ListUserEffects returns all effects in user's inventory
func (m *Manager) ListUserEffects(ctx context.Context, userID string) ([]*models.EffectItem, error) {
	items := make([]*models.EffectItem, 0)

	// Get user's crafted effects
	userEffects, err := m.getAllUserEffects(ctx, userID)
	if err != nil {
		slog.Error("Failed to get user effects", slog.Any("error", err))
	} else {
		for _, userEffect := range userEffects {
			staticItem := GetEffectItemByID(userEffect.EffectID)
			if staticItem == nil {
				continue
			}
			effectItem := staticItem.ToEffectItem()

			// Set appropriate type based on effect properties
			if staticItem.Passive {
				effectItem.Type = models.EffectTypePassive
			} else {
				effectItem.Type = models.EffectTypeActive
			}

			items = append(items, effectItem)
		}
	}

	// Get user's purchased recipes
	recipes, err := m.getUserRecipes(ctx, userID)
	if err != nil {
		slog.Error("Failed to get user recipes", slog.Any("error", err))
	} else {
		for _, recipe := range recipes {
			staticItem := GetEffectItemByID(recipe.ItemID)
			if staticItem == nil {
				continue
			}
			effectItem := staticItem.ToEffectItem()
			effectItem.Type = models.EffectTypeRecipe
			items = append(items, effectItem)
		}
	}

	return items, nil
}

// GetEffectStats returns statistics for an effect
func (m *Manager) GetEffectStats(effectID string) (*EffectExecutionStats, error) {
	return m.registry.GetExecutionStats(effectID)
}

// GetAllEffectStats returns statistics for all effects
func (m *Manager) GetAllEffectStats() map[string]*EffectExecutionStats {
	return m.registry.GetAllExecutionStats()
}

// AddEventHandler adds an event handler for effect events
func (m *Manager) AddEventHandler(handler func(EffectEvent)) {
	m.eventHandlers = append(m.eventHandlers, handler)
}

// emitEvent emits an effect event to all registered handlers
func (m *Manager) emitEvent(event EffectEvent) {
	if !m.metricsEnabled {
		return
	}

	for _, handler := range m.eventHandlers {
		go handler(event)
	}
}

// Shutdown gracefully shuts down the effect manager
func (m *Manager) Shutdown(ctx context.Context) error {
	return m.registry.Shutdown(ctx)
}

// Helper methods (keeping existing logic for now)

// createUserEffect creates a new user effect entry
func (m *Manager) createUserEffect(ctx context.Context, userID string, effectID string, metadata EffectMetadata) error {
	staticEffect := GetEffectItemByID(effectID)
	if staticEffect == nil {
		return fmt.Errorf("effect definition not found: %s", effectID)
	}

	// Check if user already has this effect
	existingEffect, err := m.repo.GetUserEffect(ctx, userID, effectID)
	if err == nil && existingEffect != nil {
		// User already has this effect - extend uses/duration
		if staticEffect.Passive {
			// For passive effects, extend duration if already active
			if existingEffect.ExpiresAt != nil {
				newExpiry := existingEffect.ExpiresAt.Add(time.Duration(staticEffect.Duration) * time.Hour)
				existingEffect.ExpiresAt = &newExpiry
				existingEffect.Notified = false
			} else {
				// First time activation
				expiry := time.Now().Add(time.Duration(staticEffect.Duration) * time.Hour)
				existingEffect.ExpiresAt = &expiry
				existingEffect.Active = true
				existingEffect.Notified = false
			}
		} else {
			// For active effects, add more uses
			existingEffect.Uses += staticEffect.Duration
		}

		return m.repo.UpdateUserEffect(ctx, existingEffect)
	}

	// Create new effect entry
	userEffect := &models.UserEffect{
		UserID:      userID,
		EffectID:    effectID,
		IsRecipe:    false,
		Active:      !staticEffect.Passive,
		Uses:        0,
		Notified:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if staticEffect.Passive {
		// Passive effects start inactive and need to be manually activated
		userEffect.Active = false
		userEffect.Uses = 0
		// Don't set expiry until activated
	} else {
		// Active effects start active with their use count
		userEffect.Active = true
		userEffect.Uses = staticEffect.Duration // Duration = number of uses for active effects
		zeroTime := time.Time{}
		userEffect.CooldownEndsAt = &zeroTime
	}

	return m.repo.AddUserEffect(ctx, userEffect)
}

// StoreRecipeForUser stores a recipe for a user
func (m *Manager) StoreRecipeForUser(ctx context.Context, userID string, effectID string) error {
	staticItem := GetEffectItemByID(effectID)
	if staticItem == nil {
		return fmt.Errorf("effect not found: %s", effectID)
	}

	var cardIDs []int64
	for _, stars := range staticItem.Recipe {
		card, err := m.repo.GetRandomCardForRecipe(ctx, userID, stars)
		if err != nil {
			return fmt.Errorf("failed to get recipe card: %w", err)
		}
		if card == nil {
			return fmt.Errorf("no available cards found for %d stars", stars)
		}
		cardIDs = append(cardIDs, card.ID)
	}

	return m.repo.StoreUserRecipe(ctx, userID, effectID, cardIDs)
}

// verifyUserHasRecipeCards checks if user owns all required recipe cards
func (m *Manager) verifyUserHasRecipeCards(ctx context.Context, userID string, cardIDs []int64) error {
	for _, cardID := range cardIDs {
		userCard, err := m.userCardRepo.GetByUserIDAndCardID(ctx, userID, cardID)
		if err != nil || userCard == nil || userCard.Amount <= 0 {
			card, _ := m.repo.GetCard(ctx, cardID)
			cardName := "Unknown Card"
			if card != nil {
				cardName = card.Name
			}
			return fmt.Errorf("missing card: %s (ID: %d)", cardName, cardID)
		}
	}
	return nil
}

// removeCardsFromUser removes specific cards from user's collection
func (m *Manager) removeCardsFromUser(ctx context.Context, userID string, cardIDs []int64) error {
	for _, cardID := range cardIDs {
		userCard, err := m.userCardRepo.GetByUserIDAndCardID(ctx, userID, cardID)
		if err != nil || userCard == nil || userCard.Amount <= 0 {
			return fmt.Errorf("cannot remove card that user doesn't own (ID: %d)", cardID)
		}

		userCard.Amount--
		if err := m.userCardRepo.Update(ctx, userCard); err != nil {
			return fmt.Errorf("failed to update user card: %w", err)
		}
	}
	return nil
}

// addCardsToUser adds cards back to user (for rollback)
func (m *Manager) addCardsToUser(ctx context.Context, userID string, cardIDs []int64) error {
	for _, cardID := range cardIDs {
		userCard, err := m.userCardRepo.GetByUserIDAndCardID(ctx, userID, cardID)
		if err != nil {
			userCard = &models.UserCard{
				UserID: userID,
				CardID: cardID,
				Amount: 1,
			}
			if err := m.userCardRepo.Create(ctx, userCard); err != nil {
				slog.Error("Failed to rollback card creation", slog.Any("error", err))
			}
		} else if userCard != nil {
			userCard.Amount++
			if err := m.userCardRepo.Update(ctx, userCard); err != nil {
				slog.Error("Failed to rollback card amount increase", slog.Any("error", err))
			}
		}
	}
	return nil
}

// getAllUserEffects gets all user effects
func (m *Manager) getAllUserEffects(ctx context.Context, userID string) ([]*models.UserEffect, error) {
	var effects []*models.UserEffect
	err := m.db.BunDB().NewSelect().
		Model(&effects).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Scan(ctx)
	return effects, err
}

// getUserRecipes gets all recipes owned by a user
func (m *Manager) getUserRecipes(ctx context.Context, userID string) ([]*models.UserRecipe, error) {
	var recipes []*models.UserRecipe
	err := m.db.BunDB().NewSelect().
		Model(&recipes).
		Where("user_id = ?", userID).
		Scan(ctx)
	return recipes, err
}

// Legacy compatibility methods

// GetEffectItem returns a specific effect item by ID (for compatibility)
func (m *Manager) GetEffectItem(ctx context.Context, effectID string) (*models.EffectItem, error) {
	staticItem := GetEffectItemByID(effectID)
	if staticItem == nil {
		return nil, fmt.Errorf("effect item not found: %s", effectID)
	}
	return staticItem.ToEffectItem(), nil
}

// ListEffectItems returns all available effect items (for compatibility)
func (m *Manager) ListEffectItems(ctx context.Context) ([]*models.EffectItem, error) {
	var items []*models.EffectItem
	for i := range StaticEffectItems {
		items = append(items, StaticEffectItems[i].ToEffectItem())
	}
	return items, nil
}

// GetUserRecipeStatus returns the recipe status for a user effect (for compatibility)
func (m *Manager) GetUserRecipeStatus(ctx context.Context, userID string, effectID string) ([]*models.Card, error) {
	recipe, err := m.repo.GetUserRecipe(ctx, userID, effectID)
	if err != nil {
		return nil, fmt.Errorf("no recipe found for this effect")
	}

	cards := make([]*models.Card, len(recipe.CardIDs))
	for i, cardID := range recipe.CardIDs {
		card, err := m.repo.GetCard(ctx, cardID)
		if err != nil {
			slog.Error("Failed to get card", slog.Int64("card_id", cardID), slog.Any("error", err))
			continue
		}
		cards[i] = card
	}

	return cards, nil
}