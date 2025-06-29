package effects

import "fmt"

// EffectApplicationResult contains the result of applying effects along with feedback
type EffectApplicationResult struct {
	OriginalValue interface{}            `json:"original_value"`
	ModifiedValue interface{}            `json:"modified_value"`
	AppliedEffects []AppliedEffectInfo   `json:"applied_effects"`
	Changed       bool                   `json:"changed"`
}

// AppliedEffectInfo contains information about an effect that was applied
type AppliedEffectInfo struct {
	EffectID     string      `json:"effect_id"`
	EffectName   string      `json:"effect_name"`
	Description  string      `json:"description"`
	Modifier     interface{} `json:"modifier"`
	Action       string      `json:"action"`
	Emoji        string      `json:"emoji"`
}

// NewEffectApplicationResult creates a new result with no effects applied
func NewEffectApplicationResult(originalValue interface{}) *EffectApplicationResult {
	return &EffectApplicationResult{
		OriginalValue:  originalValue,
		ModifiedValue:  originalValue,
		AppliedEffects: make([]AppliedEffectInfo, 0),
		Changed:        false,
	}
}

// AddAppliedEffect adds information about an effect that was applied
func (r *EffectApplicationResult) AddAppliedEffect(effectID, effectName, description, action, emoji string, modifier interface{}) {
	r.AppliedEffects = append(r.AppliedEffects, AppliedEffectInfo{
		EffectID:    effectID,
		EffectName:  effectName,
		Description: description,
		Modifier:    modifier,
		Action:      action,
		Emoji:       emoji,
	})
	r.Changed = true
}

// SetModifiedValue updates the final modified value
func (r *EffectApplicationResult) SetModifiedValue(value interface{}) {
	r.ModifiedValue = value
	if r.ModifiedValue != r.OriginalValue {
		r.Changed = true
	}
}

// HasEffects returns true if any effects were applied
func (r *EffectApplicationResult) HasEffects() bool {
	return len(r.AppliedEffects) > 0
}

// GetValue returns the final modified value
func (r *EffectApplicationResult) GetValue() interface{} {
	return r.ModifiedValue
}

// FormatEffectMessages formats applied effects as user-friendly messages
func (r *EffectApplicationResult) FormatEffectMessages() []string {
	if !r.HasEffects() {
		return []string{}
	}

	messages := make([]string, len(r.AppliedEffects))
	for i, effect := range r.AppliedEffects {
		messages[i] = formatEffectMessage(effect)
	}
	return messages
}

// formatEffectMessage formats a single effect into a user-friendly message
func formatEffectMessage(effect AppliedEffectInfo) string {
	emoji := effect.Emoji
	if emoji == "" {
		emoji = "🛡️" // Default passive effect emoji
	}

	switch effect.Action {
	case "daily_reward":
		if bonus, ok := effect.Modifier.(int); ok && bonus > 0 {
			return fmt.Sprintf("%s **%s**: +%d snowflakes from claims", emoji, effect.EffectName, bonus)
		}
	case "claim_3star_chance":
		if modifier, ok := effect.Modifier.(float64); ok && modifier > 1.0 {
			increase := int((modifier - 1.0) * 100)
			return fmt.Sprintf("%s **%s**: +%d%% 3-star chance (first claim)", emoji, effect.EffectName, increase)
		}
	case "forge_cost":
		if modifier, ok := effect.Modifier.(float64); ok && modifier < 1.0 {
			discount := int((1.0 - modifier) * 100)
			return fmt.Sprintf("%s **%s**: %d%% forge cost discount", emoji, effect.EffectName, discount)
		}
	case "vial_reward":
		if modifier, ok := effect.Modifier.(float64); ok && modifier > 1.0 {
			increase := int((modifier - 1.0) * 100)
			return fmt.Sprintf("%s **%s**: +%d%% vials from 1-2 star cards", emoji, effect.EffectName, increase)
		}
	case "daily_cooldown":
		if hours, ok := effect.Modifier.(int); ok && hours < 20 {
			return fmt.Sprintf("%s **%s**: Daily cooldown reduced to %d hours", emoji, effect.EffectName, hours)
		}
	case "effect_cooldown_reduction":
		if reduction, ok := effect.Modifier.(float64); ok && reduction > 0.0 {
			reductionPercent := int(reduction * 100)
			return fmt.Sprintf("%s **%s**: %d%% cooldown reduction on active effects", emoji, effect.EffectName, reductionPercent)
		}
	}

	// Generic fallback
	return fmt.Sprintf("%s **%s**: %s", emoji, effect.EffectName, effect.Description)
}