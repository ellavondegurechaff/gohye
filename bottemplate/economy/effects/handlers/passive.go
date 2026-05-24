package handlers

import (
	"context"
	"fmt"

	"log/slog"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/uptrace/bun"
)

func currentTierValue(ctx context.Context, deps *effects.EffectDependencies, userID, effectID string) (value int, tier int, ok bool) {
	effectRepo, ok := deps.EffectRepo.(repositories.EffectRepository)
	if !ok {
		return 0, 0, false
	}

	userEffect, err := effectRepo.GetUserEffect(ctx, userID, effectID)
	if err != nil || userEffect == nil || userEffect.IsRecipe || !userEffect.Active {
		return 0, 0, false
	}

	effectData := effects.GetEffectItemByID(effectID)
	if effectData == nil || effectData.TierData == nil {
		return 0, 0, false
	}

	tierIndex := userEffect.Tier - 1
	if tierIndex < 0 || tierIndex >= len(effectData.TierData.Values) {
		return 0, 0, false
	}

	return effectData.TierData.Values[tierIndex], userEffect.Tier, true
}

// TohrugiftHandler implements the "Gift From Tohru" passive effect
type TohrugiftHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewTohrugiftHandler creates a new Tohrugift effect handler
func NewTohrugiftHandler(deps *effects.EffectDependencies) *TohrugiftHandler {
	metadata := effects.EffectMetadata{
		ID:          "tohrugift",
		Name:        "Gift From Tohru",
		Description: "Increase chances of getting a 3-star card every first claim per day",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryClaim,
		Cooldown:    0,
		MaxUses:     -1, // Permanent passive effect
		Animated:    false,
		Tags:        []string{"passive", "claim", "first_daily"},
		Version:     "1.0.0",
	}

	return &TohrugiftHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects just returns success (actual logic is in ApplyEffect)
func (h *TohrugiftHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Tohrugift passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the 3-star claim bonus for first claim of the day
func (h *TohrugiftHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "claim_3star_chance" {
		return baseValue, nil
	}

	user, err := h.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return baseValue, err
	}

	// Only apply on first claim of the day
	if user.DailyStats.Claims != 0 {
		return baseValue, nil
	}

	baseChance, ok := baseValue.(float64)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for 3-star chance")
	}

	// Increase 3-star chance by 50% for first claim
	modifiedChance := baseChance * 1.5

	slog.Info("Applied Tohrugift effect",
		slog.String("user_id", userID),
		slog.Float64("base_chance", baseChance),
		slog.Float64("modified_chance", modifiedChance))

	return modifiedChance, nil
}

// IsActive checks if the effect is currently active for the user
func (h *TohrugiftHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	// Check if user has this passive effect active
	// This would typically check the user_effects table
	return true, nil // Simplified for now
}

// GetModifier returns the modifier value (1.5x for first claim)
func (h *TohrugiftHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "claim_3star_chance" {
		return 1.0, nil
	}

	user, err := h.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return 1.0, err
	}

	// Only apply modifier on first claim
	if user.DailyStats.Claims == 0 {
		return 1.5, nil
	}

	return 1.0, nil
}

// CakedayHandler implements the "Cake Day" passive effect
type CakedayHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
	db       *bun.DB
}

// NewCakedayHandler creates a new Cakeday effect handler
func NewCakedayHandler(deps *effects.EffectDependencies) *CakedayHandler {
	metadata := effects.EffectMetadata{
		ID:          "cakeday",
		Name:        "Cake Day",
		Description: "Get extra flakes per daily for every claim you did",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryDaily,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    true,
		Tags:        []string{"passive", "daily", "snowflakes"},
		Version:     "1.0.0",
	}

	return &CakedayHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
		db:                deps.Database.(*bun.DB),
	}
}

func (h *CakedayHandler) getDailyClaims(ctx context.Context, userID string) int {
	var stats models.ClaimStats
	err := h.db.NewSelect().
		Model(&stats).
		Column("daily_claims").
		Where("user_id = ?", userID).
		Scan(ctx)
	if err != nil {
		return 0
	}
	return stats.DailyClaims
}

// Execute for passive effects
func (h *CakedayHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Cakeday passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the daily snowflake bonus based on claims
func (h *CakedayHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "daily_reward" {
		return baseValue, nil
	}

	baseReward, ok := baseValue.(int)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for daily reward")
	}

	flakesPerClaim, tier, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "cakeday")
	if !ok {
		return baseValue, nil
	}

	dailyClaims := h.getDailyClaims(ctx, userID)
	bonus := dailyClaims * flakesPerClaim
	modifiedReward := baseReward + bonus

	slog.Info("Applied Cakeday effect",
		slog.String("user_id", userID),
		slog.Int("base_reward", baseReward),
		slog.Int("claims_today", dailyClaims),
		slog.Int("tier", tier),
		slog.Int("flakes_per_claim", flakesPerClaim),
		slog.Int("bonus", bonus),
		slog.Int("modified_reward", modifiedReward))

	return modifiedReward, nil
}

// IsActive checks if the effect is currently active
func (h *CakedayHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the bonus amount (claims * 100)
func (h *CakedayHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "daily_reward" {
		return 0.0, nil
	}

	flakesPerClaim, _, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "cakeday")
	if !ok {
		return 0.0, nil
	}
	return float64(h.getDailyClaims(ctx, userID) * flakesPerClaim), nil
}

// HolygrailHandler implements the "The Holy Grail" passive effect
type HolygrailHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewHolygrailHandler creates a new Holygrail effect handler
func NewHolygrailHandler(deps *effects.EffectDependencies) *HolygrailHandler {
	metadata := effects.EffectMetadata{
		ID:          "holygrail",
		Name:        "The Holy Grail",
		Description: "Get extra vials per liquify",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryEconomy,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    true,
		Tags:        []string{"passive", "vials", "liquefy"},
		Version:     "1.0.0",
	}

	return &HolygrailHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects
func (h *HolygrailHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Holygrail passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the vial bonus for low-level cards
func (h *HolygrailHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "vial_reward" {
		return baseValue, nil
	}

	// Expect context to contain card level
	data, ok := baseValue.(map[string]interface{})
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for vial reward")
	}

	baseVials, ok := data["vials"].(int64)
	if !ok {
		return baseValue, fmt.Errorf("missing vials in base value")
	}

	bonusValue, tier, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "holygrail")
	if !ok {
		return baseValue, nil
	}

	bonus := int64(bonusValue)
	modifiedVials := baseVials + bonus

	result := make(map[string]interface{})
	for k, v := range data {
		result[k] = v
	}
	result["vials"] = modifiedVials

	slog.Info("Applied Holygrail effect",
		slog.String("user_id", userID),
		slog.Int64("base_vials", baseVials),
		slog.Int("tier", tier),
		slog.Int64("bonus", bonus),
		slog.Int64("modified_vials", modifiedVials))

	return result, nil
}

// IsActive checks if the effect is currently active
func (h *HolygrailHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the vial bonus amount for the current tier.
func (h *HolygrailHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "vial_reward" {
		return 0.0, nil
	}

	bonus, _, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "holygrail")
	if !ok {
		return 0.0, nil
	}
	return float64(bonus), nil
}

// SkyfriendHandler implements the "Wolf of Hyejoo" passive effect
type SkyfriendHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewSkyfriendHandler creates a new Skyfriend effect handler
func NewSkyfriendHandler(deps *effects.EffectDependencies) *SkyfriendHandler {
	metadata := effects.EffectMetadata{
		ID:          "wolfofhyejoo",
		Name:        "Wolf of Hyejoo",
		Description: "Gain cashback from winning auctions",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryEconomy,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    false,
		Tags:        []string{"passive", "auction", "snowflakes"},
		Version:     "1.0.0",
	}

	return &SkyfriendHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects
func (h *SkyfriendHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Wolf of Hyejoo effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the auction win snowflake bonus
func (h *SkyfriendHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "auction_win_bonus" {
		return baseValue, nil
	}

	// Expect auction price as baseValue
	auctionPrice, ok := baseValue.(int64)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for auction price")
	}

	bonusPercent, tier, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "wolfofhyejoo")
	if !ok {
		return int64(0), nil
	}
	bonus := int64(float64(auctionPrice) * float64(bonusPercent) / 100.0)

	slog.Info("Applied Skyfriend effect",
		slog.String("user_id", userID),
		slog.Int64("auction_price", auctionPrice),
		slog.Int("tier", tier),
		slog.Int("bonus_percent", bonusPercent),
		slog.Int64("bonus", bonus))

	return bonus, nil
}

// IsActive checks if the effect is currently active
func (h *SkyfriendHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the auction cashback percentage.
func (h *SkyfriendHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "auction_win_bonus" {
		return 0.0, nil
	}

	bonusPercent, _, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "wolfofhyejoo")
	if !ok {
		return 0.0, nil
	}
	return float64(bonusPercent) / 100.0, nil
}

// CherryblossHandler implements the "Cherry Blossoms" passive effect
type CherryblossHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewCherryblossHandler creates a new Cherrybloss effect handler
func NewCherryblossHandler(deps *effects.EffectDependencies) *CherryblossHandler {
	metadata := effects.EffectMetadata{
		ID:          "cherrybloss",
		Name:        "Cherry Blossom",
		Description: "Reduce forge and ascend cost",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryEconomy,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    true,
		Tags:        []string{"passive", "forge", "discount"},
		Version:     "1.0.0",
	}

	return &CherryblossHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects
func (h *CherryblossHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Cherrybloss passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the forge cost discount
func (h *CherryblossHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "forge_cost" {
		return baseValue, nil
	}

	baseCost, ok := baseValue.(int)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for forge cost")
	}

	discountPercent, tier, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "cherrybloss")
	if !ok {
		return baseValue, nil
	}
	discountedCost := int(float64(baseCost) * (1.0 - float64(discountPercent)/100.0))

	slog.Info("Applied Cherrybloss effect",
		slog.String("user_id", userID),
		slog.Int("base_cost", baseCost),
		slog.Int("tier", tier),
		slog.Int("discount_percent", discountPercent),
		slog.Int("discounted_cost", discountedCost))

	return discountedCost, nil
}

// IsActive checks if the effect is currently active
func (h *CherryblossHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the forge discount multiplier for the current tier.
func (h *CherryblossHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "forge_cost" {
		return 1.0, nil
	}

	discountPercent, _, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "cherrybloss")
	if !ok {
		return 1.0, nil
	}
	return 1.0 - (float64(discountPercent) / 100.0), nil
}

// RulerjeanneHandler implements the "The Ruler Jeanne" passive effect
type RulerjeanneHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewRulerjeanneHandler creates a new Rulerjeanne effect handler
func NewRulerjeanneHandler(deps *effects.EffectDependencies) *RulerjeanneHandler {
	metadata := effects.EffectMetadata{
		ID:          "rulerjeanne",
		Name:        "The Ruler Jeanne",
		Description: "Reduce daily cooldown",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryDaily,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    true,
		Tags:        []string{"passive", "daily", "cooldown"},
		Version:     "1.0.0",
	}

	return &RulerjeanneHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects
func (h *RulerjeanneHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Rulerjeanne passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the daily cooldown reduction
func (h *RulerjeanneHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "daily_cooldown" {
		return baseValue, nil
	}

	baseMinutes, ok := baseValue.(int)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for daily cooldown")
	}

	reducedMinutes, tier, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "rulerjeanne")
	if !ok {
		return baseValue, nil
	}

	slog.Info("Applied Rulerjeanne effect",
		slog.String("user_id", userID),
		slog.Int("base_minutes", baseMinutes),
		slog.Int("tier", tier),
		slog.Int("reduced_minutes", reducedMinutes))

	return reducedMinutes, nil
}

// IsActive checks if the effect is currently active
func (h *RulerjeanneHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the daily cooldown in minutes.
func (h *RulerjeanneHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "daily_cooldown" {
		return 1200.0, nil // Default cooldown in minutes
	}

	reducedMinutes, _, ok := currentTierValue(ctx, h.BaseEffectHandler.GetDependencies(), userID, "rulerjeanne")
	if !ok {
		return 1200.0, nil
	}
	return float64(reducedMinutes), nil
}

// SpellcardHandler implements the "Impossible Spell Card" passive effect
type SpellcardHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewSpellcardHandler creates a new Spellcard effect handler
func NewSpellcardHandler(deps *effects.EffectDependencies) *SpellcardHandler {
	metadata := effects.EffectMetadata{
		ID:          "spellcard",
		Name:        "Impossible Spell Card",
		Description: "Reduces cooldown on active effects by 40%",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryEffect,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    true,
		Tags:        []string{"passive", "cooldown", "effects"},
		Version:     "1.0.0",
	}

	return &SpellcardHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects
func (h *SpellcardHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Spellcard passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the effect cooldown reduction
func (h *SpellcardHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "effect_cooldown_reduction" {
		return baseValue, nil
	}

	baseReduction, ok := baseValue.(float64)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for cooldown reduction")
	}

	// Add 40% cooldown reduction (0.4)
	totalReduction := baseReduction + 0.4

	slog.Info("Applied Spellcard effect",
		slog.String("user_id", userID),
		slog.Float64("base_reduction", baseReduction),
		slog.Float64("total_reduction", totalReduction))

	return totalReduction, nil
}

// IsActive checks if the effect is currently active
func (h *SpellcardHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the cooldown reduction percentage (0.4)
func (h *SpellcardHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "effect_cooldown_reduction" {
		return 0.0, nil
	}

	return 0.4, nil
}

// WalpurgisnightHandler implements the "Walpurgis Night" passive effect
type WalpurgisnightHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewWalpurgisnightHandler creates a new Walpurgisnight effect handler
func NewWalpurgisnightHandler(deps *effects.EffectDependencies) *WalpurgisnightHandler {
	metadata := effects.EffectMetadata{
		ID:          "walpurgisnight",
		Name:        "Walpurgis Night",
		Description: "Allows 3 daily draws instead of 1, max 3-star level",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryDaily,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    true,
		Tags:        []string{"passive", "daily", "multiple_draws"},
		Version:     "1.0.0",
	}

	return &WalpurgisnightHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects
func (h *WalpurgisnightHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Walpurgisnight passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect modifies daily draw mechanics
func (h *WalpurgisnightHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "daily_draws" {
		return baseValue, nil
	}

	// Allow up to 3 draws with max 3-star
	drawConfig := map[string]interface{}{
		"max_draws":  3,
		"max_rarity": 3,
	}

	slog.Info("Applied Walpurgisnight effect",
		slog.String("user_id", userID),
		slog.Any("draw_config", drawConfig))

	return drawConfig, nil
}

// IsActive checks if the effect is currently active
func (h *WalpurgisnightHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the maximum draws allowed (3)
func (h *WalpurgisnightHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "daily_draws" {
		return 1.0, nil // Default is 1 draw
	}

	return 3.0, nil
}

// LambhyejooHandler implements the "Lamb of Hyejoo" passive effect
type LambhyejooHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewLambhyejooHandler creates a new Lambhyejoo effect handler
func NewLambhyejooHandler(deps *effects.EffectDependencies) *LambhyejooHandler {
	metadata := effects.EffectMetadata{
		ID:          "lambhyejoo",
		Name:        "Lamb of Hyejoo",
		Description: "Gain extra flakes from selling cards on auction",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryEconomy,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    false,
		Tags:        []string{"passive", "auction", "sales"},
		Version:     "1.0.0",
	}

	return &LambhyejooHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects
func (h *LambhyejooHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Lambhyejoo passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the auction sale bonus
func (h *LambhyejooHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "auction_sale_bonus" {
		return baseValue, nil
	}

	// Expect auction sale price as baseValue
	salePrice, ok := baseValue.(int64)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for auction sale price")
	}

	// Get the user's effect tier
	effectRepo, ok := h.BaseEffectHandler.GetDependencies().EffectRepo.(repositories.EffectRepository)
	if !ok {
		return baseValue, fmt.Errorf("invalid effect repository type")
	}

	userEffect, err := effectRepo.GetUserEffect(ctx, userID, "lambhyejoo")
	if err != nil {
		// Effect not found, return base value
		return baseValue, nil
	}

	// Get tier value from effect definition
	effectData := effects.GetEffectItemByID("lambhyejoo")
	if effectData == nil || effectData.TierData == nil {
		return baseValue, nil
	}

	// Get value for current tier
	tierIndex := userEffect.Tier - 1
	if tierIndex < 0 || tierIndex >= len(effectData.TierData.Values) {
		return baseValue, fmt.Errorf("invalid tier index")
	}

	// Calculate bonus based on tier
	bonusPercent := effectData.TierData.Values[tierIndex]
	bonus := int64(float64(salePrice) * float64(bonusPercent) / 100.0)

	slog.Info("Applied Lambhyejoo effect",
		slog.String("user_id", userID),
		slog.Int64("sale_price", salePrice),
		slog.Int("tier", userEffect.Tier),
		slog.Int("bonus_percent", bonusPercent),
		slog.Int64("bonus", bonus))

	return bonus, nil
}

// IsActive checks if the effect is currently active
func (h *LambhyejooHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the auction sale bonus percentage
func (h *LambhyejooHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "auction_sale_bonus" {
		return 0.0, nil
	}

	// Get the user's effect tier
	effectRepo, ok := h.BaseEffectHandler.GetDependencies().EffectRepo.(repositories.EffectRepository)
	if !ok {
		return 0.0, nil
	}

	userEffect, err := effectRepo.GetUserEffect(ctx, userID, "lambhyejoo")
	if err != nil {
		return 0.0, nil
	}

	// Get tier value from effect definition
	effectData := effects.GetEffectItemByID("lambhyejoo")
	if effectData == nil || effectData.TierData == nil {
		return 0.0, nil
	}

	// Get value for current tier
	tierIndex := userEffect.Tier - 1
	if tierIndex < 0 || tierIndex >= len(effectData.TierData.Values) {
		return 0.0, nil
	}

	bonusPercent := effectData.TierData.Values[tierIndex]
	return float64(bonusPercent) / 100.0, nil
}

// YouthyouthHandler implements the "Youth Youth By Young" passive effect
type YouthyouthHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewYouthyouthHandler creates a new Youthyouth effect handler
func NewYouthyouthHandler(deps *effects.EffectDependencies) *YouthyouthHandler {
	metadata := effects.EffectMetadata{
		ID:          "youthyouth",
		Name:        "Youth Youth By Young",
		Description: "Gain extra rewards from work",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryEconomy,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    false,
		Tags:        []string{"passive", "work", "rewards"},
		Version:     "1.0.0",
	}

	return &YouthyouthHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects
func (h *YouthyouthHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Youthyouth passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the work reward bonus
func (h *YouthyouthHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "work_reward" {
		return baseValue, nil
	}

	// Expect work reward as baseValue
	baseReward, ok := baseValue.(int)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for work reward")
	}

	// Get the user's effect tier
	effectRepo, ok := h.BaseEffectHandler.GetDependencies().EffectRepo.(repositories.EffectRepository)
	if !ok {
		return baseValue, fmt.Errorf("invalid effect repository type")
	}

	userEffect, err := effectRepo.GetUserEffect(ctx, userID, "youthyouth")
	if err != nil {
		// Effect not found, return base value
		return baseValue, nil
	}

	// Get tier value from effect definition
	effectData := effects.GetEffectItemByID("youthyouth")
	if effectData == nil || effectData.TierData == nil {
		return baseValue, nil
	}

	// Get value for current tier
	tierIndex := userEffect.Tier - 1
	if tierIndex < 0 || tierIndex >= len(effectData.TierData.Values) {
		return baseValue, fmt.Errorf("invalid tier index")
	}

	// Calculate bonus based on tier
	bonusPercent := effectData.TierData.Values[tierIndex]
	bonus := int(float64(baseReward) * float64(bonusPercent) / 100.0)
	modifiedReward := baseReward + bonus

	slog.Info("Applied Youthyouth effect",
		slog.String("user_id", userID),
		slog.Int("base_reward", baseReward),
		slog.Int("tier", userEffect.Tier),
		slog.Int("bonus_percent", bonusPercent),
		slog.Int("bonus", bonus),
		slog.Int("modified_reward", modifiedReward))

	return modifiedReward, nil
}

// IsActive checks if the effect is currently active
func (h *YouthyouthHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the work bonus percentage
func (h *YouthyouthHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "work_reward" {
		return 1.0, nil
	}

	// Get the user's effect tier
	effectRepo, ok := h.BaseEffectHandler.GetDependencies().EffectRepo.(repositories.EffectRepository)
	if !ok {
		return 1.0, nil
	}

	userEffect, err := effectRepo.GetUserEffect(ctx, userID, "youthyouth")
	if err != nil {
		return 1.0, nil
	}

	// Get tier value from effect definition
	effectData := effects.GetEffectItemByID("youthyouth")
	if effectData == nil || effectData.TierData == nil {
		return 1.0, nil
	}

	// Get value for current tier
	tierIndex := userEffect.Tier - 1
	if tierIndex < 0 || tierIndex >= len(effectData.TierData.Values) {
		return 1.0, nil
	}

	bonusPercent := effectData.TierData.Values[tierIndex]
	return 1.0 + (float64(bonusPercent) / 100.0), nil
}

// KisslaterHandler implements the "Kiss Later" passive effect
type KisslaterHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewKisslaterHandler creates a new Kisslater effect handler
func NewKisslaterHandler(deps *effects.EffectDependencies) *KisslaterHandler {
	metadata := effects.EffectMetadata{
		ID:          "kisslater",
		Name:        "Kiss Later",
		Description: "Gain extra XP from levelup",
		Type:        effects.EffectTypePassive,
		Category:    effects.EffectCategoryCollection,
		Cooldown:    0,
		MaxUses:     -1,
		Animated:    false,
		Tags:        []string{"passive", "levelup", "xp"},
		Version:     "1.0.0",
	}

	return &KisslaterHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
	}
}

// Execute for passive effects
func (h *KisslaterHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	return &effects.EffectResult{
		Success:  true,
		Message:  "Kisslater passive effect is active",
		Consumed: false,
	}, nil
}

// ApplyEffect applies the levelup XP bonus
func (h *KisslaterHandler) ApplyEffect(ctx context.Context, userID string, action string, baseValue interface{}) (interface{}, error) {
	if action != "levelup_xp" {
		return baseValue, nil
	}

	// Expect XP amount as baseValue
	baseXP, ok := baseValue.(int)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for XP")
	}

	// Get the user's effect tier
	effectRepo, ok := h.BaseEffectHandler.GetDependencies().EffectRepo.(repositories.EffectRepository)
	if !ok {
		return baseValue, fmt.Errorf("invalid effect repository type")
	}

	userEffect, err := effectRepo.GetUserEffect(ctx, userID, "kisslater")
	if err != nil {
		// Effect not found, return base value
		return baseValue, nil
	}

	// Get tier value from effect definition
	effectData := effects.GetEffectItemByID("kisslater")
	if effectData == nil || effectData.TierData == nil {
		return baseValue, nil
	}

	// Get value for current tier
	tierIndex := userEffect.Tier - 1
	if tierIndex < 0 || tierIndex >= len(effectData.TierData.Values) {
		return baseValue, fmt.Errorf("invalid tier index")
	}

	// Calculate bonus based on tier
	bonusPercent := effectData.TierData.Values[tierIndex]
	bonus := int(float64(baseXP) * float64(bonusPercent) / 100.0)
	modifiedXP := baseXP + bonus

	slog.Info("Applied Kisslater effect",
		slog.String("user_id", userID),
		slog.Int("base_xp", baseXP),
		slog.Int("tier", userEffect.Tier),
		slog.Int("bonus_percent", bonusPercent),
		slog.Int("bonus", bonus),
		slog.Int("modified_xp", modifiedXP))

	return modifiedXP, nil
}

// IsActive checks if the effect is currently active
func (h *KisslaterHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the XP bonus percentage
func (h *KisslaterHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "levelup_xp" {
		return 1.0, nil
	}

	// Get the user's effect tier
	effectRepo, ok := h.BaseEffectHandler.GetDependencies().EffectRepo.(repositories.EffectRepository)
	if !ok {
		return 1.0, nil
	}

	userEffect, err := effectRepo.GetUserEffect(ctx, userID, "kisslater")
	if err != nil {
		return 1.0, nil
	}

	// Get tier value from effect definition
	effectData := effects.GetEffectItemByID("kisslater")
	if effectData == nil || effectData.TierData == nil {
		return 1.0, nil
	}

	// Get value for current tier
	tierIndex := userEffect.Tier - 1
	if tierIndex < 0 || tierIndex >= len(effectData.TierData.Values) {
		return 1.0, nil
	}

	bonusPercent := effectData.TierData.Values[tierIndex]
	return 1.0 + (float64(bonusPercent) / 100.0), nil
}
