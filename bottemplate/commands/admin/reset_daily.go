package admin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var ResetDaily = discord.SlashCommandCreate{
	Name:        "reset-daily",
	Description: "Reset a user's daily cooldown for testing purposes",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionUser{
			Name:        "user",
			Description: "The user whose daily cooldown to reset",
			Required:    true,
		},
	},
}

func ResetDailyHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		start := time.Now()
		defer func() {
			slog.Info("Command completed",
				slog.String("type", "cmd"),
				slog.String("name", "reset-daily"),
				slog.Duration("total_time", time.Since(start)),
			)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get the target user from command options
		data := e.SlashCommandInteractionData()
		targetUser := data.User("user")
		if targetUser.ID == 0 {
			return utils.EH.CreateErrorEmbed(e, "Invalid user specified.")
		}

		slog.Info("Resetting daily for user",
			slog.String("type", "cmd"),
			slog.String("admin_id", e.User().ID.String()),
			slog.String("target_user_id", targetUser.ID.String()),
			slog.String("target_username", targetUser.Username),
		)

		// Check if target user exists in database
		user, err := b.UserRepository.GetByDiscordID(ctx, targetUser.ID.String())
		if err != nil {
			slog.Error("Failed to get target user",
				slog.String("type", "db"),
				slog.String("target_user_id", targetUser.ID.String()),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Target user not found in database.")
		}

		// Start transaction for atomic operations
		tx, err := b.DB.BunDB().BeginTx(ctx, nil)
		if err != nil {
			slog.Error("Failed to start transaction",
				slog.String("type", "db"),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to reset daily. Please try again later.")
		}
		defer tx.Rollback()

		// Reset daily claims count
		if err := b.ClaimRepository.ResetDailyClaims(ctx, tx, user.DiscordID); err != nil {
			slog.Error("Failed to reset daily claims",
				slog.String("type", "db"),
				slog.String("target_user_id", user.DiscordID),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to reset daily claims. Please try again later.")
		}

		// Reset last daily timestamp to epoch (allows immediate daily)
		epochTime := time.Unix(0, 0)
		_, err = tx.NewUpdate().
			Model((*models.User)(nil)).
			Set("last_daily = ?", epochTime).
			Set("updated_at = ?", time.Now()).
			Where("discord_id = ?", user.DiscordID).
			Exec(ctx)
		if err != nil {
			slog.Error("Failed to reset last daily timestamp",
				slog.String("type", "db"),
				slog.String("target_user_id", user.DiscordID),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to reset daily timestamp. Please try again later.")
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			slog.Error("Failed to commit transaction",
				slog.String("type", "db"),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to reset daily. Please try again later.")
		}

		slog.Info("Daily reset successful",
			slog.String("type", "cmd"),
			slog.String("admin_id", e.User().ID.String()),
			slog.String("target_user_id", targetUser.ID.String()),
		)

		// Send success message
		return e.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				{
					Title:       "✅ Daily Reset Complete",
					Description: fmt.Sprintf("Successfully reset daily cooldown for **%s**.\n\nThey can now use `/daily` immediately.", targetUser.Username),
					Color:       utils.SuccessColor,
					Footer: &discord.EmbedFooter{
						Text: "Admin Testing Command",
					},
				},
			},
		})
	}
}
