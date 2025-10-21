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
        // Defer immediately to avoid 3s timeout
        if err := e.DeferCreateMessage(false); err != nil {
            return err
        }
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

		user, err := b.UserRepository.GetByDiscordID(ctx, e.User().ID.String())
		if err != nil {
			slog.Error("Failed to get user",
				slog.String("type", "db"),
				slog.String("discord_id", e.User().ID.String()),
				slog.Any("error", err),
			)
            return utils.EH.UpdateInteractionResponse(e, "Error", "Failed to get user data. Please try again later.")
		}

		// Get dynamic daily cooldown (affected by rulerjeanne effect)
		cooldownHours := b.EffectIntegrator.GetDailyCooldown(ctx, e.User().ID.String())
		cooldownDuration := time.Duration(cooldownHours) * time.Hour

		// Check cooldown
		if time.Since(user.LastDaily) < cooldownDuration {
			remaining := time.Until(user.LastDaily.Add(cooldownDuration)).Round(time.Second)
            return utils.EH.UpdateInteractionResponse(e, "Daily Cooldown", fmt.Sprintf("You can claim your daily reward again in %s.", remaining))
		}

		// Calculate reward (consider streaks, bonuses, etc.)
		baseReward := int64(1000) // Basic reward

		// Apply passive effects with feedback
		effectResult := b.EffectIntegrator.ApplyDailyEffectsWithFeedback(ctx, e.User().ID.String(), int(baseReward))
		reward := int64(effectResult.GetValue().(int))

		// Update balance and last daily in a transaction
		tx, err := b.DB.BunDB().BeginTx(ctx, nil)
		if err != nil {
			slog.Error("Failed to start transaction",
				slog.String("type", "db"),
				slog.Any("error", err),
			)
            return utils.EH.UpdateInteractionResponse(e, "Error", "Failed to claim daily reward. Please try again later.")
		}
		defer tx.Rollback()

		// Reset daily claims when claiming daily reward
		if err := b.ClaimRepository.ResetDailyClaims(ctx, tx, user.DiscordID); err != nil {
			slog.Error("Failed to reset daily claims",
				slog.String("type", "db"),
				slog.String("discord_id", user.DiscordID),
				slog.Any("error", err),
			)
            return utils.EH.UpdateInteractionResponse(e, "Error", "Failed to claim daily reward. Please try again later.")
		}

		if err := b.UserRepository.UpdateBalance(ctx, user.DiscordID, user.Balance+reward); err != nil {
			slog.Error("Failed to update user balance",
				slog.String("type", "db"),
				slog.String("discord_id", user.DiscordID),
				slog.Any("error", err),
			)
            return utils.EH.UpdateInteractionResponse(e, "Error", "Failed to claim daily reward. Please try again later.")
		}

		if err := b.UserRepository.UpdateLastDaily(ctx, user.DiscordID); err != nil {
			slog.Error("Failed to update last daily",
				slog.String("type", "db"),
				slog.String("discord_id", user.DiscordID),
				slog.Any("error", err),
			)
            return utils.EH.UpdateInteractionResponse(e, "Error", "Failed to claim daily reward. Please try again later.")
		}

		if err := tx.Commit(); err != nil {
			slog.Error("Failed to commit transaction",
				slog.String("type", "db"),
				slog.Any("error", err),
			)
			return utils.EH.CreateErrorEmbed(e, "Failed to claim daily reward. Please try again later.")
		}

		// Track effect progress for Ruler Jeanne
		if b.EffectManager != nil {
			go b.EffectManager.UpdateEffectProgress(ctx, user.DiscordID, "rulerjeanne", 1)
		}

		// Build description with effect feedback
		description := fmt.Sprintf("You have claimed your daily reward of **%d** credits!", reward)

		// Add effect feedback if any effects were applied
		if effectResult.HasEffects() {
			effectMessages := effectResult.FormatEffectMessages()
			if len(effectMessages) > 0 {
				description += "\n\n**Effects Applied:**"
				for _, msg := range effectMessages {
					description += "\n" + msg
				}
			}
		}

		// Send success message
        _, updErr := e.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{{
            Title:       "Daily Reward Claimed!",
            Description: description,
            Color:       utils.SuccessColor,
        }}})
        return updErr
    }
}
