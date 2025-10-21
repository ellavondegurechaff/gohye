package admin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
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

        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
            defer cancel()
            if err := b.DB.InitializeSchema(ctx); err != nil {
                _, _ = e.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{{
                    Title:       "❌ Database Initialization Failed",
                    Description: fmt.Sprintf("```diff\n- Error: %s\n```", err.Error()),
                    Color:       config.ErrorColor,
                }}})
                return
            }
            timestamp := fmt.Sprintf("<t:%d:F>", time.Now().Unix())
            inlineTrue := true
            _, _ = e.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{{
                Title:       "✅ Database Initialized",
                Description: "All database tables have been successfully initialized.",
                Color:       config.SuccessColor,
                Fields: []discord.EmbedField{{
                    Name:  "Tables Created",
                    Value: "• collections\n• cards\n• user_cards\n• user_quests\n" +
                        "• user_slots\n• user_stats\n• users\n• user_effects\n• claims",
                    Inline: &inlineTrue,
                }, {
                    Name:   "Initialized At",
                    Value:  timestamp,
                    Inline: &inlineTrue,
                }},
                Footer: &discord.EmbedFooter{Text: "Database Initialization System"},
            }}})
        }()
        return nil
	}
}
