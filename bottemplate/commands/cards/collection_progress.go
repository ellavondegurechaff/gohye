package cards

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var CollectionProgress = discord.SlashCommandCreate{
	Name:        "collection-progress",
	Description: "View collection completion leaderboard",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "collection",
			Description: "Collection name or alias",
			Required:    true,
		},
		discord.ApplicationCommandOptionInt{
			Name:        "limit",
			Description: "Number of top users to show (default: 10, max: 25)",
			Required:    false,
		},
	},
}


func CollectionProgressHandler(b *bottemplate.Bot) handler.CommandHandler {
	collectionService := services.NewCollectionService(b.CollectionRepository, b.CardRepository, b.UserCardRepository)

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

		limit := 10
		if limitValue := event.SlashCommandInteractionData().Int("limit"); limitValue != 0 {
			limit = int(limitValue)
			if limit > 25 {
				limit = 25
			}
			if limit < 1 {
				limit = 1
			}
		}

		collections, err := b.CollectionRepository.SearchCollections(ctx, collectionQuery)
		if err != nil {
			return utils.EH.CreateErrorEmbed(event, "Failed to search collections")
		}

		if len(collections) == 0 {
			embed := discord.Embed{
				Title:       "Collection Not Found",
				Description: fmt.Sprintf("No collection found matching '%s'", collectionQuery),
				Color:       config.ErrorColor,
			}
			return event.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}})
		}

		collection := collections[0]

		// Get collection progress leaderboard using optimized SQL aggregation
		progressResults, err := collectionService.GetCollectionLeaderboard(ctx, collection.ID, limit)
		if err != nil {
			slog.Error("Failed to get collection progress", 
				slog.String("type", "cmd"),
				slog.String("name", "collection-progress"),
				slog.String("user_id", userID),
				slog.String("collection_id", collection.ID),
				slog.String("error", err.Error()))
			return utils.EH.CreateErrorEmbed(event, "Failed to load collection progress data")
		}

		if len(progressResults) == 0 {
			embed := discord.Embed{
				Title:       fmt.Sprintf("%s - Collection Progress", collection.Name),
				Description: "No users have any cards from this collection yet.",
				Color:       config.BackgroundColor,
			}
			return event.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}})
		}

		// Build leaderboard description
		var description strings.Builder
		description.WriteString(fmt.Sprintf("**Top %d users by completion:**\n\n", len(progressResults)))

		for i, result := range progressResults {
			rank := i + 1
			var medal string
			switch rank {
			case 1:
				medal = "🥇"
			case 2:
				medal = "🥈"
			case 3:
				medal = "🥉"
			default:
				medal = fmt.Sprintf("**%d.**", rank)
			}
			
			description.WriteString(fmt.Sprintf("%s **%s** - %d cards (%.1f%%)\n",
				medal, result.Username, result.OwnedCards, result.Progress))
		}

		embed := discord.Embed{
			Title:       fmt.Sprintf("%s - Collection Progress Leaderboard", collection.Name),
			Description: description.String(),
			Color:       config.SuccessColor,
			Footer: &discord.EmbedFooter{
				Text: fmt.Sprintf("Showing top %d • Updated %s", 
					len(progressResults), time.Now().Format("15:04 MST")),
			},
		}
		return event.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}})
	}
}