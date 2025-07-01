package social

import (
	"context"
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Miss = discord.SlashCommandCreate{
	Name:        "miss",
	Description: "View missing cards from your collection",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "card_query",
			Description: "Search for specific missing cards (optional)",
			Required:    false,
		},
	},
}

func MissHandler(b *bottemplate.Bot) handler.CommandHandler {
	// Initialize services
	cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)
	cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)
	
	// Create pagination handler with minimal items per page to prevent Discord form body errors
	paginationHandler := &utils.PaginationHandler{
		Config: utils.PaginationConfig{
			ItemsPerPage: 3, // Further reduced to prevent form body errors
			Prefix:       "miss",
		},
		FormatItems: func(items []interface{}, page, totalPages int, userID, query string) (discord.Embed, error) {
			// Convert back to CardDisplayItem slice
			displayItems := make([]services.CardDisplayItem, len(items))
			for i, item := range items {
				displayItems[i] = item.(services.CardDisplayItem)
			}
			
			description, err := cardDisplayService.FormatCardDisplayItems(context.Background(), displayItems)
			if err != nil {
				return discord.Embed{}, fmt.Errorf("failed to format card display: %w", err)
			}
			
			// Calculate total items from pagination data
			itemsPerPage := 3 // Match the configured ItemsPerPage
			totalItems := totalPages * itemsPerPage
			if page == totalPages-1 {
				// Last page might have fewer items
				totalItems = (totalPages-1)*itemsPerPage + len(items)
			}
			
			embed := discord.NewEmbedBuilder().
				SetTitle("Missing Cards").
				SetDescription(description).
				SetColor(config.BackgroundColor).
				SetFooter(fmt.Sprintf("Page %d/%d • Total Missing: %d", page+1, totalPages, totalItems), "")
			
			// Add search query to description if provided
			if query != "" {
				embed.SetDescription(fmt.Sprintf("🔍`%s`\n\n%s", query, description))
			}
			
			return embed.Build(), nil
		},
		FormatCopy: func(items []interface{}) string {
			// Convert back to CardDisplayItem slice
			displayItems := make([]services.CardDisplayItem, len(items))
			for i, item := range items {
				displayItems[i] = item.(services.CardDisplayItem)
			}
			
			copyText, err := cardDisplayService.FormatCopyText(context.Background(), displayItems, "Missing Cards")
			if err != nil {
				return "Error formatting copy text"
			}
			return copyText
		},
		ValidateUser: func(eventUserID, targetUserID string) bool {
			return eventUserID == targetUserID
		},
	}

	return func(e *handler.CommandEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
		defer cancel()

		query := strings.TrimSpace(e.SlashCommandInteractionData().String("card_query"))
		
		// Use CardOperationsService to get missing cards
		missingCards, err := cardOperationsService.GetMissingCards(ctx, e.User().ID.String(), query)
		if err != nil {
			return utils.EH.CreateErrorEmbed(e, "Failed to fetch missing cards")
		}

		if len(missingCards) == 0 {
			if query != "" {
				return utils.EH.CreateErrorEmbed(e, "No missing cards found matching your search criteria.")
			}
			return utils.EH.CreateErrorEmbed(e, "You own all available cards! 🎉")
		}

		// Convert to CardDisplayItem slice for pagination handler
		displayItems := cardDisplayService.ConvertCardsToMissingDisplayItems(missingCards)

		// Convert to interface{} slice for pagination handler
		items := make([]interface{}, len(displayItems))
		for i, item := range displayItems {
			items[i] = item
		}

		embed, components, err := paginationHandler.CreateInitialPaginationEmbed(items, e.User().ID.String(), query)
		if err != nil {
			return utils.EH.CreateErrorEmbed(e, "Failed to create pagination")
		}

		return e.CreateMessage(discord.MessageCreate{
			Embeds:     []discord.Embed{embed},
			Components: components,
		})
	}
}

// MissComponentHandler handles pagination for missing cards using the new unified factory
func MissComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)
	
	// Create data fetcher
	fetcher := &MissDataFetcher{
		bot:                   b,
		cardOperationsService: cardOperationsService,
	}
	
	// Create formatter
	formatter := &MissFormatter{
		bot: b,
	}
	
	// Create validator
	validator := &MissValidator{}
	
	// Create factory configuration with minimal items per page to prevent Discord form body errors
	factoryConfig := utils.PaginationFactoryConfig{
		ItemsPerPage: 3, // Further reduced to prevent form body errors
		Prefix:       "miss",
		Parser:       utils.NewRegularParser("miss"),
		Fetcher:      fetcher,
		Formatter:    formatter,
		Validator:    validator,
	}
	
	// Create factory and return handler
	factory := utils.NewPaginationFactory(factoryConfig)
	return factory.CreateHandler()
}

// MissDataFetcher implements DataFetcher for miss pagination
type MissDataFetcher struct {
	bot                   *bottemplate.Bot
	cardOperationsService *services.CardOperationsService
}

func (mdf *MissDataFetcher) FetchData(ctx context.Context, params utils.PaginationParams) ([]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, config.DefaultQueryTimeout)
	defer cancel()

	// Use CardOperationsService to get missing cards
	missingCards, err := mdf.cardOperationsService.GetMissingCards(ctx, params.UserID, params.Query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch missing cards: %w", err)
	}

	if len(missingCards) == 0 {
		return nil, fmt.Errorf("you own all available cards")
	}

	// Convert to CardDisplayItem slice
	cardDisplayService := services.NewCardDisplayService(mdf.bot.CardRepository, mdf.bot.SpacesService)
	displayItems := cardDisplayService.ConvertCardsToMissingDisplayItems(missingCards)

	// Convert to interface{} slice
	items := make([]interface{}, len(displayItems))
	for i, item := range displayItems {
		items[i] = item
	}

	return items, nil
}

// MissFormatter implements ItemFormatter for miss pagination
type MissFormatter struct {
	bot *bottemplate.Bot
}

func (mf *MissFormatter) FormatItems(items []interface{}, page, totalPages int, params utils.PaginationParams) (discord.Embed, error) {
	displayItems := make([]services.CardDisplayItem, len(items))
	for i, item := range items {
		displayItems[i] = item.(services.CardDisplayItem)
	}
	
	cardDisplayService := services.NewCardDisplayService(mf.bot.CardRepository, mf.bot.SpacesService)
	description, err := cardDisplayService.FormatCardDisplayItems(context.Background(), displayItems)
	if err != nil {
		return discord.Embed{}, fmt.Errorf("failed to format card display: %w", err)
	}
	
	// Calculate total items from pagination data
	itemsPerPage := 3 // Match the configured ItemsPerPage
	totalItems := totalPages * itemsPerPage
	if page == totalPages-1 {
		// Last page might have fewer items
		totalItems = (totalPages-1)*itemsPerPage + len(items)
	}
	
	embed := discord.NewEmbedBuilder().
		SetTitle("Missing Cards").
		SetDescription(description).
		SetColor(config.BackgroundColor).
		SetFooter(fmt.Sprintf("Page %d/%d • Total Missing: %d", page+1, totalPages, totalItems), "")
	
	// Add search query to description if provided
	if params.Query != "" {
		embed.SetDescription(fmt.Sprintf("🔍`%s`\n\n%s", params.Query, description))
	}
	
	return embed.Build(), nil
}

func (mf *MissFormatter) FormatCopy(items []interface{}, params utils.PaginationParams) string {
	displayItems := make([]services.CardDisplayItem, len(items))
	for i, item := range items {
		displayItems[i] = item.(services.CardDisplayItem)
	}
	
	cardDisplayService := services.NewCardDisplayService(mf.bot.CardRepository, mf.bot.SpacesService)
	copyText, err := cardDisplayService.FormatCopyText(context.Background(), displayItems, "Missing Cards")
	if err != nil {
		return "Error formatting copy text"
	}
	return copyText
}

// MissValidator implements UserValidator for miss pagination
type MissValidator struct{}

func (mv *MissValidator) ValidateUser(eventUserID string, params utils.PaginationParams) bool {
	return eventUserID == params.UserID
}