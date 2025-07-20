package effects

import (
	"context"
	"fmt"
	"time"

	"log/slog"
)

// GameIntegrator handles effect integration with game commands using the new system
type GameIntegrator struct {
	effectManager *Manager
}

// NewGameIntegrator creates a new effect integrator using the modern system
func NewGameIntegrator(effectManager *Manager) *GameIntegrator {
	return &GameIntegrator{
		effectManager: effectManager,
	}
}

// GetActivePassiveEffects returns list of active passive effect IDs for a user
func (gi *GameIntegrator) GetActivePassiveEffects(ctx context.Context, userID string) ([]string, error) {
	activeEffects, err := gi.effectManager.repo.GetActiveUserEffects(ctx, userID)
	if err != nil {
		return nil, err
	}

	var passiveEffects []string
	for _, effect := range activeEffects {
		// Get effect handler to check if passive
		handler, err := gi.effectManager.registry.GetEffect(effect.EffectID)
		if err != nil {
			continue
		}

		metadata := handler.GetMetadata()
		if metadata.Type == EffectTypePassive {
			passiveEffects = append(passiveEffects, effect.EffectID)
		}
	}

	return passiveEffects, nil
}

// ApplyDailyEffects applies passive effects to daily rewards
func (gi *GameIntegrator) ApplyDailyEffects(ctx context.Context, userID string, baseReward int) int {
	result := gi.applyPassiveEffectWithFeedback(ctx, userID, "daily_reward", baseReward)

	modifiedReward, ok := result.GetValue().(int)
	if !ok {
		slog.Warn("Invalid result type from passive effect application",
			slog.String("user_id", userID),
			slog.String("action", "daily_reward"))
		return baseReward
	}

	return modifiedReward
}

// ApplyDailyEffectsWithFeedback applies passive effects to daily rewards and returns feedback
func (gi *GameIntegrator) ApplyDailyEffectsWithFeedback(ctx context.Context, userID string, baseReward int) *EffectApplicationResult {
	return gi.applyPassiveEffectWithFeedback(ctx, userID, "daily_reward", baseReward)
}

// ApplyClaimEffects applies passive effects to claim chances
func (gi *GameIntegrator) ApplyClaimEffects(ctx context.Context, userID string, baseChance float64) float64 {
	result := gi.applyPassiveEffectWithFeedback(ctx, userID, "claim_3star_chance", baseChance)

	modifiedChance, ok := result.GetValue().(float64)
	if !ok {
		slog.Warn("Invalid result type from passive effect application",
			slog.String("user_id", userID),
			slog.String("action", "claim_3star_chance"))
		return baseChance
	}

	return modifiedChance
}

// ApplyClaimEffectsWithFeedback applies passive effects to claim chances and returns feedback
func (gi *GameIntegrator) ApplyClaimEffectsWithFeedback(ctx context.Context, userID string, baseChance float64) *EffectApplicationResult {
	return gi.applyPassiveEffectWithFeedback(ctx, userID, "claim_3star_chance", baseChance)
}

// ApplyForgeDiscount applies passive effects to forge costs
func (gi *GameIntegrator) ApplyForgeDiscount(ctx context.Context, userID string, baseCost int) int {
	result := gi.applyPassiveEffectWithFeedback(ctx, userID, "forge_cost", baseCost)

	modifiedCost, ok := result.GetValue().(int)
	if !ok {
		slog.Warn("Invalid result type from passive effect application",
			slog.String("user_id", userID),
			slog.String("action", "forge_cost"))
		return baseCost
	}

	return modifiedCost
}

// ApplyForgeDiscountWithFeedback applies passive effects to forge costs and returns feedback
func (gi *GameIntegrator) ApplyForgeDiscountWithFeedback(ctx context.Context, userID string, baseCost int) *EffectApplicationResult {
	return gi.applyPassiveEffectWithFeedback(ctx, userID, "forge_cost", baseCost)
}

// ApplyLiquefyBonus applies passive effects to vial rewards
func (gi *GameIntegrator) ApplyLiquefyBonus(ctx context.Context, userID string, baseVials int64, cardLevel int) int64 {
	data := map[string]interface{}{
		"vials":      baseVials,
		"card_level": cardLevel,
	}

	result, err := gi.applyPassiveEffect(ctx, userID, "vial_reward", data)
	if err != nil {
		slog.Warn("Failed to apply passive effects to vial reward",
			slog.String("user_id", userID),
			slog.Int64("base_vials", baseVials),
			slog.Int("card_level", cardLevel),
			slog.Any("error", err))
		return baseVials
	}

	resultData, ok := result.(map[string]interface{})
	if !ok {
		slog.Warn("Invalid result type from passive effect application",
			slog.String("user_id", userID),
			slog.String("action", "vial_reward"))
		return baseVials
	}

	modifiedVials, ok := resultData["vials"].(int64)
	if !ok {
		slog.Warn("Invalid vials type in result data",
			slog.String("user_id", userID),
			slog.String("action", "vial_reward"))
		return baseVials
	}

	return modifiedVials
}

// GetDailyCooldown returns modified daily cooldown based on passive effects
func (gi *GameIntegrator) GetDailyCooldown(ctx context.Context, userID string) int {
	baseHours := 20

	result, err := gi.applyPassiveEffect(ctx, userID, "daily_cooldown", baseHours)
	if err != nil {
		slog.Warn("Failed to apply passive effects to daily cooldown",
			slog.String("user_id", userID),
			slog.Int("base_hours", baseHours),
			slog.Any("error", err))
		return baseHours
	}

	modifiedHours, ok := result.(int)
	if !ok {
		slog.Warn("Invalid result type from passive effect application",
			slog.String("user_id", userID),
			slog.String("action", "daily_cooldown"))
		return baseHours
	}

	return modifiedHours
}

// GetEffectCooldownReduction returns cooldown reduction for active effects
func (gi *GameIntegrator) GetEffectCooldownReduction(ctx context.Context, userID string) float64 {
	baseReduction := 0.0

	result, err := gi.applyPassiveEffect(ctx, userID, "effect_cooldown_reduction", baseReduction)
	if err != nil {
		slog.Warn("Failed to apply passive effects to effect cooldown reduction",
			slog.String("user_id", userID),
			slog.Float64("base_reduction", baseReduction),
			slog.Any("error", err))
		return baseReduction
	}

	modifiedReduction, ok := result.(float64)
	if !ok {
		slog.Warn("Invalid result type from passive effect application",
			slog.String("user_id", userID),
			slog.String("action", "effect_cooldown_reduction"))
		return baseReduction
	}

	return modifiedReduction
}

// GetEffectRemainingTime gets the remaining time for an active effect
func (gi *GameIntegrator) GetEffectRemainingTime(ctx context.Context, userID string, effectID string) (time.Duration, error) {
	userEffect, err := gi.effectManager.repo.GetUserEffect(ctx, userID, effectID)
	if err != nil {
		return 0, err
	}

	if !userEffect.Active {
		return 0, nil
	}

	// For passive effects with expiration
	if userEffect.ExpiresAt != nil {
		if time.Now().After(*userEffect.ExpiresAt) {
			return 0, nil // Expired
		}
		return time.Until(*userEffect.ExpiresAt), nil
	}

	// For active effects (use-based), no time limit
	return time.Duration(0), nil
}

// GetUserEffectStatus returns detailed status for a user's effect
func (gi *GameIntegrator) GetUserEffectStatus(ctx context.Context, userID string, effectID string) (map[string]interface{}, error) {
	userEffect, err := gi.effectManager.repo.GetUserEffect(ctx, userID, effectID)
	if err != nil {
		return nil, err
	}

	staticEffect := GetEffectItemByID(effectID)
	if staticEffect == nil {
		return nil, fmt.Errorf("effect definition not found")
	}

	status := map[string]interface{}{
		"effect_id":   effectID,
		"name":        staticEffect.Name,
		"description": staticEffect.Description,
		"active":      userEffect.Active,
		"passive":     staticEffect.Passive,
		"uses":        userEffect.Uses,
		"max_uses":    staticEffect.Duration, // Duration represents max uses for active effects
	}

	if userEffect.ExpiresAt != nil {
		status["expires_at"] = *userEffect.ExpiresAt
		if time.Now().After(*userEffect.ExpiresAt) {
			status["expired"] = true
		} else {
			status["remaining_time"] = time.Until(*userEffect.ExpiresAt)
		}
	}

	if userEffect.CooldownEndsAt != nil && time.Now().Before(*userEffect.CooldownEndsAt) {
		status["cooldown_remaining"] = time.Until(*userEffect.CooldownEndsAt)
	}

	return status, nil
}

// ValidateEffectUsage validates if an effect can be used
func (gi *GameIntegrator) ValidateEffectUsage(ctx context.Context, userID string, effectID string, args string) error {
	// Check if effect exists
	handler, err := gi.effectManager.GetRegistry().GetEffect(effectID)
	if err != nil {
		return fmt.Errorf("effect not found: %w", err)
	}

	// Validate parameters
	metadata := handler.GetMetadata()
	params := EffectParams{
		UserID:    userID,
		Arguments: args,
		Context:   make(map[string]interface{}),
		Metadata:  &metadata,
	}

	validationErrors := gi.effectManager.registry.ValidateEffect(ctx, effectID, params)
	if len(validationErrors) > 0 {
		return fmt.Errorf("validation failed: %s", validationErrors[0].Message)
	}

	// Check if can execute
	canExecute, reason := gi.effectManager.registry.CanExecuteEffect(ctx, effectID, userID)
	if !canExecute {
		return fmt.Errorf("cannot execute effect: %s", reason)
	}

	return nil
}

// GetEffectMetadata returns metadata for an effect
func (gi *GameIntegrator) GetEffectMetadata(effectID string) (*EffectMetadata, error) {
	handler, err := gi.effectManager.GetRegistry().GetEffect(effectID)
	if err != nil {
		return nil, err
	}

	metadata := handler.GetMetadata()
	return &metadata, nil
}

// ListAvailableEffects returns all available effects by category
func (gi *GameIntegrator) ListAvailableEffects(category EffectCategory) []EffectMetadata {
	effectIDs := gi.effectManager.GetRegistry().ListEffectsByCategory(category)
	var effects []EffectMetadata

	for _, effectID := range effectIDs {
		if handler, err := gi.effectManager.GetRegistry().GetEffect(effectID); err == nil {
			effects = append(effects, handler.GetMetadata())
		}
	}

	return effects
}

// EstimateEffectCost estimates the cost/impact of using an effect
func (gi *GameIntegrator) EstimateEffectCost(ctx context.Context, userID string, effectID string, args string) (map[string]interface{}, error) {
	handler, err := gi.effectManager.GetRegistry().GetEffect(effectID)
	if err != nil {
		return nil, err
	}

	metadata := handler.GetMetadata()
	params := EffectParams{
		UserID:    userID,
		Arguments: args,
		Context:   make(map[string]interface{}),
		Metadata:  &metadata,
	}

	return handler.EstimateCost(ctx, params)
}

// GetEffectUsageStats returns usage statistics for effects (delegated to manager)
func (gi *GameIntegrator) GetEffectUsageStats() map[string]*EffectExecutionStats {
	return gi.effectManager.GetAllEffectStats()
}

// RegisterEventHandler registers a handler for effect events (delegated to manager)
func (gi *GameIntegrator) RegisterEventHandler(handler func(EffectEvent)) {
	gi.effectManager.AddEventHandler(handler)
}

// IsEffectActive checks if a specific effect is active for a user
func (gi *GameIntegrator) IsEffectActive(ctx context.Context, userID string, effectID string) (bool, error) {
	handler, err := gi.effectManager.GetRegistry().GetPassiveEffect(effectID)
	if err != nil {
		// Not a passive effect, check if user has it as active effect
		userEffect, err := gi.effectManager.GetActiveUserEffects(ctx, userID)
		if err != nil {
			return false, nil
		}

		for _, effect := range userEffect {
			if effect.EffectID == effectID && effect.Active {
				return true, nil
			}
		}
		return false, nil
	}

	return handler.IsActive(ctx, userID)
}

// GetPassiveEffectModifier gets the modifier value for a passive effect
func (gi *GameIntegrator) GetPassiveEffectModifier(ctx context.Context, userID string, effectID string, action string) (float64, error) {
	handler, err := gi.effectManager.GetRegistry().GetPassiveEffect(effectID)
	if err != nil {
		return 1.0, err
	}

	return handler.GetModifier(ctx, userID, action)
}

// RefreshEffects refreshes all active effects for a user (cleanup expired, etc.)
func (gi *GameIntegrator) RefreshEffects(ctx context.Context, userID string) error {
	// Refresh individual user's effect status
	if err := gi.effectManager.RefreshEffectStatus(ctx, userID); err != nil {
		slog.Warn("Failed to refresh user effect status",
			slog.String("user_id", userID),
			slog.Any("error", err))
	}

	// Also run global expired effect cleanup
	if err := gi.effectManager.DeactivateExpiredEffects(ctx); err != nil {
		slog.Warn("Failed to deactivate expired effects", slog.Any("error", err))
	}

	slog.Info("Effects refreshed for user", slog.String("user_id", userID))
	return nil
}

// ActivatePassiveEffect allows users to activate a passive effect they own
func (gi *GameIntegrator) ActivatePassiveEffect(ctx context.Context, userID string, effectID string) error {
	return gi.effectManager.ActivatePassiveEffect(ctx, userID, effectID)
}

// DeactivatePassiveEffect allows users to deactivate a passive effect
func (gi *GameIntegrator) DeactivatePassiveEffect(ctx context.Context, userID string, effectID string) error {
	return gi.effectManager.DeactivatePassiveEffect(ctx, userID, effectID)
}

// IsEffectExpired checks if a user's effect has expired
func (gi *GameIntegrator) IsEffectExpired(ctx context.Context, userID string, effectID string) (bool, error) {
	return gi.effectManager.IsEffectExpired(ctx, userID, effectID)
}

// GetEffectsByType returns effects of a specific type for a user
func (gi *GameIntegrator) GetEffectsByType(ctx context.Context, userID string, effectType EffectType) ([]string, error) {
	var effectIDs []string

	switch effectType {
	case EffectTypeActive:
		effectIDs = gi.effectManager.GetRegistry().ListActiveEffects()
	case EffectTypePassive:
		effectIDs = gi.effectManager.GetRegistry().ListPassiveEffects()
	default:
		return nil, fmt.Errorf("unknown effect type: %s", effectType)
	}

	// Filter by user ownership
	var userEffects []string
	for _, effectID := range effectIDs {
		if active, err := gi.IsEffectActive(ctx, userID, effectID); err == nil && active {
			userEffects = append(userEffects, effectID)
		}
	}

	return userEffects, nil
}

// Shutdown gracefully shuts down the integrator
func (gi *GameIntegrator) Shutdown(ctx context.Context) error {
	return gi.effectManager.Shutdown(ctx)
}

// applyPassiveEffect applies passive effects to a game action (internal method)
func (gi *GameIntegrator) applyPassiveEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	// Get all active passive effects for user
	activeEffects, err := gi.effectManager.GetActiveUserEffects(ctx, userID)
	if err != nil {
		return baseValue, err
	}

	result := baseValue
	for _, userEffect := range activeEffects {
		// Get passive handler
		handler, err := gi.effectManager.GetRegistry().GetPassiveEffect(userEffect.EffectID)
		if err != nil {
			continue // Skip if not a passive effect
		}

		// Check if effect is still active
		if active, err := handler.IsActive(ctx, userID); err != nil || !active {
			continue
		}

		// Apply the effect
		result, err = handler.ApplyEffect(ctx, userID, action, result)
		if err != nil {
			slog.Warn("Failed to apply passive effect",
				slog.String("effect_id", userEffect.EffectID),
				slog.String("user_id", userID),
				slog.String("action", action),
				slog.Any("error", err))
			continue
		}
	}

	return result, nil
}

// applyPassiveEffectWithFeedback applies passive effects and returns detailed feedback
func (gi *GameIntegrator) applyPassiveEffectWithFeedback(ctx context.Context, userID string, action string, baseValue interface{}) *EffectApplicationResult {
	result := NewEffectApplicationResult(baseValue)

	// Get all active passive effects for user
	activeEffects, err := gi.effectManager.GetActiveUserEffects(ctx, userID)
	if err != nil {
		slog.Warn("Failed to get active user effects",
			slog.String("user_id", userID),
			slog.String("action", action),
			slog.Any("error", err))
		return result
	}

	currentValue := baseValue
	for _, userEffect := range activeEffects {
		// Get passive handler
		handler, err := gi.effectManager.GetRegistry().GetPassiveEffect(userEffect.EffectID)
		if err != nil {
			continue // Skip if not a passive effect
		}

		// Check if effect is still active
		if active, err := handler.IsActive(ctx, userID); err != nil || !active {
			continue
		}

		// Get effect metadata for feedback
		metadata := handler.GetMetadata()
		staticEffect := GetEffectItemByID(userEffect.EffectID)
		effectName := metadata.Name
		if staticEffect != nil {
			effectName = staticEffect.Name
		}

		// Store original value before applying effect
		originalValue := currentValue

		// Apply the effect
		currentValue, err = handler.ApplyEffect(ctx, userID, action, currentValue)
		if err != nil {
			slog.Warn("Failed to apply passive effect",
				slog.String("effect_id", userEffect.EffectID),
				slog.String("user_id", userID),
				slog.String("action", action),
				slog.Any("error", err))
			continue
		}

		// Check if effect actually modified the value
		if currentValue != originalValue {
			// Get modifier for feedback
			modifier, _ := handler.GetModifier(ctx, userID, action)

			// Add to applied effects list
			result.AddAppliedEffect(
				userEffect.EffectID,
				effectName,
				metadata.Description,
				action,
				"🛡️", // Passive effect emoji
				modifier,
			)
		}
	}

	result.SetModifiedValue(currentValue)
	return result
}
