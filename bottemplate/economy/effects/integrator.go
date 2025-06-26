package effects

import (
	"context"
	"log"
)

// GameIntegrator handles effect integration with game commands
type GameIntegrator struct {
	effectManager *Manager
}

// NewGameIntegrator creates a new effect integrator
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
		// Get effect definition to check if passive
		staticEffect := GetEffectItemByID(effect.EffectID)
		if staticEffect != nil && staticEffect.Passive {
			passiveEffects = append(passiveEffects, effect.EffectID)
		}
	}

	return passiveEffects, nil
}

// ApplyDailyEffects applies passive effects to daily rewards
func (gi *GameIntegrator) ApplyDailyEffects(ctx context.Context, userID string, baseReward int) int {
	effects, err := gi.GetActivePassiveEffects(ctx, userID)
	if err != nil {
		log.Printf("Warning: failed to get passive effects for daily: %v", err)
		return baseReward
	}

	reward := baseReward
	for _, effectID := range effects {
		switch effectID {
		case "cakeday":
			// Get user's claim count for today
			user, err := gi.effectManager.userRepo.GetByDiscordID(ctx, userID)
			if err == nil {
				claimBonus := user.DailyStats.Claims * 100
				reward += claimBonus
				log.Printf("Applied cakeday effect: +%d tomatoes (claims: %d)", claimBonus, user.DailyStats.Claims)
			}
		}
	}

	return reward
}

// ApplyClaimEffects applies passive effects to claim chances
func (gi *GameIntegrator) ApplyClaimEffects(ctx context.Context, userID string, baseChance float64) float64 {
	effects, err := gi.GetActivePassiveEffects(ctx, userID)
	if err != nil {
		log.Printf("Warning: failed to get passive effects for claim: %v", err)
		return baseChance
	}

	chance := baseChance
	for _, effectID := range effects {
		switch effectID {
		case "tohrugift":
			// Check if this is first claim of the day
			user, err := gi.effectManager.userRepo.GetByDiscordID(ctx, userID)
			if err == nil && user.DailyStats.Claims == 0 {
				// Increase 3-star chance for first claim
				chance *= 1.5 // 50% increase
				log.Printf("Applied tohrugift effect: 3-star chance increased to %.2f", chance)
			}
		}
	}

	return chance
}

// ApplyForgeDiscount applies passive effects to forge costs
func (gi *GameIntegrator) ApplyForgeDiscount(ctx context.Context, userID string, baseCost int) int {
	effects, err := gi.GetActivePassiveEffects(ctx, userID)
	if err != nil {
		log.Printf("Warning: failed to get passive effects for forge: %v", err)
		return baseCost
	}

	cost := baseCost
	for _, effectID := range effects {
		switch effectID {
		case "cherrybloss":
			// 50% discount on forge cost
			cost = cost / 2
			log.Printf("Applied cherrybloss effect: forge cost reduced from %d to %d", baseCost, cost)
		}
	}

	return cost
}

// ApplyLiquefyBonus applies passive effects to vial rewards
func (gi *GameIntegrator) ApplyLiquefyBonus(ctx context.Context, userID string, baseVials int64, cardLevel int) int64 {
	effects, err := gi.GetActivePassiveEffects(ctx, userID)
	if err != nil {
		log.Printf("Warning: failed to get passive effects for liquefy: %v", err)
		return baseVials
	}

	vials := baseVials
	for _, effectID := range effects {
		switch effectID {
		case "holygrail":
			// +25% vials for 1-2 star cards
			if cardLevel <= 2 {
				bonus := int64(float64(baseVials) * 0.25)
				vials += bonus
				log.Printf("Applied holygrail effect: +%d vials bonus for %d-star card", bonus, cardLevel)
			}
		}
	}

	return vials
}

// GetDailyCooldown returns modified daily cooldown based on passive effects
func (gi *GameIntegrator) GetDailyCooldown(ctx context.Context, userID string) int {
	effects, err := gi.GetActivePassiveEffects(ctx, userID)
	if err != nil {
		log.Printf("Warning: failed to get passive effects for daily cooldown: %v", err)
		return 20 // Default 20 hours
	}

	cooldown := 20 // Default hours
	for _, effectID := range effects {
		switch effectID {
		case "rulerjeanne":
			// Reduce daily cooldown to 17 hours
			cooldown = 17
			log.Printf("Applied rulerjeanne effect: daily cooldown reduced to %d hours", cooldown)
		}
	}

	return cooldown
}

// GetEffectCooldownReduction returns cooldown reduction for active effects
func (gi *GameIntegrator) GetEffectCooldownReduction(ctx context.Context, userID string) float64 {
	effects, err := gi.GetActivePassiveEffects(ctx, userID)
	if err != nil {
		return 0.0
	}

	reduction := 0.0
	for _, effectID := range effects {
		switch effectID {
		case "spellcard":
			// 40% cooldown reduction for usable effects
			reduction = 0.4
			log.Printf("Applied spellcard effect: 40%% cooldown reduction")
		}
	}

	return reduction
}