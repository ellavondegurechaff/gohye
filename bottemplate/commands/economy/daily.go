package economy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Daily = discord.SlashCommandCreate{
	Name:        "daily",
	Description: "Claim your daily reward!",
}

func DailyHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		user, err := b.UserRepository.GetByDiscordID(ctx, e.User().ID.String())
		if err != nil {
			slog.Error("Failed to get user",
				slog.String("type", "db"),
				slog.String("discord_id", e.User().ID.String()),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to get user data. Please try again later.")
		}

		// Get dynamic daily cooldown (affected by rulerjeanne effect)
		cooldownHours := b.EffectIntegrator.GetDailyCooldown(ctx, e.User().ID.String())
		cooldownDuration := time.Duration(cooldownHours) * time.Hour
		
		// Check cooldown
		if time.Since(user.LastDaily) < cooldownDuration {
			remaining := time.Until(user.LastDaily.Add(cooldownDuration)).Round(time.Second)
			return utils.EH.CreateErrorEmbed(e, fmt.Sprintf("You can claim your daily reward again in %s.", remaining))
		}

		// Calculate reward (consider streaks, bonuses, etc.)
		baseReward := int64(1000) // Basic reward
		
		// Apply passive effects
		reward := int64(b.EffectIntegrator.ApplyDailyEffects(ctx, e.User().ID.String(), int(baseReward)))

		// Update balance and last daily in a transaction
		tx, err := b.DB.BunDB().BeginTx(ctx, nil)
		if err != nil {
			slog.Error("Failed to start transaction",
				slog.String("type", "db"),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to claim daily reward. Please try again later.")
		}
		defer tx.Rollback()

		// Reset daily claims when claiming daily reward
		if err := b.ClaimRepository.ResetDailyClaims(ctx, tx, user.DiscordID); err != nil {
			slog.Error("Failed to reset daily claims",
				slog.String("type", "db"),
				slog.String("discord_id", user.DiscordID),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to claim daily reward. Please try again later.")
		}

		if err := b.UserRepository.UpdateBalance(ctx, user.DiscordID, user.Balance+reward); err != nil {
			slog.Error("Failed to update user balance",
				slog.String("type", "db"),
				slog.String("discord_id", user.DiscordID),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to claim daily reward. Please try again later.")
		}

		if err := b.UserRepository.UpdateLastDaily(ctx, user.DiscordID); err != nil {
			slog.Error("Failed to update last daily",
				slog.String("type", "db"),
				slog.String("discord_id", user.DiscordID),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to claim daily reward. Please try again later.")
		}

		if err := tx.Commit(); err != nil {
			slog.Error("Failed to commit transaction",
				slog.String("type", "db"),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to claim daily reward. Please try again later.")
		}

		// Send success message
		return e.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				{
					Title:       "Daily Reward Claimed!",
					Description: fmt.Sprintf("You have claimed your daily reward of %d credits!", reward),
					Color:       utils.SuccessColor,
				},
			},
		})
	}
}
