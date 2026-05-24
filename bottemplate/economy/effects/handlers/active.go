package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"log/slog"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/uptrace/bun"
)

// ClaimRecallHandler implements the "Claim Recall" active effect
type ClaimRecallHandler struct {
	*effects.BaseEffectHandler
	userRepo   repositories.UserRepository
	effectRepo repositories.EffectRepository
	db         *bun.DB
}

// NewClaimRecallHandler creates a new Claim Recall effect handler
func NewClaimRecallHandler(deps *effects.EffectDependencies) *ClaimRecallHandler {
	metadata := effects.EffectMetadata{
		ID:          "claimrecall",
		Name:        "Claim Recall",
		Description: "Claim cost gets recalled by 4 claims, as if they never happened",
		Type:        effects.EffectTypeActive,
		Category:    effects.EffectCategoryClaim,
		Cooldown:    15 * time.Hour,
		MaxUses:     20, // Based on static data duration
		Animated:    false,
		Tags:        []string{"active", "claim", "cost_reduction"},
		Version:     "1.0.0",
	}

	return &ClaimRecallHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
		effectRepo:        deps.EffectRepo.(repositories.EffectRepository),
		db:                deps.Database.(*bun.DB),
	}
}

// Execute implements the claim recall logic
func (h *ClaimRecallHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	var stats models.ClaimStats
	err := h.db.NewSelect().
		Model(&stats).
		Where("user_id = ?", params.UserID).
		Scan(ctx)

	if err != nil || stats.DailyClaims < 4 {
		return &effects.EffectResult{
			Success:  false,
			Message:  "you can only use Claim Recall after at least 4 claims today!",
			Consumed: false,
		}, nil
	}

	previousClaims := stats.DailyClaims
	newClaims := previousClaims - 4
	newCost := (newClaims + 1) * 700

	_, err = h.db.NewUpdate().
		Model((*models.ClaimStats)(nil)).
		Set("daily_claims = ?", newClaims).
		Set("updated_at = ?", time.Now()).
		Where("user_id = ?", params.UserID).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update claim stats: %w", err)
	}

	slog.Info("Claim Recall effect executed",
		slog.String("user_id", params.UserID),
		slog.Int("previous_claims", previousClaims),
		slog.Int("new_claims", newClaims),
		slog.Int("new_cost", newCost))

	return &effects.EffectResult{
		Success:  true,
		Message:  fmt.Sprintf("claim cost has been reset to **%d**", newCost),
		Consumed: true,
		Data: map[string]interface{}{
			"previous_claims": previousClaims,
			"new_claims":      newClaims,
			"new_cost":        newCost,
		},
		Events: []effects.EffectEvent{
			{
				Type:      "claim_recall_used",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"user_id":         params.UserID,
					"claims_reduced":  4,
					"new_claim_count": newClaims,
					"new_cost":        newCost,
				},
			},
		},
	}, nil
}

// GetCooldown returns the cooldown duration
func (h *ClaimRecallHandler) GetCooldown(ctx context.Context, userID string) (time.Duration, error) {
	return h.GetMetadata().Cooldown, nil
}

// ConsumeUse decrements the use count
func (h *ClaimRecallHandler) ConsumeUse(ctx context.Context, userID string) error {
	// This would typically update the user_effects table
	// For now, we'll let the effect manager handle this
	return nil
}

// GetRemainingUses returns remaining uses
func (h *ClaimRecallHandler) GetRemainingUses(ctx context.Context, userID string) (int, error) {
	// This would query the user_effects table
	// For now, return max uses
	return h.GetMetadata().MaxUses, nil
}

// SpaceUnityHandler implements the "The Space Unity" active effect
type SpaceUnityHandler struct {
	*effects.BaseEffectHandler
	userRepo       repositories.UserRepository
	cardRepo       repositories.CardRepository
	userCardRepo   repositories.UserCardRepository
	collectionRepo repositories.CollectionRepository
	db             *bun.DB
}

// NewSpaceUnityHandler creates a new Space Unity effect handler
func NewSpaceUnityHandler(deps *effects.EffectDependencies) *SpaceUnityHandler {
	metadata := effects.EffectMetadata{
		ID:          "spaceunity",
		Name:        "Space Unity",
		Description: "Gives a random unique card from a non-promo collection",
		Type:        effects.EffectTypeActive,
		Category:    effects.EffectCategoryCollection,
		Cooldown:    40 * time.Hour,
		MaxUses:     8, // Based on static data duration
		Animated:    false,
		Tags:        []string{"active", "collection", "unique_card"},
		Version:     "1.0.0",
	}

	return &SpaceUnityHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
		cardRepo:          deps.CardRepo.(repositories.CardRepository),
		userCardRepo:      deps.UserCardRepo.(repositories.UserCardRepository),
		collectionRepo:    deps.CollectionRepo.(repositories.CollectionRepository),
		db:                deps.Database.(*bun.DB),
	}
}

// Execute implements the space unity logic
func (h *SpaceUnityHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	if params.Arguments == "" {
		return &effects.EffectResult{
			Success:  false,
			Message:  "Please specify collection ID (e.g., 'twice' or 'blackpink')",
			Consumed: false,
		}, nil
	}

	// Remove leading dash if present
	collectionArg := strings.TrimPrefix(params.Arguments, "-")

	// Find collection using multiple strategies
	collection, err := h.findCollection(ctx, collectionArg)
	if err != nil {
		return &effects.EffectResult{
			Success:  false,
			Message:  fmt.Sprintf("Collection '%s' not found. Please check the collection name and try again.", collectionArg),
			Consumed: false,
		}, nil
	}

	// Check collection restrictions
	if restriction := h.checkCollectionRestrictions(collection); restriction != "" {
		return &effects.EffectResult{
			Success:  false,
			Message:  restriction,
			Consumed: false,
		}, nil
	}

	// Get unique card from collection
	card, err := h.getUniqueCard(ctx, params.UserID, collection)
	if err != nil {
		return &effects.EffectResult{
			Success:  false,
			Message:  err.Error(),
			Consumed: false,
		}, nil
	}

	// Add card to user's collection
	if err := h.addCardToUser(ctx, params.UserID, card.ID); err != nil {
		return nil, fmt.Errorf("failed to add card to user: %w", err)
	}

	slog.Info("Space Unity effect executed",
		slog.String("user_id", params.UserID),
		slog.String("collection", collection.Name),
		slog.Int64("card_id", card.ID),
		slog.String("card_name", card.Name),
		slog.Int("card_level", card.Level))

	return &effects.EffectResult{
		Success:  true,
		Message:  fmt.Sprintf("You got **%s** (%s) from %s collection!", card.Name, strings.Repeat("⭐", card.Level), collection.Name),
		Consumed: true,
		Data: map[string]interface{}{
			"collection_id":   collection.ID,
			"collection_name": collection.Name,
			"card_id":         card.ID,
			"card_name":       card.Name,
			"card_level":      card.Level,
		},
		Events: []effects.EffectEvent{
			{
				Type:      "space_unity_used",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"user_id":         params.UserID,
					"collection_id":   collection.ID,
					"collection_name": collection.Name,
					"card_obtained":   card.ID,
				},
			},
		},
	}, nil
}

// findCollection finds a collection by ID, alias, or name
func (h *SpaceUnityHandler) findCollection(ctx context.Context, query string) (*models.Collection, error) {
	var collection models.Collection

	// First try exact ID match
	err := h.db.NewSelect().
		Model(&collection).
		Where("id = ?", query).
		Scan(ctx)

	// If not found by ID, try aliases
	if err != nil {
		err = h.db.NewSelect().
			Model(&collection).
			Where("aliases @> ?", fmt.Sprintf(`["%s"]`, query)).
			Scan(ctx)
	}

	// If still not found, try case-insensitive search
	if err != nil {
		err = h.db.NewSelect().
			Model(&collection).
			Where("LOWER(id) = LOWER(?)", query).
			Scan(ctx)
	}

	// If still not found, try partial name matching
	if err != nil {
		err = h.db.NewSelect().
			Model(&collection).
			Where("LOWER(name) LIKE LOWER(?)", "%"+query+"%").
			Limit(1).
			Scan(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("collection not found")
	}

	return &collection, nil
}

// checkCollectionRestrictions checks if collection is allowed for Space Unity
func (h *SpaceUnityHandler) checkCollectionRestrictions(collection *models.Collection) string {
	if collection.Promo {
		return "cannot use this effect on promo collections"
	}

	if collection.Fragments {
		return "cannot use this effect on fragmented collections"
	}

	// Check for restricted tags
	for _, tag := range collection.Tags {
		switch tag {
		case "lottery":
			return "cannot use this effect on lottery collections"
		case "jackpot":
			return "cannot use this effect on jackpot collections"
		case "album":
			return "cannot use this effect on album collections"
		}
	}

	return ""
}

// getUniqueCard gets a random unique card from the collection
func (h *SpaceUnityHandler) getUniqueCard(ctx context.Context, userID string, collection *models.Collection) (*models.Card, error) {
	// Get user's existing cards
	existingCards, err := h.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user cards: %w", err)
	}

	// Create map of owned card IDs
	ownedCardIDs := make(map[int64]bool)
	for _, uc := range existingCards {
		if uc.Amount > 0 {
			ownedCardIDs[uc.CardID] = true
		}
	}

	// Get available cards from collection (level < 4, not excluded, not owned)
	var availableCards []*models.Card
	err = h.db.NewSelect().
		Model(&availableCards).
		Where("col_id = ? AND level < 4", collection.ID).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get collection cards: %w", err)
	}

	// Filter out owned and excluded cards
	var uniqueCards []*models.Card
	for _, card := range availableCards {
		if ownedCardIDs[card.ID] {
			continue
		}

		// Check for excluded tag
		isExcluded := false
		for _, tag := range card.Tags {
			if tag == "excluded" {
				isExcluded = true
				break
			}
		}
		if isExcluded {
			continue
		}

		uniqueCards = append(uniqueCards, card)
	}

	if len(uniqueCards) == 0 {
		return nil, fmt.Errorf("Cannot fetch unique card from **%s** collection (you own them all!)", collection.Name)
	}

	// Select random card
	selectedCard := uniqueCards[rand.Intn(len(uniqueCards))]
	return selectedCard, nil
}

// addCardToUser adds a card to user's collection
func (h *SpaceUnityHandler) addCardToUser(ctx context.Context, userID string, cardID int64) error {
	userCard := &models.UserCard{
		UserID: userID,
		CardID: cardID,
		Amount: 1,
	}
	return h.userCardRepo.Create(ctx, userCard)
}

// GetCooldown returns the cooldown duration
func (h *SpaceUnityHandler) GetCooldown(ctx context.Context, userID string) (time.Duration, error) {
	return h.GetMetadata().Cooldown, nil
}

// ConsumeUse decrements the use count
func (h *SpaceUnityHandler) ConsumeUse(ctx context.Context, userID string) error {
	return nil
}

// GetRemainingUses returns remaining uses
func (h *SpaceUnityHandler) GetRemainingUses(ctx context.Context, userID string) (int, error) {
	return h.GetMetadata().MaxUses, nil
}

// WalpurgisNightHandler implements the "Walpurgis Night" active item.
type WalpurgisNightHandler struct {
	*effects.BaseEffectHandler
	db *bun.DB
}

// NewWalpurgisNightHandler creates a new Walpurgis Night item handler.
func NewWalpurgisNightHandler(deps *effects.EffectDependencies) *WalpurgisNightHandler {
	metadata := effects.EffectMetadata{
		ID:          "walpurgisnight",
		Name:        "Walpurgis Night",
		Description: "Grants an extra draw",
		Type:        effects.EffectTypeActive,
		Category:    effects.EffectCategoryClaim,
		Cooldown:    24 * time.Hour,
		MaxUses:     20,
		Animated:    false,
		Tags:        []string{"active", "claim", "extra_draw"},
		Version:     "1.0.0",
	}

	return &WalpurgisNightHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		db:                deps.Database.(*bun.DB),
	}
}

// Execute grants an extra draw by rolling today's claim counter back by one.
func (h *WalpurgisNightHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	var stats models.ClaimStats
	err := h.db.NewSelect().
		Model(&stats).
		Where("user_id = ?", params.UserID).
		Scan(ctx)
	if err != nil || stats.DailyClaims <= 0 {
		return &effects.EffectResult{
			Success:  false,
			Message:  "You need to claim at least once today before Walpurgis Night can grant an extra draw.",
			Consumed: false,
		}, nil
	}

	newClaims := stats.DailyClaims - 1
	_, err = h.db.NewUpdate().
		Model((*models.ClaimStats)(nil)).
		Set("daily_claims = ?", newClaims).
		Set("updated_at = ?", time.Now()).
		Where("user_id = ?", params.UserID).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to grant extra draw: %w", err)
	}

	return &effects.EffectResult{
		Success:  true,
		Message:  "Walpurgis Night granted you an extra draw. Your next claim cost has been rolled back by one claim.",
		Consumed: true,
		Data: map[string]interface{}{
			"previous_claims": stats.DailyClaims,
			"new_claims":      newClaims,
		},
		Events: []effects.EffectEvent{
			{
				Type:      "walpurgisnight_used",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"user_id":         params.UserID,
					"claims_reduced":  1,
					"new_claim_count": newClaims,
				},
			},
		},
	}, nil
}

func (h *WalpurgisNightHandler) GetCooldown(ctx context.Context, userID string) (time.Duration, error) {
	return h.GetMetadata().Cooldown, nil
}

func (h *WalpurgisNightHandler) ConsumeUse(ctx context.Context, userID string) error {
	return nil
}

func (h *WalpurgisNightHandler) GetRemainingUses(ctx context.Context, userID string) (int, error) {
	return h.GetMetadata().MaxUses, nil
}

// JudgeDayHandler implements the "The Judgment Day" active effect
type JudgeDayHandler struct {
	*effects.BaseEffectHandler
	registry *effects.EffectRegistry
}

// NewJudgeDayHandler creates a new Judgment Day effect handler
func NewJudgeDayHandler(deps *effects.EffectDependencies, registry *effects.EffectRegistry) *JudgeDayHandler {
	metadata := effects.EffectMetadata{
		ID:          "judgeday",
		Name:        "Judgement Day",
		Description: "Can be used as any other item",
		Type:        effects.EffectTypeActive,
		Category:    effects.EffectCategoryCollection,
		Cooldown:    48 * time.Hour,
		MaxUses:     14, // Based on static data duration
		Animated:    false,
		Tags:        []string{"active", "proxy", "meta"},
		Version:     "1.0.0",
	}

	return &JudgeDayHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		registry:          registry,
	}
}

// Execute implements the judgment day logic
func (h *JudgeDayHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	if params.Arguments == "" {
		return &effects.EffectResult{
			Success:  false,
			Message:  "Please specify effect ID (e.g., 'claimrecall' or 'spaceunity -twice')",
			Consumed: false,
		}, nil
	}

	// Parse effect ID and remaining args
	parts := strings.SplitN(params.Arguments, " ", 2)
	effectID := parts[0]
	remainingArgs := ""
	if len(parts) > 1 {
		remainingArgs = parts[1]
	}

	// Check if effect exists and is active
	handler, err := h.registry.GetActiveEffect(effectID)
	if err != nil {
		return &effects.EffectResult{
			Success:  false,
			Message:  fmt.Sprintf("Effect '%s' not found or not usable", effectID),
			Consumed: false,
		}, nil
	}

	// Check exclusion list
	excludedEffects := []string{"memoryval", "memoryxmas", "memorybday", "memoryhall", "judgeday"}
	for _, excluded := range excludedEffects {
		if effectID == excluded {
			return &effects.EffectResult{
				Success:  false,
				Message:  "You cannot use that effect with Judgment Day",
				Consumed: false,
			}, nil
		}
	}

	// Format arguments for specific effects
	formattedArgs := remainingArgs
	if effectID == "spaceunity" && remainingArgs != "" && !strings.HasPrefix(remainingArgs, "-") {
		formattedArgs = "-" + remainingArgs
	}

	// Execute the target effect directly
	targetParams := effects.EffectParams{
		UserID:    params.UserID,
		Arguments: formattedArgs,
		Context:   params.Context,
		Metadata:  params.Metadata,
	}

	result, err := handler.Execute(ctx, targetParams)
	if err != nil {
		return nil, fmt.Errorf("failed to execute target effect: %w", err)
	}

	// Override the message to indicate Judgment Day usage
	if result.Success {
		result.Message = fmt.Sprintf("[Judgment Day] %s", result.Message)
		result.Consumed = true // Judgment Day is consumed even if target effect isn't
	}

	slog.Info("Judgment Day effect executed",
		slog.String("user_id", params.UserID),
		slog.String("target_effect", effectID),
		slog.String("target_args", formattedArgs),
		slog.Bool("success", result.Success))

	return result, nil
}

// GetCooldown returns the cooldown duration
func (h *JudgeDayHandler) GetCooldown(ctx context.Context, userID string) (time.Duration, error) {
	return h.GetMetadata().Cooldown, nil
}

// ConsumeUse decrements the use count
func (h *JudgeDayHandler) ConsumeUse(ctx context.Context, userID string) error {
	return nil
}

// GetRemainingUses returns remaining uses
func (h *JudgeDayHandler) GetRemainingUses(ctx context.Context, userID string) (int, error) {
	return h.GetMetadata().MaxUses, nil
}

// EnayanoHandler implements the "Enlightened Ayano" active effect
type EnayanoHandler struct {
	*effects.BaseEffectHandler
	userRepo   repositories.UserRepository
	effectRepo repositories.EffectRepository
	db         *bun.DB
}

// NewEnayanoHandler creates a new Enlightened Ayano effect handler
func NewEnayanoHandler(deps *effects.EffectDependencies) *EnayanoHandler {
	metadata := effects.EffectMetadata{
		ID:          "enayano",
		Name:        "Enlightened Ayano",
		Description: "Instantly completes tier 1 quest",
		Type:        effects.EffectTypeActive,
		Category:    effects.EffectCategoryUtility,
		Cooldown:    12 * time.Hour,
		MaxUses:     5, // Based on static data duration
		Animated:    true,
		Tags:        []string{"active", "quest", "completion"},
		Version:     "1.0.0",
	}

	return &EnayanoHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
		effectRepo:        deps.EffectRepo.(repositories.EffectRepository),
		db:                deps.Database.(*bun.DB),
	}
}

// Execute implements the quest completion logic
func (h *EnayanoHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	// For now, implement a placeholder that simulates quest completion
	// This will need to be integrated with the actual quest system when available

	slog.Info("Enlightened Ayano effect executed (placeholder)",
		slog.String("user_id", params.UserID),
		slog.String("note", "Quest system integration pending"))

	// Simulate quest completion - this would typically:
	// 1. Find active tier 1 quest for user
	// 2. Mark it as completed
	// 3. Award quest rewards

	return &effects.EffectResult{
		Success:  true,
		Message:  "Quest completion functionality pending implementation. Effect consumed.",
		Consumed: true,
		Data: map[string]interface{}{
			"quest_type": "tier1",
			"status":     "pending_integration",
		},
		Events: []effects.EffectEvent{
			{
				Type:      "enayano_used",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"user_id":    params.UserID,
					"effect_id":  "enayano",
					"quest_type": "tier1",
				},
			},
		},
	}, nil
}

// GetCooldown returns the cooldown duration
func (h *EnayanoHandler) GetCooldown(ctx context.Context, userID string) (time.Duration, error) {
	return h.GetMetadata().Cooldown, nil
}

// ConsumeUse decrements the use count
func (h *EnayanoHandler) ConsumeUse(ctx context.Context, userID string) error {
	return nil
}

// GetRemainingUses returns remaining uses
func (h *EnayanoHandler) GetRemainingUses(ctx context.Context, userID string) (int, error) {
	return h.GetMetadata().MaxUses, nil
}

// PbocchiHandler implements the "Powerful Bocchi" active effect
type PbocchiHandler struct {
	*effects.BaseEffectHandler
	userRepo   repositories.UserRepository
	effectRepo repositories.EffectRepository
	db         *bun.DB
}

// NewPbocchiHandler creates a new Powerful Bocchi effect handler
func NewPbocchiHandler(deps *effects.EffectDependencies) *PbocchiHandler {
	metadata := effects.EffectMetadata{
		ID:          "pbocchi",
		Name:        "Powerful Bocchi",
		Description: "Generates a tier 1 quest",
		Type:        effects.EffectTypeActive,
		Category:    effects.EffectCategoryUtility,
		Cooldown:    18 * time.Hour,
		MaxUses:     3, // Based on static data duration
		Animated:    true,
		Tags:        []string{"active", "quest", "generation"},
		Version:     "1.0.0",
	}

	return &PbocchiHandler{
		BaseEffectHandler: effects.NewBaseEffectHandler(metadata, deps),
		userRepo:          deps.UserRepo.(repositories.UserRepository),
		effectRepo:        deps.EffectRepo.(repositories.EffectRepository),
		db:                deps.Database.(*bun.DB),
	}
}

// Execute implements the quest generation logic
func (h *PbocchiHandler) Execute(ctx context.Context, params effects.EffectParams) (*effects.EffectResult, error) {
	// For now, implement a placeholder that simulates quest generation
	// This will need to be integrated with the actual quest system when available

	slog.Info("Powerful Bocchi effect executed (placeholder)",
		slog.String("user_id", params.UserID),
		slog.String("note", "Quest system integration pending"))

	// Simulate quest generation - this would typically:
	// 1. Check if user has active quests
	// 2. Generate a new tier 1 quest
	// 3. Add it to user's active quests

	return &effects.EffectResult{
		Success:  true,
		Message:  "Quest generation functionality pending implementation. Effect consumed.",
		Consumed: true,
		Data: map[string]interface{}{
			"quest_type":  "tier1",
			"status":      "pending_integration",
			"quest_count": 1,
		},
		Events: []effects.EffectEvent{
			{
				Type:      "pbocchi_used",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"user_id":    params.UserID,
					"effect_id":  "pbocchi",
					"quest_type": "tier1",
					"generated":  1,
				},
			},
		},
	}, nil
}

// GetCooldown returns the cooldown duration
func (h *PbocchiHandler) GetCooldown(ctx context.Context, userID string) (time.Duration, error) {
	return h.GetMetadata().Cooldown, nil
}

// ConsumeUse decrements the use count
func (h *PbocchiHandler) ConsumeUse(ctx context.Context, userID string) error {
	return nil
}

// GetRemainingUses returns remaining uses
func (h *PbocchiHandler) GetRemainingUses(ctx context.Context, userID string) (int, error) {
	return h.GetMetadata().MaxUses, nil
}
