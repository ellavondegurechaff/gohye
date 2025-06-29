package handlers

import (
	"context"
	"fmt"

	"log/slog"

	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
)

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
}

// NewCakedayHandler creates a new Cakeday effect handler
func NewCakedayHandler(deps *effects.EffectDependencies) *CakedayHandler {
	metadata := effects.EffectMetadata{
		ID:          "cakeday",
		Name:        "Cake Day",
		Description: "Get +100 snowflakes in your daily for every claim you did",
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
	}
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

	user, err := h.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return baseValue, err
	}

	baseReward, ok := baseValue.(int)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for daily reward")
	}

	// Add 100 snowflakes per claim made today
	bonus := user.DailyStats.Claims * 100
	modifiedReward := baseReward + bonus

	slog.Info("Applied Cakeday effect",
		slog.String("user_id", userID),
		slog.Int("base_reward", baseReward),
		slog.Int("claims_today", user.DailyStats.Claims),
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

	user, err := h.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		return 0.0, err
	}

	return float64(user.DailyStats.Claims * 100), nil
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
		Description: "Get +25% of vials when liquifying 1 and 2-star cards",
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

	cardLevel, ok := data["card_level"].(int)
	if !ok {
		return baseValue, fmt.Errorf("missing card_level in base value")
	}

	// Only apply bonus for 1-2 star cards
	if cardLevel > 2 {
		return baseValue, nil
	}

	bonus := int64(float64(baseVials) * 0.25)
	modifiedVials := baseVials + bonus

	result := make(map[string]interface{})
	for k, v := range data {
		result[k] = v
	}
	result["vials"] = modifiedVials

	slog.Info("Applied Holygrail effect",
		slog.String("user_id", userID),
		slog.Int64("base_vials", baseVials),
		slog.Int("card_level", cardLevel),
		slog.Int64("bonus", bonus),
		slog.Int64("modified_vials", modifiedVials))

	return result, nil
}

// IsActive checks if the effect is currently active
func (h *HolygrailHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the vial bonus multiplier (1.25 for 1-2 star cards)
func (h *HolygrailHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "vial_reward" {
		return 1.0, nil
	}

	return 1.25, nil
}

// SkyfriendHandler implements the "Skies Of Friendship" passive effect
type SkyfriendHandler struct {
	*effects.BaseEffectHandler
	userRepo repositories.UserRepository
}

// NewSkyfriendHandler creates a new Skyfriend effect handler
func NewSkyfriendHandler(deps *effects.EffectDependencies) *SkyfriendHandler {
	metadata := effects.EffectMetadata{
		ID:          "skyfriend",
		Name:        "Skies Of Friendship",
		Description: "Get 10% snowflakes back when winning an auction",
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
		Message:  "Skyfriend passive effect is active",
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

	// Return 10% of auction price as bonus
	bonus := int64(float64(auctionPrice) * 0.10)

	slog.Info("Applied Skyfriend effect",
		slog.String("user_id", userID),
		slog.Int64("auction_price", auctionPrice),
		slog.Int64("bonus", bonus))

	return bonus, nil
}

// IsActive checks if the effect is currently active
func (h *SkyfriendHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the auction bonus rate (0.10)
func (h *SkyfriendHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "auction_win_bonus" {
		return 0.0, nil
	}

	return 0.10, nil
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
		Name:        "Cherry Blossoms",
		Description: "Card forging costs 50% less",
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

	// Apply 50% discount
	discountedCost := int(float64(baseCost) * 0.50)

	slog.Info("Applied Cherrybloss effect",
		slog.String("user_id", userID),
		slog.Int("base_cost", baseCost),
		slog.Int("discounted_cost", discountedCost))

	return discountedCost, nil
}

// IsActive checks if the effect is currently active
func (h *CherryblossHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the forge discount multiplier (0.50)
func (h *CherryblossHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "forge_cost" {
		return 1.0, nil
	}

	return 0.50, nil
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
		Description: "Reduces daily cooldown from 20 to 17 hours",
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

	baseHours, ok := baseValue.(int)
	if !ok {
		return baseValue, fmt.Errorf("invalid base value type for daily cooldown")
	}

	// Reduce from 20 to 17 hours
	reducedHours := 17

	slog.Info("Applied Rulerjeanne effect",
		slog.String("user_id", userID),
		slog.Int("base_hours", baseHours),
		slog.Int("reduced_hours", reducedHours))

	return reducedHours, nil
}

// IsActive checks if the effect is currently active
func (h *RulerjeanneHandler) IsActive(ctx context.Context, userID string) (bool, error) {
	return true, nil // Simplified for now
}

// GetModifier returns the daily cooldown in hours (17)
func (h *RulerjeanneHandler) GetModifier(ctx context.Context, userID string, action string) (float64, error) {
	if action != "daily_cooldown" {
		return 20.0, nil // Default cooldown
	}

	return 17.0, nil
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
		"max_draws":   3,
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