package system

import (
	"context"
	"fmt"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/economy/effects"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var CraftEffect = discord.SlashCommandCreate{
	Name:        "craft-effect",
	Description: "Craft an effect using recipe cards",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "effect",
			Description: "The effect to craft",
			Required:    true,
		},
	},
}

type CraftEffectHandler struct {
	bot           *bottemplate.Bot
	effectManager *effects.Manager
}

func NewCraftEffectHandler(b *bottemplate.Bot, effectManager *effects.Manager) *CraftEffectHandler {
	return &CraftEffectHandler{
		bot:           b,
		effectManager: effectManager,
	}
}

func (h *CraftEffectHandler) Handle(event *handler.CommandEvent) error {
	ctx := context.Background()

	effectID := event.SlashCommandInteractionData().String("effect")

	// Craft the effect
	err := h.effectManager.CraftEffect(ctx, event.User().ID.String(), effectID)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				{
					Title:       "❌ Crafting Failed",
					Description: err.Error(),
					Color:       config.ErrorColor,
				},
			},
			Flags: discord.MessageFlagEphemeral,
		})
	}

	// Create success embed
	embed := discord.NewEmbedBuilder().
		SetTitle("🔨 Effect Crafted Successfully").
		SetDescription(fmt.Sprintf("```md\n# Crafting Complete\n* Effect: %s\n* Added to your inventory\n* Recipe cards consumed\n```\n> 💡 **Tip**: Use `/inventory` to see your crafted effects!", effectID)).
		SetColor(config.SuccessColor).
		Build()

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
		Flags:  discord.MessageFlagEphemeral,
	})
}
