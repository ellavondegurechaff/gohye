package system

import (
	"context"
	"fmt"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

type EffectUpgradeHandler struct {
	bot           *bottemplate.Bot
	effectManager *effects.Manager
}

func NewEffectUpgradeHandler(b *bottemplate.Bot, effectManager *effects.Manager) *EffectUpgradeHandler {
	return &EffectUpgradeHandler{
		bot:           b,
		effectManager: effectManager,
	}
}

func (h *EffectUpgradeHandler) HandleUpgrade(event *handler.CommandEvent) error {
	ctx := context.Background()
	userID := event.User().ID.String()
	effectID := event.SlashCommandInteractionData().String("effect")

	// Get effect definition
	effectData := effects.GetEffectItemByID(effectID)
	if effectData == nil {
		return utils.EH.CreateErrorEmbed(event, "Effect not found")
	}

	// Check if effect has tier data
	if effectData.TierData == nil {
		return utils.EH.CreateErrorEmbed(event, "This effect cannot be upgraded")
	}

	// Check if ready for upgrade
	canUpgrade, progress, threshold, err := h.effectManager.CheckEffectUpgrade(ctx, userID, effectID)
	if err != nil {
		return utils.EH.CreateErrorEmbed(event, fmt.Sprintf("Failed to check upgrade status: %v", err))
	}

	if !canUpgrade {
		// Get current tier
		userEffect, _ := h.effectManager.GetRepository().GetUserEffect(ctx, userID, effectID)
		if userEffect == nil {
			return utils.EH.CreateErrorEmbed(event, "You don't have this effect")
		}

		if userEffect.Tier >= 5 {
			embed := discord.NewEmbedBuilder().
				SetTitle("🌟 Maximum Tier Reached!").
				SetDescription(fmt.Sprintf("**%s** is already at maximum tier!\n\nYou've mastered this effect completely.", effectData.Name)).
				SetColor(0xF1C40F).
				Build()

			return event.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{embed},
			})
		}

		// Not enough progress
		percentage := (progress * 100) / threshold
		embed := discord.NewEmbedBuilder().
			SetTitle("❌ Not Ready for Upgrade").
			SetDescription(fmt.Sprintf("**%s** needs more progress before upgrading.\n\n**Progress:** %d/%d (%d%%)\n**Needed:** %d more %s",
				effectData.Name, progress, threshold, percentage, threshold-progress, h.getActionName(effectID))).
			SetColor(0xED4245).
			Build()

		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{embed},
			Flags:  discord.MessageFlagEphemeral,
		})
	}

	// Show upgrade confirmation
	userEffect, _ := h.effectManager.GetRepository().GetUserEffect(ctx, userID, effectID)
	currentTier := userEffect.Tier
	nextTier := currentTier + 1

	currentValue := effectData.TierData.Values[currentTier-1]
	nextValue := effectData.TierData.Values[nextTier-1]

	embed := discord.NewEmbedBuilder().
		SetTitle("🎊 TIER UPGRADE READY!").
		SetDescription(fmt.Sprintf("**%s**: Tier %d → Tier %d\n\n**Current:** %s\n**Upgrade:** %s\n\nUpgrade this effect to the next tier?",
			effectData.Name, currentTier, nextTier,
			h.formatEffectValue(effectID, currentValue),
			h.formatEffectValue(effectID, nextValue))).
		SetColor(0x57F287).
		Build()

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
		Components: []discord.ContainerComponent{
			discord.ActionRowComponent{
				discord.ButtonComponent{
					Label:    "✅ Upgrade",
					Style:    discord.ButtonStyleSuccess,
					CustomID: fmt.Sprintf("effect_upgrade_confirm_%s", effectID),
				},
				discord.ButtonComponent{
					Label:    "❌ Cancel",
					Style:    discord.ButtonStyleDanger,
					CustomID: "effect_upgrade_cancel",
				},
			},
		},
	})
}

func (h *EffectUpgradeHandler) HandleUpgradeConfirm(event *handler.ComponentEvent) error {
	ctx := context.Background()
	userID := event.User().ID.String()

	// Extract effect ID from custom ID
	customID := event.Data.CustomID()
	effectID := customID[len("effect_upgrade_confirm_"):]

	// Perform upgrade
	err := h.effectManager.UpgradeEffectTier(ctx, userID, effectID)
	if err != nil {
		_, err = event.UpdateInteractionResponse(discord.NewMessageUpdateBuilder().
			SetEmbeds(discord.NewEmbedBuilder().
				SetTitle("❌ Upgrade Failed").
				SetDescription(fmt.Sprintf("Failed to upgrade: %v", err)).
				SetColor(0xFF0000).
				Build()).
			Build())
		return err
	}

	// Get updated effect info
	effectData := effects.GetEffectItemByID(effectID)
	userEffect, _ := h.effectManager.GetRepository().GetUserEffect(ctx, userID, effectID)

	newValue := effectData.TierData.Values[userEffect.Tier-1]

	embed := discord.NewEmbedBuilder().
		SetTitle("🎉 Upgrade Successful!").
		SetDescription(fmt.Sprintf("**%s** has been upgraded to Tier %d!\n\n**New Bonus:** %s\n\nContinue using this effect to progress to the next tier!",
			effectData.Name, userEffect.Tier, h.formatEffectValue(effectID, newValue))).
		SetColor(0x57F287).
		Build()

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{},
	})
}

func (h *EffectUpgradeHandler) HandleUpgradeCancel(event *handler.ComponentEvent) error {
	embed := discord.NewEmbedBuilder().
		SetTitle("❌ Upgrade Cancelled").
		SetDescription("The upgrade has been cancelled. You can upgrade anytime when ready!").
		SetColor(0xED4245).
		Build()

	return event.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{},
	})
}

func (h *EffectUpgradeHandler) formatEffectValue(effectID string, value int) string {
	switch effectID {
	case "cakeday":
		return fmt.Sprintf("+%d flakes/claim", value)
	case "holygrail":
		return fmt.Sprintf("+%d vials/liquify", value)
	case "wolfofhyejoo":
		return fmt.Sprintf("%d%% cashback", value)
	case "lambofhyejoo":
		return fmt.Sprintf("%d%% bonus", value)
	case "cherrybloss":
		return fmt.Sprintf("%d%% cheaper", value)
	case "rulerjeanne":
		hours := float64(value) / 100.0
		return fmt.Sprintf("%.1fh cooldown", hours)
	case "youthyouthbyyoung":
		return fmt.Sprintf("+%d%% bonus", value)
	case "kisslater":
		return fmt.Sprintf("+%d%% XP", value)
	default:
		return fmt.Sprintf("+%d", value)
	}
}

func (h *EffectUpgradeHandler) getActionName(effectID string) string {
	switch effectID {
	case "cakeday":
		return "claims"
	case "holygrail":
		return "liquefies"
	case "wolfofhyejoo":
		return "flakes spent"
	case "lambofhyejoo":
		return "flakes earned"
	case "cherrybloss":
		return "forges/ascends"
	case "rulerjeanne":
		return "dailies"
	case "youthyouthbyyoung":
		return "works"
	case "kisslater":
		return "levelups"
	default:
		return "actions"
	}
}
