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

var CollectionInfo = discord.SlashCommandCreate{
	Name:        "collection-info",
	Description: "View detailed information about a collection",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "collection",
			Description: "Collection name or alias",
			Required:    true,
		},
	},
}

func CollectionInfoHandler(b *bottemplate.Bot) handler.CommandHandler {
	collectionService := services.NewCollectionService(b.CollectionRepository, b.CardRepository, b.UserCardRepository)

	return func(event *handler.CommandEvent) error {
		start := time.Now()
		userID := event.User().ID.String()
		
		slog.Info("Collection-info command started",
			slog.String("type", "cmd"),
			slog.String("name", "collection-info"),
			slog.String("user_id", userID))
		
		defer func() {
			slog.Info("Collection-info command completed",
				slog.String("type", "cmd"),
				slog.String("name", "collection-info"),
				slog.String("user_id", userID),
				slog.Duration("total_time", time.Since(start)))
		}()

		ctx := context.Background()
		
		slog.Debug("Attempting to get user by Discord ID",
			slog.String("type", "cmd"),
			slog.String("name", "collection-info"),
			slog.String("user_id", userID))
		
		user, err := b.UserRepository.GetByDiscordID(ctx, userID)
		if err != nil {
			slog.Error("Failed to get user data",
				slog.String("type", "cmd"),
				slog.String("name", "collection-info"),
				slog.String("user_id", userID),
				slog.String("error", err.Error()))
			return utils.EH.CreateErrorEmbed(event, "Failed to get user data")
		}
		
		slog.Debug("Successfully retrieved user data",
			slog.String("type", "cmd"),
			slog.String("name", "collection-info"),
			slog.String("user_id", userID),
			slog.String("username", user.Username))

		collectionQuery := strings.TrimSpace(event.SlashCommandInteractionData().String("collection"))
		if collectionQuery == "" {
			slog.Warn("Collection parameter is empty",
				slog.String("type", "cmd"),
				slog.String("name", "collection-info"),
				slog.String("user_id", userID))
			return utils.EH.CreateErrorEmbed(event, "Collection parameter is required")
		}

		slog.Debug("Searching for collections",
			slog.String("type", "cmd"),
			slog.String("name", "collection-info"),
			slog.String("user_id", userID),
			slog.String("query", collectionQuery))

		collections, err := b.CollectionRepository.SearchCollections(ctx, collectionQuery)
		if err != nil {
			slog.Error("Failed to search collections",
				slog.String("type", "cmd"),
				slog.String("name", "collection-info"),
				slog.String("user_id", userID),
				slog.String("query", collectionQuery),
				slog.String("error", err.Error()))
			return utils.EH.CreateErrorEmbed(event, "Failed to search collections")
		}
		
		slog.Debug("Collection search completed",
			slog.String("type", "cmd"),
			slog.String("name", "collection-info"),
			slog.String("user_id", userID),
			slog.String("query", collectionQuery),
			slog.Int("results_count", len(collections)))

		if len(collections) == 0 {
			embed := discord.Embed{
				Title:       "Collection Not Found",
				Description: fmt.Sprintf("No collection found matching '%s'", collectionQuery),
				Color:       config.ErrorColor,
			}
			return event.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}})
		}

		collection := collections[0]

		slog.Debug("Calculating collection progress",
			slog.String("type", "cmd"),
			slog.String("name", "collection-info"),
			slog.String("user_id", userID),
			slog.String("collection_id", collection.ID),
			slog.String("collection_name", collection.Name))

		progress, err := collectionService.CalculateProgress(ctx, user.DiscordID, collection.ID)
		if err != nil {
			slog.Error("Failed to calculate collection progress",
				slog.String("type", "cmd"),
				slog.String("name", "collection-info"),
				slog.String("user_id", userID),
				slog.String("collection_id", collection.ID),
				slog.String("error", err.Error()))
			return utils.EH.CreateErrorEmbed(event, "Failed to calculate collection progress")
		}
		
		slog.Debug("Collection progress calculated",
			slog.String("type", "cmd"),
			slog.String("name", "collection-info"),
			slog.String("user_id", userID),
			slog.String("collection_id", collection.ID),
			slog.Float64("percentage", progress.Percentage),
			slog.Int("owned_cards", progress.OwnedCards),
			slog.Int("total_cards", progress.TotalCards))

		// Get random sample card from this collection
		sampleCardModel, err := collectionService.GetRandomSampleCard(ctx, collection.ID)
		if err != nil {
			slog.Warn("Failed to get sample card, continuing without it",
				slog.String("type", "cmd"),
				slog.String("name", "collection-info"),
				slog.String("user_id", userID),
				slog.String("collection_id", collection.ID),
				slog.String("error", err.Error()))
		}

		var sampleCard *string
		if sampleCardModel != nil {
			sampleCard = &sampleCardModel.Name
		}

		var fields []discord.EmbedField

		progressText := fmt.Sprintf("%.1f%% (%d/%d cards)", 
			progress.Percentage, progress.OwnedCards, progress.TotalCards)
		if progress.IsCompleted {
			progressText = "✅ **100%** Complete!"
		}
		fields = append(fields, discord.EmbedField{
			Name:   "Your Progress",
			Value:  progressText,
			Inline: &[]bool{true}[0],
		})

		collectionType := "Regular Collection"
		if progress.IsFragment {
			collectionType = "🧩 Fragment Collection (only 1-star cards count)"
		}
		fields = append(fields, discord.EmbedField{
			Name:   "Collection Type",
			Value:  collectionType,
			Inline: &[]bool{true}[0],
		})

		fields = append(fields, discord.EmbedField{
			Name:   "Total Cards",
			Value:  fmt.Sprintf("%d cards", progress.TotalCards),
			Inline: &[]bool{true}[0],
		})

		if len(collection.Aliases) > 0 {
			aliasText := strings.Join(collection.Aliases, ", ")
			if len(aliasText) > 100 {
				aliasText = aliasText[:97] + "..."
			}
			fields = append(fields, discord.EmbedField{
				Name:   "Aliases",
				Value:  fmt.Sprintf("`%s`", aliasText),
				Inline: &[]bool{false}[0],
			})
		}


		if sampleCard != nil {
			fields = append(fields, discord.EmbedField{
				Name:   "Sample Card",
				Value:  utils.FormatCardName(*sampleCard),
				Inline: &[]bool{false}[0],
			})
		}

		var cloutInfo string
		for _, cloutCol := range user.CloutedCols {
			if cloutCol.ID == collection.ID {
				cloutInfo = fmt.Sprintf("⭐ %d clout stars", cloutCol.Amount)
				break
			}
		}
		if cloutInfo == "" {
			cloutInfo = "No clout stars yet"
		}
		fields = append(fields, discord.EmbedField{
			Name:   "Your Clout",
			Value:  cloutInfo,
			Inline: &[]bool{true}[0],
		})

		var description string
		if progress.IsCompleted {
			description = "🎉 **Congratulations!** You have completed this collection!\n\n"
			if progress.IsFragment {
				description += "As a fragment collection, you can reset it to earn a 4-star card and clout stars."
			} else {
				description += "You can reset this collection to earn clout stars and legendary tickets."
			}
		} else {
			missing := progress.TotalCards - progress.OwnedCards
			description = fmt.Sprintf("You're missing **%d cards** to complete this collection.", missing)
		}

		embed := discord.Embed{
			Title:       collection.Name,
			Description: description,
			Color:       config.BackgroundColor,
			Fields:      fields,
		}

		// Add sample card image if available
		if sampleCardModel != nil {
			// Generate image URL for the sample card using existing patterns
			spacesConfig := b.SpacesService.GetSpacesConfig()
			cardInfo := utils.GetCardDisplayInfo(
				sampleCardModel.Name,
				sampleCardModel.ColID,
				sampleCardModel.Level,
				utils.GetGroupType(sampleCardModel.Tags),
				spacesConfig,
			)
			embed.Image = &discord.EmbedResource{
				URL: cardInfo.ImageURL,
			}
		}

		return event.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}})
	}
}