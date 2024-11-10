package commands

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Init = discord.SlashCommandCreate{
	Name:        "init",
	Description: "Initialize database tables if they don't exist",
}

func InitHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		start := time.Now()
		defer func() {
			slog.Info("Command completed",
				slog.String("type", "cmd"),
				slog.String("name", "init"),
				slog.Duration("total_time", time.Since(start)),
			)
		}()

		// First, defer the response to let us process longer than 3 seconds
		if err := e.DeferCreateMessage(false); err != nil {
			return fmt.Errorf("failed to defer message: %w", err)
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Initialize schema
		if err := b.DB.InitializeSchema(ctx); err != nil {
			_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
				Embeds: &[]discord.Embed{
					{
						Title:       "❌ Database Initialization Failed",
						Description: fmt.Sprintf("```diff\n- Error: %s\n```", err.Error()),
						Color:       0xFF0000,
					},
				},
			})
			return err
		}

		// Get current timestamp in Unix format for Discord timestamp
		timestamp := fmt.Sprintf("<t:%d:F>", time.Now().Unix())

		// Create inline value for field booleans
		inlineTrue := true

		// Update the deferred response with success message
		_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds: &[]discord.Embed{
				{
					Title:       "✅ Database Initialized",
					Description: "All database tables have been successfully initialized.",
					Color:       0x00FF00,
					Fields: []discord.EmbedField{
						{
							Name: "Tables Created",
							Value: "• collections\n• cards\n• user_cards\n• user_quests\n" +
								"• user_slots\n• user_stats\n• users\n• user_effects",
							Inline: &inlineTrue,
						},
						{
							Name:   "Initialized At",
							Value:  timestamp,
							Inline: &inlineTrue,
						},
					},
					Footer: &discord.EmbedFooter{
						Text: "Database Initialization System",
					},
				},
			},
		})
		return err
	}
}
