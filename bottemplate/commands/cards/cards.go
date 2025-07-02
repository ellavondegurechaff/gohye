package cards

import (
	"context"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Cards = discord.SlashCommandCreate{
	Name:        "cards",
	Description: "View your card collection",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "query",
			Description: "Search query (e.g., '5 gif winter -aespa >level')",
			Required:    false,
		},
	},
}

func CardsHandler(b *bottemplate.Bot) handler.CommandHandler {
	cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)
	cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)
	
	paginationHandler := &utils.PaginationHandler{
		Config: utils.PaginationConfig{
			ItemsPerPage: config.CardsPerPage,
			Prefix:       "cards",
		},
		FormatItems: func(items []interface{}, page, totalPages int, userID, query string) (discord.Embed, error) {
			displayItems := make([]services.CardDisplayItem, len(items))
			for i, item := range items {
				displayItems[i] = item.(services.CardDisplayItem)
			}

			// Calculate total items from pagination data
			itemsPerPage := config.CardsPerPage
			totalItems := totalPages * itemsPerPage
			if page == totalPages-1 {
				// Last page might have fewer items
				totalItems = (totalPages-1)*itemsPerPage + len(items)
			}

			return cardDisplayService.CreateCardsEmbed(
				context.Background(),
				"My Collection",
				displayItems,
				page,
				totalPages,
				totalItems,
				query,
				config.BackgroundColor,
			)
		},
		FormatCopy: func(items []interface{}) string {
			displayItems := make([]services.CardDisplayItem, len(items))
			for i, item := range items {
				displayItems[i] = item.(services.CardDisplayItem)
			}
			
			copyText, err := cardDisplayService.FormatCopyText(context.Background(), displayItems, "My Collection")
			if err != nil {
				return "Error formatting copy text"
			}
			return copyText
		},
		ValidateUser: func(eventUserID, targetUserID string) bool {
			return eventUserID == targetUserID
		},
	}

	return func(event *handler.CommandEvent) error {
		query := strings.TrimSpace(event.SlashCommandInteractionData().String("query"))
		
		// Get user data for new card detection
		user, err := b.UserRepository.GetByDiscordID(context.Background(), event.User().ID.String())
		if err != nil {
			return utils.EH.CreateErrorEmbed(event, "Failed to fetch user data")
		}
		
		// Use CardOperationsService to get user cards with details, filtering, and search context
		displayCards, _, filters, err := cardOperationsService.GetUserCardsWithDetailsAndFiltersWithUser(context.Background(), event.User().ID.String(), query, user)
		if err != nil {
			return utils.EH.CreateErrorEmbed(event, "Failed to fetch cards")
		}

		if len(displayCards) == 0 {
			return utils.EH.CreateErrorEmbed(event, "No cards found")
		}

		// Convert to CardDisplayItem slice with user data for new card detection and sorting context
		displayItems, err := cardDisplayService.ConvertUserCardsToDisplayItemsWithUserAndContext(context.Background(), displayCards, user, filters)
		if err != nil {
			return utils.EH.CreateErrorEmbed(event, "Failed to prepare card display")
		}

		// Convert to interface{} slice for pagination handler
		items := make([]interface{}, len(displayItems))
		for i, item := range displayItems {
			items[i] = item
		}

		embed, components, err := paginationHandler.CreateInitialPaginationEmbed(items, event.User().ID.String(), query)
		if err != nil {
			return utils.EH.CreateErrorEmbed(event, "Failed to create card display")
		}

		return event.CreateMessage(discord.MessageCreate{
			Embeds:     []discord.Embed{embed},
			Components: components,
		})
	}
}


// CardsComponentHandler handles pagination for cards using the new unified factory
func CardsComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)
	cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)
	
	// Create data fetcher
	fetcher := &CardsDataFetcher{
		bot:                   b,
		cardDisplayService:    cardDisplayService,
		cardOperationsService: cardOperationsService,
	}
	
	// Create formatter
	formatter := &CardsFormatter{
		cardDisplayService: cardDisplayService,
	}
	
	// Create validator
	validator := &CardsValidator{}
	
	// Create factory configuration
	factoryConfig := utils.PaginationFactoryConfig{
		ItemsPerPage: config.CardsPerPage,
		Prefix:       "cards",
		Parser:       utils.NewRegularParser("cards"),
		Fetcher:      fetcher,
		Formatter:    formatter,
		Validator:    validator,
	}
	
	// Create factory and return handler
	factory := utils.NewPaginationFactory(factoryConfig)
	return factory.CreateHandler()
}

// CardsDataFetcher implements DataFetcher for cards pagination
type CardsDataFetcher struct {
	bot                   *bottemplate.Bot
	cardDisplayService    *services.CardDisplayService
	cardOperationsService *services.CardOperationsService
}

func (cdf *CardsDataFetcher) FetchData(ctx context.Context, params utils.PaginationParams) ([]interface{}, error) {
	// Get user data for new card detection
	user, err := cdf.bot.UserRepository.GetByDiscordID(ctx, params.UserID)
	if err != nil {
		return nil, err
	}
	
	// Use CardOperationsService to get user cards with details and filtering
	displayCards, _, _, err := cdf.cardOperationsService.GetUserCardsWithDetailsAndFiltersWithUser(ctx, params.UserID, params.Query, user)
	if err != nil {
		return nil, err
	}

	// Convert to CardDisplayItem slice with user data
	displayItems, err := cdf.cardDisplayService.ConvertUserCardsToDisplayItemsWithUser(ctx, displayCards, user)
	if err != nil {
		return nil, err
	}

	// Convert to interface{} slice
	items := make([]interface{}, len(displayItems))
	for i, item := range displayItems {
		items[i] = item
	}

	return items, nil
}

// CardsFormatter implements ItemFormatter for cards pagination
type CardsFormatter struct {
	cardDisplayService *services.CardDisplayService
}

func (cf *CardsFormatter) FormatItems(items []interface{}, page, totalPages int, params utils.PaginationParams) (discord.Embed, error) {
	displayItems := make([]services.CardDisplayItem, len(items))
	for i, item := range items {
		displayItems[i] = item.(services.CardDisplayItem)
	}

	// Calculate total items from pagination data
	itemsPerPage := config.CardsPerPage
	totalItems := totalPages * itemsPerPage
	if page == totalPages-1 {
		// Last page might have fewer items
		totalItems = (totalPages-1)*itemsPerPage + len(items)
	}

	return cf.cardDisplayService.CreateCardsEmbed(
		context.Background(),
		"My Collection",
		displayItems,
		page,
		totalPages,
		totalItems,
		params.Query,
		config.BackgroundColor,
	)
}

func (cf *CardsFormatter) FormatCopy(items []interface{}, params utils.PaginationParams) string {
	displayItems := make([]services.CardDisplayItem, len(items))
	for i, item := range items {
		displayItems[i] = item.(services.CardDisplayItem)
	}
	
	copyText, err := cf.cardDisplayService.FormatCopyText(context.Background(), displayItems, "My Collection")
	if err != nil {
		return "Error formatting copy text"
	}
	return copyText
}

// CardsValidator implements UserValidator for cards pagination
type CardsValidator struct{}

func (cv *CardsValidator) ValidateUser(eventUserID string, params utils.PaginationParams) bool {
	return eventUserID == params.UserID
}


