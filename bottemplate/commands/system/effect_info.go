package system

import (
	"context"
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var EffectInfo = discord.SlashCommandCreate{
	Name:        "effect",
	Description: "View detailed information about effects",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommand{
			Name:        "info",
			Description: "View detailed information about an effect",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "effect",
					Description: "The effect to view",
					Required:    true,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "upgrade",
			Description: "Upgrade an effect to the next tier",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "effect",
					Description: "The effect to upgrade",
					Required:    true,
				},
			},
		},
	},
}

type EffectInfoHandler struct {
	bot           *bottemplate.Bot
	effectManager *effects.Manager
}

func NewEffectInfoHandler(b *bottemplate.Bot, effectManager *effects.Manager) *EffectInfoHandler {
	return &EffectInfoHandler{
		bot:           b,
		effectManager: effectManager,
	}
}

func (h *EffectInfoHandler) Handle(event *handler.CommandEvent) error {
	data := event.SlashCommandInteractionData()

	subcommand := data.SubCommandName
	if subcommand == nil {
		return fmt.Errorf("no subcommand provided")
	}
	switch *subcommand {
	case "info":
		return h.handleInfo(event)
	case "upgrade":
		upgradeHandler := NewEffectUpgradeHandler(h.bot, h.effectManager)
		return upgradeHandler.HandleUpgrade(event)
	default:
		return utils.EH.CreateErrorEmbed(event, "Invalid subcommand")
	}
}

func (h *EffectInfoHandler) handleInfo(event *handler.CommandEvent) error {
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
		embed := discord.NewEmbedBuilder().
			SetTitle(fmt.Sprintf("ℹ️ %s", effectData.Name)).
			SetDescription(effectData.Description).
			SetColor(0x5865F2).
			AddField("Type", "This effect does not have a tier progression system", false).
			Build()

		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{embed},
		})
	}

	// Get user's effect status if they have it
	userEffect, err := h.effectManager.GetRepository().GetUserEffect(ctx, userID, effectID)
	hasEffect := err == nil && userEffect != nil && !userEffect.IsRecipe

	// Build description
	var description strings.Builder
	description.WriteString(fmt.Sprintf("**%s**\n\n", effectData.Description))

	// Current status if owned
	if hasEffect {
		stars := h.formatTierStars(userEffect.Tier)
		currentValue := 0
		if userEffect.Tier > 0 && userEffect.Tier <= len(effectData.TierData.Values) {
			currentValue = effectData.TierData.Values[userEffect.Tier-1]
		}

		description.WriteString(fmt.Sprintf("**Your Status:** %s (Tier %d)\n", stars, userEffect.Tier))
		description.WriteString(fmt.Sprintf("**Current Bonus:** %s\n", h.formatEffectValue(effectID, currentValue)))

		if userEffect.Tier < 5 {
			threshold := effectData.TierData.Thresholds[userEffect.Tier-1]
			description.WriteString(fmt.Sprintf("**Progress:** %d/%d (%d%%)\n\n",
				userEffect.Progress, threshold, (userEffect.Progress*100)/threshold))
		} else {
			description.WriteString("**Status:** 🌟 MAX TIER REACHED!\n\n")
		}
	}

	// Tier breakdown
	description.WriteString("**📊 Tier Progression:**\n```")
	for i := 0; i < len(effectData.TierData.Values); i++ {
		tier := i + 1
		value := effectData.TierData.Values[i]
		stars := h.formatTierStars(tier)

		// Highlight current tier
		if hasEffect && userEffect.Tier == tier {
			description.WriteString("→ ")
		} else {
			description.WriteString("  ")
		}

		description.WriteString(fmt.Sprintf("Tier %d %s: %s", tier, stars, h.formatEffectValue(effectID, value)))

		// Add requirement for next tier
		if i < len(effectData.TierData.Thresholds) {
			requirement := effectData.TierData.Thresholds[i]
			description.WriteString(fmt.Sprintf(" (need %d %s)", requirement, h.getActionName(effectID)))
		}

		description.WriteString("\n")
	}
	description.WriteString("```\n")

	// How to progress
	description.WriteString(fmt.Sprintf("\n**📈 How to Progress:**\n%s", h.getProgressDescription(effectID)))

	// Shop info if not owned
	if !hasEffect {
		description.WriteString(fmt.Sprintf("\n\n**💰 Price:** %s %s\n", utils.FormatNumber(effectData.Price), h.getCurrencyEmoji(effectData.Currency)))
		description.WriteString("**📦 Available in:** `/shop`")
	}

	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("📖 Effect Information: %s", effectData.Name)).
		SetDescription(description.String()).
		SetColor(0x5865F2).
		SetFooter(fmt.Sprintf("Requested by %s", event.User().Username), event.User().EffectiveAvatarURL()).
		Build()

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
}

func (h *EffectInfoHandler) formatTierStars(tier int) string {
	if tier >= 5 {
		return "🌟"
	}

	stars := ""
	for i := 0; i < tier; i++ {
		stars += "⭐"
	}

	return stars
}

func (h *EffectInfoHandler) formatEffectValue(effectID string, value int) string {
	switch effectID {
	case "cakeday":
		return fmt.Sprintf("+%d flakes/claim", value)
	case "holygrail":
		return fmt.Sprintf("+%d vials/liquify", value)
	case "skyfriend":
		return fmt.Sprintf("%d%% cashback from auction wins", value)
	case "lambhyejoo":
		return fmt.Sprintf("%d%% bonus from auction sales", value)
	case "cherrybloss":
		return fmt.Sprintf("%d%% cheaper forge/ascend", value)
	case "rulerjeanne":
		hours := float64(value) / 60.0
		return fmt.Sprintf("%.1fh daily cooldown", hours)
	case "youthyouth":
		return fmt.Sprintf("+%d%% work rewards", value)
	case "kisslater":
		return fmt.Sprintf("+%d%% levelup XP", value)
	default:
		return fmt.Sprintf("+%d", value)
	}
}

func (h *EffectInfoHandler) getActionName(effectID string) string {
	switch effectID {
	case "cakeday":
		return "claims"
	case "holygrail":
		return "liquefies"
	case "skyfriend":
		return "flakes spent on wins"
	case "lambhyejoo":
		return "flakes earned from sales"
	case "cherrybloss":
		return "forges/ascends"
	case "rulerjeanne":
		return "dailies"
	case "youthyouth":
		return "works"
	case "kisslater":
		return "levelups"
	default:
		return "actions"
	}
}

func (h *EffectInfoHandler) getProgressDescription(effectID string) string {
	switch effectID {
	case "cakeday":
		return "Use `/claim` to claim cards. Each claim counts toward your progress."
	case "holygrail":
		return "Use `/liquefy` to convert cards to vials. Each liquefied card counts."
	case "skyfriend":
		return "Win auctions to progress. The amount you spend on winning bids counts."
	case "lambhyejoo":
		return "Sell cards on auction. The flakes you earn from sales count."
	case "cherrybloss":
		return "Use `/forge` or `/ascend` commands. Each successful forge or ascend counts."
	case "rulerjeanne":
		return "Use `/daily` to collect your daily reward. Each daily collection counts."
	case "youthyouth":
		return "Use `/work` to earn rewards. Each work completion counts."
	case "kisslater":
		return "Use `/levelup` to level up cards. Each levelup counts."
	default:
		return "Progress by using the related game features."
	}
}

func (h *EffectInfoHandler) getCurrencyEmoji(currency string) string {
	switch currency {
	case "tomato":
		return "❄️"
	case "vials":
		return "🧪"
	default:
		return "💰"
	}
}
