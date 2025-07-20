package system

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Profile = discord.SlashCommandCreate{
	Name:        "profile",
	Description: "View your profile card with stats and info",
}

func ProfileHandler(b *bottemplate.Bot) handler.CommandHandler {
	imageService := services.NewProfileImageService()

	return func(event *handler.CommandEvent) error {
		start := time.Now()
		userID := event.User().ID.String()

		slog.Info("Profile command started",
			slog.String("type", "cmd"),
			slog.String("name", "profile"),
			slog.String("user_id", userID))

		defer func() {
			slog.Info("Profile command completed",
				slog.String("type", "cmd"),
				slog.String("name", "profile"),
				slog.String("user_id", userID),
				slog.Duration("total_time", time.Since(start)))
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get user data
		user, err := b.UserRepository.GetByDiscordID(ctx, userID)
		if err != nil {
			slog.Error("Failed to get user data",
				slog.String("user_id", userID),
				slog.String("error", err.Error()))
			return utils.EH.CreateErrorEmbed(event, "Failed to get your profile data. Please try again later.")
		}

		// Get actual card count from user_cards table
		userCards, err := b.UserCardRepository.GetAllByUserID(ctx, userID)
		if err != nil {
			slog.Error("Failed to get user cards",
				slog.String("user_id", userID),
				slog.String("error", err.Error()))
			return utils.EH.CreateErrorEmbed(event, "Failed to get your card data. Please try again later.")
		}
		cardCount := len(userCards)

		// Calculate daily streak (placeholder - using 0 for now since streak tracking needs investigation)
		dailyStreak := 0

		// Calculate simple rank based on card count (placeholder)
		rank := calculateUserRank(cardCount)

		// Check premium status and background
		isPremium := user.Premium
		var backgroundImage string
		if isPremium && user.Preferences != nil && user.Preferences.Profile.Image != "" {
			backgroundImage = user.Preferences.Profile.Image
		}

		// Generate profile image
		slog.Info("Generating profile image",
			slog.String("username", user.Username),
			slog.Int("card_count", cardCount),
			slog.Bool("is_premium", isPremium))

		imageBytes, err := imageService.GenerateProfileImage(
			ctx,
			user.Username,
			user.Joined,
			cardCount,
			dailyStreak,
			rank,
			isPremium,
			backgroundImage,
		)
		if err != nil {
			slog.Error("Failed to generate profile image",
				slog.String("user_id", userID),
				slog.String("error", err.Error()))
			return utils.EH.CreateErrorEmbed(event, "Failed to generate your profile image. Please try again later.")
		}

		// Create Discord file attachment
		file := discord.File{
			Name:        fmt.Sprintf("profile_%s.png", user.Username),
			Description: fmt.Sprintf("%s's profile card", user.Username),
			Reader:      bytes.NewReader(imageBytes),
		}

		// Send the profile image
		return event.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("🎯 **%s's Profile**", user.Username),
			Files:   []*discord.File{&file},
		})
	}
}

func calculateUserRank(cardCount int) string {
	switch {
	case cardCount >= 1000:
		return "Elite"
	case cardCount >= 500:
		return "Master"
	case cardCount >= 250:
		return "Expert"
	case cardCount >= 100:
		return "Advanced"
	case cardCount >= 50:
		return "Regular"
	case cardCount >= 10:
		return "Novice"
	default:
		return "Rookie"
	}
}
