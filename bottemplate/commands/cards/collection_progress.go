package cards

import (
	"bytes"
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

var CollectionProgress = discord.SlashCommandCreate{
	Name:        "collection-progress",
	Description: "View collection completion leaderboard (generates image)",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "collection",
			Description: "Collection name or alias",
			Required:    true,
		},
	},
}

func CollectionProgressHandler(b *bottemplate.Bot) handler.CommandHandler {
	collectionService := services.NewCollectionService(b.CollectionRepository, b.CardRepository, b.UserCardRepository)
	imageService := services.NewLeaderboardImageService()

	return func(event *handler.CommandEvent) error {
		start := time.Now()
		userID := event.User().ID.String()

		slog.Info("Collection-progress command started",
			slog.String("type", "cmd"),
			slog.String("name", "collection-progress"),
			slog.String("user_id", userID))

		defer func() {
			slog.Info("Collection-progress command completed",
				slog.String("type", "cmd"),
				slog.String("name", "collection-progress"),
				slog.String("user_id", userID),
				slog.Duration("total_time", time.Since(start)))
		}()

		ctx := context.Background()

		collectionQuery := strings.TrimSpace(event.SlashCommandInteractionData().String("collection"))
		if collectionQuery == "" {
			return utils.EH.CreateErrorEmbed(event, "Collection parameter is required")
		}

		// Always generate image, limit to top 5, defer response
		limit := 5
    if err := event.DeferCreateMessage(false); err != nil {
        return utils.EH.CreateErrorEmbed(event, "Failed to defer message")
    }

    slog.Info("Collection-progress parameters parsed",
        slog.String("type", "cmd"),
        slog.String("name", "collection-progress"),
        slog.String("user_id", userID),
        slog.String("collection_query", collectionQuery),
        slog.Int("limit", limit))

    // Run heavy work asynchronously and return immediately
    go func() {
        collections, err := b.CollectionRepository.SearchCollections(ctx, collectionQuery)
        if err != nil {
            _ = utils.EH.CreateErrorEmbed(event, "Failed to search collections")
            return
        }
        if len(collections) == 0 {
            _, _ = event.CreateFollowupMessage(discord.MessageCreate{Content: fmt.Sprintf("❌ **Collection Not Found**\nNo collection found matching '%s'", collectionQuery)})
            return
        }
        collection := collections[0]
        progressResults, err := collectionService.GetCollectionLeaderboard(ctx, collection.ID, limit)
        if err != nil {
            slog.Error("Failed to get collection progress",
                slog.String("type", "cmd"),
                slog.String("name", "collection-progress"),
                slog.String("user_id", userID),
                slog.String("collection_id", collection.ID),
                slog.String("error", err.Error()))
            _ = utils.EH.CreateErrorEmbed(event, "Failed to load collection progress data")
            return
        }
        if len(progressResults) == 0 {
            _, _ = event.CreateFollowupMessage(discord.MessageCreate{Content: fmt.Sprintf("📊 **%s - Collection Progress**\nNo users have any cards from this collection yet.", collection.Name)})
            return
        }
        imageBytes, err := imageService.GenerateLeaderboardImage(ctx, collection.Name, collection.ID, progressResults)
        if err != nil {
            slog.Error("Failed to generate leaderboard image",
                slog.String("type", "cmd"),
                slog.String("name", "collection-progress"),
                slog.String("user_id", userID),
                slog.String("collection_id", collection.ID),
                slog.String("error", err.Error()))
            _, _ = event.CreateFollowupMessage(discord.MessageCreate{Content: fmt.Sprintf("❌ **Image Generation Failed**\nSorry, I couldn't generate the leaderboard image for %s. Please try again later.", collection.Name)})
            return
        }
        _, err = event.CreateFollowupMessage(discord.MessageCreate{
            Content: fmt.Sprintf("🏆 **%s - Collection Progress Leaderboard**", collection.Name),
            Files: []*discord.File{{
                Name:   fmt.Sprintf("%s_leaderboard_%d.png", collection.ID, time.Now().Unix()),
                Reader: bytes.NewReader(imageBytes),
            }},
        })
        if err != nil {
            slog.Error("Failed to send image to Discord",
                slog.String("type", "cmd"),
                slog.String("name", "collection-progress"),
                slog.String("user_id", userID),
                slog.String("error", err.Error()))
        }
    }()

    return nil
	}
}
