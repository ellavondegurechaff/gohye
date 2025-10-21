package cards

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
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

	// Create factory
	factory := utils.NewPaginationFactory(factoryConfig)

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
		completedOnly := event.SlashCommandInteractionData().Bool("completed")
		sortByProgress := event.SlashCommandInteractionData().Bool("sort-by-progress")

		// Create pagination parameters
		params := utils.PaginationParams{
			UserID:         userID,
			Page:           0,
			Query:          search,
			SortByProgress: sortByProgress,
			CompletedOnly:  completedOnly,
		}

        // Defer immediately to avoid Discord's 3s timeout
        if err := event.DeferCreateMessage(false); err != nil {
            return fmt.Errorf("failed to defer: %w", err)
        }

        // Provide immediate placeholder to replace "thinking" state
        _, _ = event.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{{
            Title:       "Collections",
            Description: "Loading your collection progress...",
            Color:       config.BackgroundColor,
        }}})

        // Run expensive work asynchronously to avoid 10s wrapper timeout
        go func() {
            embed, components, err := factory.CreateInitialPaginationEmbed(ctx, params)
            if err != nil {
                if err.Error() == "no items found" {
                    _, _ = event.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{{
                        Title:       "No Collections Found",
                        Description: "No collections match your search criteria.",
                        Color:       config.ErrorColor,
                    }}})
                    return
                }
                slog.Error("Failed to create collection display",
                    slog.String("type", "cmd"),
                    slog.String("name", "collection-list"),
                    slog.String("user_id", userID),
                    slog.String("error", err.Error()))
                _, _ = event.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{{
                    Title:       "Error",
                    Description: "Failed to create collection display",
                    Color:       config.ErrorColor,
                }}})
                return
            }

            _, _ = event.UpdateInteractionResponse(discord.MessageUpdate{
                Embeds:     &[]discord.Embed{embed},
                Components: &components,
            })
        }()
        return nil
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

	// Use batch calculation for efficiency
	progressMap, err := f.collectionService.CalculateProgressBatch(ctx, user.DiscordID, collections)
	if err != nil {
		return nil, err
	}

	var collectionItems []CollectionProgressItem
	for _, col := range collections {
		progress, exists := progressMap[col.ID]
		if !exists {
			// Fallback to zero progress if calculation failed for this collection
			progress = &models.CollectionProgress{
				UserID:       params.UserID,
				CollectionID: col.ID,
				TotalCards:   0,
				OwnedCards:   0,
				Percentage:   0,
				IsCompleted:  false,
				IsFragment:   col.Fragments,
				LastUpdated:  time.Now(),
			}
		}

		// Filter completed collections if requested
		if params.CompletedOnly && !progress.IsCompleted {
			continue
		}

		collectionItems = append(collectionItems, CollectionProgressItem{
			Collection: col,
			Progress:   progress,
		})
	}

	// Sort by progress if requested
	if params.SortByProgress {
		sort.Slice(collectionItems, func(i, j int) bool {
			// Sort by completion percentage descending
			return collectionItems[i].Progress.Percentage > collectionItems[j].Progress.Percentage
		})
	}

	// Convert to interface slice
	var items []interface{}
	for _, item := range collectionItems {
		items = append(items, item)
	}

	return items, nil
}

// CollectionListFormatter implements ItemFormatter for collection list
type CollectionListFormatter struct{}

func (f *CollectionListFormatter) FormatItems(allItems []interface{}, page, totalPages int, params utils.PaginationParams) (discord.Embed, error) {
    // Items provided by PaginationFactory are already page-scoped
    pageItems := allItems

    var fields []discord.EmbedField

	for _, item := range pageItems {
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

		inlineTrue := true
		fields = append(fields, discord.EmbedField{
			Name:   fmt.Sprintf("%s%s", col.Name, fragmentText),
			Value:  fmt.Sprintf("%s%s", progressText, aliasText),
			Inline: &inlineTrue,
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
