package system

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var QuestsCommand = discord.SlashCommandCreate{
	Name:        "quests",
	Description: "🎵 View your K-pop idol journey quests!",
}

func QuestsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		userID := e.User().ID.String()

		// Ensure user exists
		user, err := b.UserRepository.GetByDiscordID(ctx, userID)
		if err != nil {
			slog.Error("Failed to get user",
				slog.String("user_id", userID),
				slog.Any("error", err))
			return utils.EH.CreateErrorEmbed(e, "Failed to load user data. Please try again.")
		}

		// Get quest service
		questService := b.QuestService
		if questService == nil {
			return utils.EH.CreateErrorEmbed(e, "Quest system is not available right now. Please try again later.")
		}

		// NOTE: Removed automatic quest assignment - users must wait for reset periods
		// Quests are now only assigned during daily/weekly/monthly resets

		// Get user's quest status
		status, err := questService.GetUserQuestStatus(ctx, userID)
		if err != nil {
			slog.Error("Failed to get quest status",
				slog.String("user_id", userID),
				slog.Any("error", err))
			return utils.EH.CreateErrorEmbed(e, "Failed to load your quests. Please try again.")
		}

		// Create initial embed (Daily quests by default)
		embed := createQuestEmbed(status.DailyQuests, "Daily", user.Username, "daily")

		// Create components for navigation
		components := createQuestComponents("daily", userID)

		return e.CreateMessage(discord.MessageCreate{
			Embeds:     []discord.Embed{embed},
			Components: components,
		})
	}
}

// QuestComponentHandler handles quest navigation
func QuestComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		customID := e.Data.CustomID()

		// Parse component ID: /quest/{action}/{userID}
		parts := strings.Split(customID, "/")
		if len(parts) < 4 {
			return utils.EH.CreateEphemeralError(e, "Invalid quest component")
		}

		action := parts[2]
		originalUserID := parts[3]

		// Verify it's the same user
		if e.User().ID.String() != originalUserID {
			return utils.EH.CreateEphemeralError(e, "You can only interact with your own quests!")
		}

		// Handle claim button
		if action == "claim" {
			return handleQuestClaim(b, e)
		}

		// Handle navigation buttons (daily, weekly, monthly)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get quest service
		questService := b.QuestService
		if questService == nil {
			return utils.EH.CreateEphemeralError(e, "Quest system is not available")
		}

		// Get user's quest status
		status, err := questService.GetUserQuestStatus(ctx, originalUserID)
		if err != nil {
			return utils.EH.CreateEphemeralError(e, "Failed to load quests")
		}

		// Get user for display
		user, err := b.UserRepository.GetByDiscordID(ctx, originalUserID)
		if err != nil {
			return utils.EH.CreateEphemeralError(e, "Failed to load user data")
		}

		// Select appropriate quests based on type
		var quests []*models.UserQuestProgress
		var displayType string

		switch action {
		case "daily":
			quests = status.DailyQuests
			displayType = "Daily"
		case "weekly":
			quests = status.WeeklyQuests
			displayType = "Weekly"
		case "monthly":
			quests = status.MonthlyQuests
			displayType = "Monthly"
		default:
			return utils.EH.CreateEphemeralError(e, "Invalid quest type")
		}

		// Create updated embed
		embed := createQuestEmbed(quests, displayType, user.Username, action)

		// Create components
		components := createQuestComponents(action, originalUserID)

		return e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &components,
		})
	}
}

// handleQuestClaim handles the claim button interaction
func handleQuestClaim(b *bottemplate.Bot, e *handler.ComponentEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userID := e.User().ID.String()

	// Get quest service
	questService := b.QuestService
	if questService == nil {
		return utils.EH.CreateEphemeralError(e, "Quest system is not available right now.")
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

		return utils.EH.CreateEphemeralError(e, errorMsg)
	}

	if !result.Success {
		return utils.EH.CreateEphemeralError(e, result.Message)
	}

	// Create success embed (ephemeral to avoid spam)
	embed := createClaimSummaryEmbed(result, e.User().Username)

	// Update the original message to refresh quest status
	go func() {
		// Small delay to ensure database is updated
		time.Sleep(500 * time.Millisecond)

		// Get updated quest status
		status, err := questService.GetUserQuestStatus(ctx, userID)
		if err != nil {
			return
		}

		// Get user for display
		user, err := b.UserRepository.GetByDiscordID(ctx, userID)
		if err != nil {
			return
		}

		// Default to daily view after claiming
		quests := status.DailyQuests
		embed := createQuestEmbed(quests, "Daily", user.Username, "daily")
		components := createQuestComponents("daily", userID)

		// Update the original message
		e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &components,
		})
	}()

	// Send ephemeral success message
	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
		Flags:  discord.MessageFlagEphemeral,
	})
}

// createClaimSummaryEmbed creates a summary embed for claimed rewards
func createClaimSummaryEmbed(result *services.QuestRewardResult, username string) discord.Embed {
	embed := discord.NewEmbedBuilder().
		SetTitle("🎉 Quest Rewards Claimed!").
		SetColor(0x2ecc71) // Green

	// Build description
	description := "**Successfully claimed:**\n"

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
		description += strings.Join(counts, ", ") + "\n\n"
	}

	// Add total rewards
	description += "**Total Rewards:**\n"
	if result.TotalSnowflakes > 0 {
		description += fmt.Sprintf("❄️ **%d** snowflakes\n", result.TotalSnowflakes)
	}
	if result.TotalVials > 0 {
		description += fmt.Sprintf("🧪 **%d** vials\n", result.TotalVials)
	}
	if result.TotalXP > 0 {
		description += fmt.Sprintf("⭐ **%d** XP\n", result.TotalXP)
	}

	embed.SetDescription(description)
	embed.SetFooter("Your rewards have been added to your account!", "")

	return embed.Build()
}

func createQuestEmbed(quests []*models.UserQuestProgress, questType string, username string, typeKey string) discord.Embed {
	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("🎵 %s's %s Quests", username, questType)).
		SetColor(getQuestTypeColor(typeKey))

	if len(quests) == 0 {
		nextReset := getNextResetTime(typeKey)
		timeUntilReset := time.Until(nextReset)
		embed.SetDescription(fmt.Sprintf("No quests available!\n\n⏰ New quests arrive in: %s", formatDuration(timeUntilReset)))
		return embed.Build()
	}

	// Count completed quests
	totalQuests := 0
	completedQuests := 0
	claimedQuests := 0

	for _, quest := range quests {
		if quest.QuestDefinition != nil {
			totalQuests++
			if quest.Completed {
				completedQuests++
				if quest.Claimed {
					claimedQuests++
				}
			}
		}
	}

	// Sort quests by tier
	description := ""
	for tier := 1; tier <= 3; tier++ {
		tierQuests := filterQuestsByTier(quests, tier)
		if len(tierQuests) == 0 {
			continue
		}

		// Add tier header
		tierName := getTierName(tier)
		description += fmt.Sprintf("**__%s__**\n", tierName)

		for _, quest := range tierQuests {
			if quest.QuestDefinition == nil {
				continue
			}

			// Progress bar
			progressBar := createProgressBar(quest)

			// Status emoji and text
			statusEmoji := "⏳"
			statusText := ""
			if quest.Completed && quest.Claimed {
				statusEmoji = "✅"
				statusText = " *(Completed & Claimed)*"
			} else if quest.Completed {
				statusEmoji = "🎁"
				statusText = " *(Ready to claim!)*"
			}

			// Quest line
			questLine := fmt.Sprintf("%s **%s**%s\n", statusEmoji, quest.QuestDefinition.Name, statusText)
			questLine += fmt.Sprintf("└ %s\n", quest.QuestDefinition.Description)

			// Only show progress if not claimed
			if !quest.Claimed {
				questLine += fmt.Sprintf("└ Progress: %s %d/%d\n",
					progressBar,
					quest.CurrentProgress,
					quest.QuestDefinition.RequirementCount)
			}

			// Add rewards preview
			rewards := []string{}
			if quest.QuestDefinition.RewardSnowflakes > 0 {
				rewards = append(rewards, fmt.Sprintf("❄️ %d", quest.QuestDefinition.RewardSnowflakes))
			}
			if quest.QuestDefinition.RewardVials > 0 {
				rewards = append(rewards, fmt.Sprintf("🧪 %d", quest.QuestDefinition.RewardVials))
			}
			if quest.QuestDefinition.RewardXP > 0 {
				rewards = append(rewards, fmt.Sprintf("⭐ %d XP", quest.QuestDefinition.RewardXP))
			}
			if len(rewards) > 0 {
				questLine += fmt.Sprintf("└ Rewards: %s\n", strings.Join(rewards, " • "))
			}

			description += questLine + "\n"
		}
	}

	// Add summary if all quests are completed
	if claimedQuests == totalQuests && totalQuests > 0 {
		nextReset := getNextResetTime(typeKey)
		timeUntilReset := time.Until(nextReset)
		description += fmt.Sprintf("\n✨ **All quests completed!**\n⏰ New quests in: %s", formatDuration(timeUntilReset))
	} else if len(quests) > 0 && quests[0].ExpiresAt.After(time.Now()) {
		// Add expiration info
		timeLeft := time.Until(quests[0].ExpiresAt)
		description += fmt.Sprintf("\n⏰ Resets in: %s", formatDuration(timeLeft))
	}

	embed.SetDescription(description)

	// Add footer with contextual tips
	tips := []string{}
	if completedQuests > claimedQuests {
		tips = append(tips, "You have unclaimed rewards! Click 'Claim Rewards' below!")
	} else if claimedQuests == totalQuests && totalQuests > 0 {
		tips = append(tips, "Great job completing all quests! Come back after reset for more!")
	} else {
		tips = append(tips,
			"Complete quests to earn rewards!",
			"Higher tier quests give better rewards!",
			"Quests reset daily, weekly, and monthly!",
			"Complete quest chains for bonus rewards!",
		)
	}

	if len(tips) > 0 {
		randomTip := tips[time.Now().Unix()%int64(len(tips))]
		embed.SetFooter(fmt.Sprintf("💡 %s", randomTip), "")
	}

	return embed.Build()
}

func createQuestComponents(activeType string, userID string) []discord.ContainerComponent {
	buttons := []discord.InteractiveComponent{
		discord.NewPrimaryButton("Daily", fmt.Sprintf("/quest/daily/%s", userID)).
			WithEmoji(discord.ComponentEmoji{Name: "📅"}).
			WithDisabled(activeType == "daily"),
		discord.NewPrimaryButton("Weekly", fmt.Sprintf("/quest/weekly/%s", userID)).
			WithEmoji(discord.ComponentEmoji{Name: "📆"}).
			WithDisabled(activeType == "weekly"),
		discord.NewPrimaryButton("Monthly", fmt.Sprintf("/quest/monthly/%s", userID)).
			WithEmoji(discord.ComponentEmoji{Name: "🗓️"}).
			WithDisabled(activeType == "monthly"),
	}

	// Add claim button - using custom ID that matches the registered pattern
	claimButton := discord.NewSuccessButton("Claim Rewards", fmt.Sprintf("/quest/claim/%s", userID)).
		WithEmoji(discord.ComponentEmoji{Name: "🎁"})

	return []discord.ContainerComponent{
		discord.NewActionRow(buttons...),
		discord.NewActionRow(claimButton),
	}
}

func createProgressBar(quest *models.UserQuestProgress) string {
	percentage := quest.GetProgressPercentage()
	filled := int(percentage / 10)
	empty := 10 - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}

	// Add milestone indicators
	if quest.Milestone75 {
		bar += " ⭐⭐⭐"
	} else if quest.Milestone50 {
		bar += " ⭐⭐"
	} else if quest.Milestone25 {
		bar += " ⭐"
	}

	return bar
}

func getQuestTypeColor(questType string) int {
	switch questType {
	case "daily":
		return 0x3498db // Blue
	case "weekly":
		return 0x9b59b6 // Purple
	case "monthly":
		return 0xe74c3c // Red
	default:
		return 0x95a5a6 // Gray
	}
}

func getTierName(tier int) string {
	switch tier {
	case 1:
		return "🎵 Tier 1 - Trainee"
	case 2:
		return "🌟 Tier 2 - Debut"
	case 3:
		return "👑 Tier 3 - Idol"
	default:
		return fmt.Sprintf("Tier %d", tier)
	}
}

func filterQuestsByTier(quests []*models.UserQuestProgress, tier int) []*models.UserQuestProgress {
	var filtered []*models.UserQuestProgress
	for _, q := range quests {
		if q.QuestDefinition != nil && q.QuestDefinition.Tier == tier {
			filtered = append(filtered, q)
		}
	}
	return filtered
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}

	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func getNextResetTime(questType string) time.Time {
	now := time.Now()

	switch questType {
	case "daily":
		// Next day at midnight
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())

	case "weekly":
		// Next Monday at midnight
		days := (7 - int(now.Weekday()) + 1) % 7
		if days == 0 {
			days = 7
		}
		return time.Date(now.Year(), now.Month(), now.Day()+days, 0, 0, 0, 0, now.Location())

	case "monthly":
		// First day of next month at midnight
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())

	default:
		// Default to 24 hours
		return now.Add(24 * time.Hour)
	}
}
