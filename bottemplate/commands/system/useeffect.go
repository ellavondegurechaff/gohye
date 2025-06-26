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

var UseEffect = discord.SlashCommandCreate{
	Name:        "use-effect",
	Description: "Use an active effect from your inventory",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "effect",
			Description: "The effect to use",
			Required:    true,
		},
		discord.ApplicationCommandOptionString{
			Name:        "arguments",
			Description: "Additional arguments for the effect (if required)",
			Required:    false,
		},
	},
}

type UseEffectHandler struct {
	bot           *bottemplate.Bot
	effectManager *effects.Manager
}

func NewUseEffectHandler(b *bottemplate.Bot, effectManager *effects.Manager) *UseEffectHandler {
	return &UseEffectHandler{
		bot:           b,
		effectManager: effectManager,
	}
}

func (h *UseEffectHandler) Handle(event *handler.CommandEvent) error {
	ctx := context.Background()

	effectID := event.SlashCommandInteractionData().String("effect")
	args := event.SlashCommandInteractionData().String("arguments")

	// Use the effect
	result, err := h.effectManager.UseActiveEffect(ctx, event.User().ID.String(), effectID, args)
	if err != nil {
		return event.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				{
					Title:       "❌ Effect Failed",
					Description: err.Error(),
					Color:       config.ErrorColor,
				},
			},
			Flags: discord.MessageFlagEphemeral,
		})
	}

	// Create success embed
	embed := discord.NewEmbedBuilder().
		SetTitle("✅ Effect Used").
		SetDescription(fmt.Sprintf("```md\n# Effect Result\n%s\n```", result)).
		SetColor(config.SuccessColor).
		Build()

	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
		Flags:  discord.MessageFlagEphemeral,
	})
}