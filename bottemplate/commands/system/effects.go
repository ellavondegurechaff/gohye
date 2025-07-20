package system

import (
	"context"
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Effects = discord.SlashCommandCreate{
	Name:        "effects",
	Description: "View your effects and their progression",
}

type EffectsHandler struct {
	bot           *bottemplate.Bot
	effectManager *effects.Manager
}

func NewEffectsHandler(b *bottemplate.Bot, effectManager *effects.Manager) *EffectsHandler {
	return &EffectsHandler{
		bot:           b,
		effectManager: effectManager,
	}
}

func (h *EffectsHandler) Handle(event *handler.CommandEvent) error {
	ctx := context.Background()
	userID := event.User().ID.String()

	// Get user effects sorted by tier
	userEffects, err := h.effectManager.GetUserEffectsSorted(ctx, userID)
	if err != nil {
		return utils.EH.CreateErrorEmbed(event, fmt.Sprintf("Failed to fetch effects: %v", err))
	}

	// Filter to only show non-recipe effects
	var activeEffects []*models.UserEffect
	for _, effect := range userEffects {
		if !effect.IsRecipe && effect.Active {
			activeEffects = append(activeEffects, effect)
		}
	}

	if len(activeEffects) == 0 {
		embed := discord.NewEmbedBuilder().
			SetTitle("📊 Your Effects Progress").
			SetDescription("You don't have any active effects yet!\n\nVisit the `/shop` to purchase effect recipes and start your progression journey.").
			SetColor(0x2b2d31).
			SetFooter("💡 Tip: Effects get stronger as you use them!", "").
			Build()

		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{embed},
		})
	}

	// Build the effects display
	var description strings.Builder
	description.WriteString("═══════════════════════\n\n")
	description.WriteString("**YOUR EFFECTS**\n")

	for _, userEffect := range activeEffects {
		// Get effect definition
		effectData := effects.GetEffectItemByID(userEffect.EffectID)
		if effectData == nil {
			continue
		}

		// Skip effects without tier data
		if effectData.TierData == nil {
			continue
		}

		// Format tier stars
		stars := h.formatTierStars(userEffect.Tier)

		// Get current and next tier values
		currentValue := 0
		nextValue := 0
		if userEffect.Tier > 0 && userEffect.Tier <= len(effectData.TierData.Values) {
			currentValue = effectData.TierData.Values[userEffect.Tier-1]
		}
		if userEffect.Tier < len(effectData.TierData.Values) {
			nextValue = effectData.TierData.Values[userEffect.Tier]
		}

		// Get progress threshold
		threshold := 0
		if userEffect.Tier > 0 && userEffect.Tier <= len(effectData.TierData.Thresholds) {
			threshold = effectData.TierData.Thresholds[userEffect.Tier-1]
		}

		// Build effect box
		description.WriteString("```\n")
		description.WriteString(fmt.Sprintf("┌─────────────────────────┐\n"))
		description.WriteString(fmt.Sprintf("│ %-14s %s │\n", effectData.Name, stars))
		description.WriteString(fmt.Sprintf("│ %s               │\n", h.formatEffectValue(effectData.ID, currentValue)))

		if userEffect.Tier < 5 {
			// Show progress bar
			progressBar := h.createProgressBar(userEffect.Progress, threshold)
			description.WriteString(fmt.Sprintf("│ %s %d/%d   │\n", progressBar, userEffect.Progress, threshold))
			description.WriteString(fmt.Sprintf("│ 📈 Next: %s     │\n", h.formatEffectValue(effectData.ID, nextValue)))
		} else {
			// Max tier reached
			description.WriteString(fmt.Sprintf("│ 🌟 MAX TIER REACHED!    │\n"))
			description.WriteString(fmt.Sprintf("│                         │\n"))
		}

		description.WriteString(fmt.Sprintf("└─────────────────────────┘\n"))
		description.WriteString("```\n")
	}

	// Add footer with tips
	description.WriteString("\n💡 **Tips:**\n")
	description.WriteString("• Effects progress automatically as you play\n")
	description.WriteString("• Use `/effect info <effect>` for detailed information\n")
	description.WriteString("• Upgrade with `/effect upgrade <effect>` when ready")

	embed := discord.NewEmbedBuilder().
		SetTitle("📊 Your Effects Progress").
		SetDescription(description.String()).
		SetColor(0x57F287).
		SetFooter(fmt.Sprintf("Requested by %s", event.User().Username), event.User().EffectiveAvatarURL()).
		Build()

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
}

func (h *EffectsHandler) formatTierStars(tier int) string {
	if tier >= 5 {
		return "🌟"
	}

	stars := ""
	for i := 0; i < tier; i++ {
		stars += "⭐"
	}

	// Pad with spaces for alignment
	for i := tier; i < 5; i++ {
		stars += "  "
	}

	return stars
}

func (h *EffectsHandler) formatEffectValue(effectID string, value int) string {
	switch effectID {
	case "cakeday":
		return fmt.Sprintf("+%d flakes/claim", value)
	case "holygrail":
		return fmt.Sprintf("+%d vials/liquify", value)
	case "wolfofhyejoo", "lambofhyejoo":
		return fmt.Sprintf("%d%% cashback", value)
	case "cherrybloss":
		return fmt.Sprintf("%d%% cheaper", value)
	case "rulerjeanne":
		hours := float64(value) / 100.0
		return fmt.Sprintf("%.1fh cooldown", hours)
	case "youthyouthbyyoung":
		return fmt.Sprintf("+%d%% work bonus", value)
	case "kisslater":
		return fmt.Sprintf("+%d%% XP bonus", value)
	default:
		return fmt.Sprintf("+%d", value)
	}
}

func (h *EffectsHandler) createProgressBar(current, max int) string {
	if max <= 0 {
		return "▓▓▓▓▓▓▓▓▓▓"
	}

	filled := (current * 10) / max
	if filled > 10 {
		filled = 10
	}

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "▓"
	}
	for i := filled; i < 10; i++ {
		bar += "░"
	}

	return bar
}
