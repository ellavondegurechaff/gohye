package system

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var QuestClaimCommand = discord.SlashCommandCreate{
	Name:        "questclaim",
	Description: "🎁 Claim your completed quest rewards!",
}

func QuestClaimHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		userID := e.User().ID.String()

		// Get quest service
		questService := b.QuestService
		if questService == nil {
			return utils.EH.CreateErrorEmbed(e, "Quest system is not available right now. Please try again later.")
		}

		// Claim rewards
		result, err := questService.ClaimRewards(ctx, userID)
		if err != nil {
			slog.Error("Failed to claim quest rewards",
				slog.String("user_id", userID),
				slog.Any("error", err))

			// Provide more specific error message
			errorMsg := "Failed to claim rewards. Please try again."
			if strings.Contains(err.Error(), "no completed quests") {
				errorMsg = "You don't have any completed quests to claim!"
			} else if strings.Contains(err.Error(), "database") || strings.Contains(err.Error(), "SQLSTATE") {
				errorMsg = "A database error occurred. Please try again in a moment."
			}

			return utils.EH.CreateErrorEmbed(e, errorMsg)
		}

		if !result.Success {
			return utils.EH.CreateErrorEmbed(e, result.Message)
		}

		// Create success embed
		embed := createClaimEmbed(result, e.User().Username)

		// Add celebration reaction
		go func() {
			// Add some celebration emojis
			// Note: In production, you'd want to properly handle the message ID
			// This is simplified for the example
			time.Sleep(800 * time.Millisecond)
		}()

		return e.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{embed},
		})
	}
}

func createClaimEmbed(result *services.QuestRewardResult, username string) discord.Embed {
	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("🎉 Quest Rewards Claimed!")).
		SetColor(0x2ecc71) // Green

	// Build description
	description := fmt.Sprintf("**%s completed:**\n", username)

	// Add quest counts
	counts := []string{}
	if result.DailyCount > 0 {
		counts = append(counts, fmt.Sprintf("📅 %d daily quest%s", result.DailyCount, pluralize(result.DailyCount)))
	}
	if result.WeeklyCount > 0 {
		counts = append(counts, fmt.Sprintf("📆 %d weekly quest%s", result.WeeklyCount, pluralize(result.WeeklyCount)))
	}
	if result.MonthlyCount > 0 {
		counts = append(counts, fmt.Sprintf("🗓️ %d monthly quest%s", result.MonthlyCount, pluralize(result.MonthlyCount)))
	}

	if len(counts) > 0 {
		description += strings.Join(counts, "\n") + "\n\n"
	}

	// Add total rewards
	description += "**🎁 Total Rewards:**\n"

	rewards := []string{}
	if result.TotalSnowflakes > 0 {
		rewards = append(rewards, fmt.Sprintf("❄️ **%d** snowflakes", result.TotalSnowflakes))
	}
	if result.TotalVials > 0 {
		rewards = append(rewards, fmt.Sprintf("🧪 **%d** vials", result.TotalVials))
	}
	if result.TotalXP > 0 {
		rewards = append(rewards, fmt.Sprintf("⭐ **%d** XP", result.TotalXP))
	}

	if len(rewards) > 0 {
		description += strings.Join(rewards, "\n")
	}

	// Add individual quest details if not too many
	if len(result.ClaimedQuests) <= 5 {
		description += "\n\n**📋 Quest Details:**\n"
		for _, quest := range result.ClaimedQuests {
			tierEmoji := getTierEmoji(quest.Tier)
			description += fmt.Sprintf("%s **%s** (T%d)\n", tierEmoji, quest.QuestName, quest.Tier)
		}
	}

	embed.SetDescription(description)

	// Add motivational footer
	motivationalMessages := []string{
		"Keep up the great work, idol! 🌟",
		"You're on your way to stardom! ⭐",
		"Fighting! Your dedication is inspiring! 💪",
		"Amazing progress on your idol journey! 🎵",
		"You're shining brighter every day! ✨",
	}
	randomMessage := motivationalMessages[time.Now().Unix()%int64(len(motivationalMessages))]
	embed.SetFooter(randomMessage, "")

	// Add thumbnail based on total quests claimed
	totalQuests := result.DailyCount + result.WeeklyCount + result.MonthlyCount
	if totalQuests >= 5 {
		embed.SetThumbnail("https://i.imgur.com/4M34hi2.png") // Crown emoji or similar
	} else if totalQuests >= 3 {
		embed.SetThumbnail("https://i.imgur.com/AfFp7pu.png") // Star emoji or similar
	}

	return embed.Build()
}

func getTierEmoji(tier int) string {
	switch tier {
	case 1:
		return "🎵"
	case 2:
		return "🌟"
	case 3:
		return "👑"
	default:
		return "🎯"
	}
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
