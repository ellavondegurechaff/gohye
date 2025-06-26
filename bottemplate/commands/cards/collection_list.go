package cards

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var CollectionList = discord.SlashCommandCreate{
	Name:        "collection-list",
	Description: "View collections with your completion progress",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "search",
			Description: "Search for specific collections",
			Required:    false,
		},
		discord.ApplicationCommandOptionBool{
			Name:        "completed",
			Description: "Show only completed collections",
			Required:    false,
		},
		discord.ApplicationCommandOptionBool{
			Name:        "sort-by-progress",
			Description: "Sort by completion progress",
			Required:    false,
		},
	},
}

type CollectionProgressItem struct {
	Collection *models.Collection
	Progress   *models.CollectionProgress
}

func CollectionListHandler(b *bottemplate.Bot) handler.CommandHandler {

	paginationHandler := &utils.PaginationHandler{
		Config: utils.PaginationConfig{
			ItemsPerPage: 10,
			Prefix:       "collection-list",
		},
		FormatItems: func(items []interface{}, page, totalPages int, userID, query string) (discord.Embed, error) {
			var fields []discord.EmbedField
			
			for _, item := range items {
				collectionItem := item.(CollectionProgressItem)
				col := collectionItem.Collection
				progress := collectionItem.Progress
				
				var progressText string
				if progress.IsCompleted {
					progressText = "✅ **100%** Complete"
				} else {
					progressText = fmt.Sprintf("📊 **%.1f%%** (%d/%d cards)", 
						progress.Percentage, progress.OwnedCards, progress.TotalCards)
				}
				
				var fragmentText string
				if progress.IsFragment {
					fragmentText = " 🧩"
				}
				
				aliasText := ""
				if len(col.Aliases) > 0 {
					aliasText = fmt.Sprintf("\n`%s`", strings.Join(col.Aliases[:min(3, len(col.Aliases))], ", "))
				}
				
				fields = append(fields, discord.EmbedField{
					Name:   fmt.Sprintf("%s%s", col.Name, fragmentText),
					Value:  fmt.Sprintf("%s%s", progressText, aliasText),
					Inline: &[]bool{true}[0],
				})
			}

			title := fmt.Sprintf("Collections - Page %d/%d", page+1, totalPages)
			if query != "" {
				title = fmt.Sprintf("Collections matching '%s' - Page %d/%d", query, page+1, totalPages)
			}

			return discord.Embed{
				Title:       title,
				Color:       config.BackgroundColor,
				Fields:      fields,
				Description: "🧩 = Fragment Collection (only 1-star cards count)",
			}, nil
		},
	}

	return func(event *handler.CommandEvent) error {
		start := time.Now()
		userID := event.User().ID.String()
		
		slog.Info("Collection-list command started",
			slog.String("type", "cmd"),
			slog.String("name", "collection-list"),
			slog.String("user_id", userID))
		
		defer func() {
			slog.Info("Collection-list command completed",
				slog.String("type", "cmd"),
				slog.String("name", "collection-list"),
				slog.String("user_id", userID),
				slog.Duration("total_time", time.Since(start)))
		}()

		ctx := context.Background()
		search := strings.TrimSpace(event.SlashCommandInteractionData().String("search"))

		// Simple collection listing - no user data needed for basic list
		var collections []*models.Collection
		var err error
		
		if search != "" {
			collections, err = b.CollectionRepository.SearchCollections(ctx, search)
		} else {
			collections, err = b.CollectionRepository.GetAll(ctx)
		}
		
		if err != nil {
			slog.Error("Failed to get collections",
				slog.String("type", "cmd"),
				slog.String("name", "collection-list"),
				slog.String("user_id", userID),
				slog.String("error", err.Error()))
			return utils.EH.CreateErrorEmbed(event, "Failed to get collections")
		}

		if len(collections) == 0 {
			embed := discord.Embed{
				Title:       "No Collections Found",
				Description: "No collections match your search criteria.",
				Color:       config.ErrorColor,
			}
			return event.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}})
		}

		// Create simple list items without progress calculation
		var items []interface{}
		for _, col := range collections {
			items = append(items, CollectionProgressItem{
				Collection: col,
				Progress: &models.CollectionProgress{
					UserID:       userID,
					CollectionID: col.ID,
					TotalCards:   0, // Will be populated if needed
					OwnedCards:   0,
					Percentage:   0,
					IsCompleted:  false,
					IsFragment:   col.Fragments,
					LastUpdated:  time.Now(),
				},
			})
		}

		embed, components, err := paginationHandler.CreateInitialPaginationEmbed(items, userID, search)
		if err != nil {
			return utils.EH.CreateErrorEmbed(event, "Failed to create collection display")
		}

		return event.CreateMessage(discord.MessageCreate{
			Embeds:     []discord.Embed{embed},
			Components: components,
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// CollectionListComponentHandler handles pagination for collection list
func CollectionListComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	collectionService := services.NewCollectionService(b.CollectionRepository, b.CardRepository, b.UserCardRepository)
	
	// Create data fetcher
	fetcher := &CollectionListDataFetcher{
		bot:               b,
		collectionService: collectionService,
	}
	
	// Create formatter
	formatter := &CollectionListFormatter{}
	
	// Create validator
	validator := &CollectionListValidator{}
	
	// Create factory configuration
	factoryConfig := utils.PaginationFactoryConfig{
		ItemsPerPage: 10,
		Prefix:       "collection-list",
		Parser:       utils.NewRegularParser("collection-list"),
		Fetcher:      fetcher,
		Formatter:    formatter,
		Validator:    validator,
	}
	
	// Create factory and return handler
	factory := utils.NewPaginationFactory(factoryConfig)
	return factory.CreateHandler()
}

// CollectionListDataFetcher implements DataFetcher for collection list
type CollectionListDataFetcher struct {
	bot               *bottemplate.Bot
	collectionService *services.CollectionService
}

func (f *CollectionListDataFetcher) FetchData(ctx context.Context, params utils.PaginationParams) ([]interface{}, error) {
	slog.Debug("Collection-list fetching data for pagination",
		slog.String("type", "cmd"),
		slog.String("name", "collection-list"),
		slog.String("user_id", params.UserID),
		slog.String("query", params.Query))

	user, err := f.bot.UserRepository.GetByDiscordID(ctx, params.UserID)
	if err != nil {
		slog.Error("Failed to get user data in pagination fetcher",
			slog.String("type", "cmd"),
			slog.String("name", "collection-list"),
			slog.String("user_id", params.UserID),
			slog.String("error", err.Error()))
		return nil, err
	}
	
	slog.Debug("Successfully retrieved user data in pagination fetcher",
		slog.String("type", "cmd"),
		slog.String("name", "collection-list"),
		slog.String("user_id", params.UserID),
		slog.String("username", user.Username))

	var collections []*models.Collection
	if params.Query != "" {
		collections, err = f.bot.CollectionRepository.SearchCollections(ctx, params.Query)
	} else {
		collections, err = f.bot.CollectionRepository.GetAll(ctx)
	}
	if err != nil {
		return nil, err
	}

	var items []interface{}
	for _, col := range collections {
		progress, err := f.collectionService.CalculateProgress(ctx, user.DiscordID, col.ID)
		if err != nil {
			continue
		}
		
		items = append(items, CollectionProgressItem{
			Collection: col,
			Progress:   progress,
		})
	}

	return items, nil
}

// CollectionListFormatter implements ItemFormatter for collection list
type CollectionListFormatter struct{}

func (f *CollectionListFormatter) FormatItems(items []interface{}, page, totalPages int, params utils.PaginationParams) (discord.Embed, error) {
	var fields []discord.EmbedField
	
	for _, item := range items {
		collectionItem := item.(CollectionProgressItem)
		col := collectionItem.Collection
		
		var fragmentText string
		if col.Fragments {
			fragmentText = " 🧩"
		}
		
		aliasText := ""
		if len(col.Aliases) > 0 {
			aliasText = fmt.Sprintf("\n`%s`", strings.Join(col.Aliases[:min(3, len(col.Aliases))], ", "))
		}
		
		collectionInfo := fmt.Sprintf("ID: `%s`%s", col.ID, aliasText)
		
		fields = append(fields, discord.EmbedField{
			Name:   fmt.Sprintf("%s%s", col.Name, fragmentText),
			Value:  collectionInfo,
			Inline: &[]bool{true}[0],
		})
	}

	title := fmt.Sprintf("Collections - Page %d/%d", page+1, totalPages)
	if params.Query != "" {
		title = fmt.Sprintf("Collections matching '%s' - Page %d/%d", params.Query, page+1, totalPages)
	}

	return discord.Embed{
		Title:       title,
		Color:       config.BackgroundColor,
		Fields:      fields,
		Description: "🧩 = Fragment Collection (only 1-star cards count)\nUse `/collection-info` to see your progress.",
	}, nil
}

func (f *CollectionListFormatter) FormatCopy(items []interface{}, params utils.PaginationParams) string {
	var result []string
	for _, item := range items {
		collectionItem := item.(CollectionProgressItem)
		col := collectionItem.Collection
		progress := collectionItem.Progress
		result = append(result, fmt.Sprintf("%s: %.1f%% (%d/%d)", col.Name, progress.Percentage, progress.OwnedCards, progress.TotalCards))
	}
	return strings.Join(result, "\n")
}

// CollectionListValidator implements UserValidator for collection list
type CollectionListValidator struct{}

func (v *CollectionListValidator) ValidateUser(eventUserID string, params utils.PaginationParams) bool {
	return eventUserID == params.UserID
}