// File: utils/embedhandler.go

package utils

import (
	"fmt"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// EmbedHandler provides common embed creation methods
type EmbedHandler struct{}

var EH = &EmbedHandler{}

func (h *EmbedHandler) CreateErrorEmbed(event *handler.CommandEvent, message string) error {
	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Description: message,
			Color:       0xFF0000,
		}},
	})
}

func (h *EmbedHandler) CreateSuccessEmbed(event *handler.CommandEvent, message string) error {
	return event.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Description: message,
			Color:       0x00FF00,
		}},
	})
}

func (eh *EmbedHandler) UpdateInteractionResponse(event *handler.CommandEvent, title, description string) error {
	_, err := event.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds: &[]discord.Embed{
			{
				Title:       "❌ " + title,
				Description: fmt.Sprintf("```diff\n- %s\n```", description),
				Color:       0xFF0000,
			},
		},
	})
	return err
}
