package social

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"
)

var Diff = discord.SlashCommandCreate{
	Name:        "diff",
	Description: "Compare card collections between users",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommand{
			Name:        "for",
			Description: "View cards you have that another user doesn't",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionUser{
					Name:        "user",
					Description: "User to compare with",
					Required:    true,
				},
				discord.ApplicationCommandOptionString{
					Name:        "query",
					Description: "Filter cards by name, collection, or other attributes",
					Required:    false,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "from",
			Description: "View cards another user has that you don't",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionUser{
					Name:        "user",
					Description: "User to compare with",
					Required:    true,
				},
				discord.ApplicationCommandOptionString{
					Name:        "query",
					Description: "Filter cards by name, collection, or other attributes",
					Required:    false,
				},
			},
		},
	},
}

func DiffHandler(b *bottemplate.Bot) handler.CommandHandler {
	cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)
	
	return func(e *handler.CommandEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
		defer cancel()

		data := e.SlashCommandInteractionData()
		subCmd := *data.SubCommandName

		targetUser := data.User("user")
		query := strings.TrimSpace(data.String("query"))

		var diffCards []*models.Card
		var title string
		var err error

		switch subCmd {
		case "for":
			diffCards, err = cardOperationsService.GetCardDifferences(ctx, e.User().ID.String(), targetUser.ID.String(), "for")
			title = fmt.Sprintf("Cards you have that %s doesn't", targetUser.Username)
		case "from":
			diffCards, err = cardOperationsService.GetCardDifferences(ctx, e.User().ID.String(), targetUser.ID.String(), "from")
			title = fmt.Sprintf("Cards %s has that you don't", targetUser.Username)
		default:
			return utils.EH.CreateErrorEmbed(e, "Invalid subcommand")
		}

		if err != nil {
			return utils.EH.CreateErrorEmbed(e, err.Error())
		}

		if len(diffCards) == 0 {
			return utils.EH.CreateErrorEmbed(e, "No difference found in card collections!")
		}

		// Apply search filter if provided
		if query != "" {
			filters := utils.ParseSearchQuery(query)
			diffCards = cardOperationsService.SearchCardsInCollection(ctx, diffCards, filters)
			if len(diffCards) == 0 {
				return utils.EH.CreateErrorEmbed(e, fmt.Sprintf("No cards match the query: %s", query))
			}
		} else {
			// Default sorting by level and name when no query is provided
			sort.Slice(diffCards, func(i, j int) bool {
				if diffCards[i].Level != diffCards[j].Level {
					return diffCards[i].Level > diffCards[j].Level
				}
				return strings.ToLower(diffCards[i].Name) < strings.ToLower(diffCards[j].Name)
			})
		}

		// Use service-based pagination
		cardDisplayService := services.NewCardDisplayService(b.CardRepository, b.SpacesService)
		displayItems := cardDisplayService.ConvertCardsToDiffDisplayItemsSimple(diffCards)

		paginationHandler := utils.NewDiffPaginationHandler()
		paginationHandler.FormatItems = func(items []interface{}, page, totalPages int, data *utils.DiffPaginationData) (discord.Embed, error) {
			startIdx := page * utils.CardsPerPage
			endIdx := min(startIdx+utils.CardsPerPage, len(items))
			pageItems := make([]services.CardDisplayItem, endIdx-startIdx)
			for i, item := range items[startIdx:endIdx] {
				pageItems[i] = item.(services.CardDisplayItem)
			}

			return cardDisplayService.CreateCardsEmbed(
				ctx,
				data.Title,
				pageItems,
				page,
				totalPages,
				len(items),
				data.Query,
				config.BackgroundColor,
			)
		}

		paginationHandler.FormatCopy = func(items []interface{}, title string) string {
			pageItems := make([]services.CardDisplayItem, len(items))
			for i, item := range items {
				pageItems[i] = item.(services.CardDisplayItem)
			}
			copyText, _ := cardDisplayService.FormatCopyText(ctx, pageItems, title)
			return copyText
		}

		items := make([]interface{}, len(displayItems))
		for i, item := range displayItems {
			items[i] = item
		}

		paginationData := &utils.DiffPaginationData{
			Items:        items,
			TotalItems:   len(items),
			UserID:       e.User().ID.String(),
			SubCommand:   subCmd,
			TargetUserID: targetUser.ID.String(),
			Query:        query,
			Title:        title,
		}

		embed, components, err := paginationHandler.CreateInitialDiffPaginationEmbed(paginationData)
		if err != nil {
			return utils.EH.CreateErrorEmbed(e, "Failed to create pagination")
		}

		return e.CreateMessage(discord.MessageCreate{
			Embeds:     []discord.Embed{embed},
			Components: components,
		})
	}
}


// DiffComponentHandler handles diff command pagination using the new unified factory
func DiffComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)
	
	// Create data fetcher
	fetcher := &DiffDataFetcher{
		bot:                   b,
		cardOperationsService: cardOperationsService,
	}
	
	// Create formatter
	formatter := &DiffFormatter{
		bot: b,
	}
	
	// Create validator
	validator := &DiffValidator{}
	
	// Create factory configuration
	factoryConfig := utils.PaginationFactoryConfig{
		ItemsPerPage: config.CardsPerPage,
		Prefix:       "diff",
		Parser:       utils.NewDiffParser(),
		Fetcher:      fetcher,
		Formatter:    formatter,
		Validator:    validator,
	}
	
	// Create factory and return handler
	factory := utils.NewPaginationFactory(factoryConfig)
	return factory.CreateHandler()
}

// DiffDataFetcher implements DataFetcher for diff pagination
type DiffDataFetcher struct {
	bot                   *bottemplate.Bot
	cardOperationsService *services.CardOperationsService
}

func (ddf *DiffDataFetcher) FetchData(ctx context.Context, params utils.PaginationParams) ([]interface{}, error) {
	var diffCards []*models.Card
	var err error

	// Get diff cards based on subcommand using CardOperationsService
	if params.SubCommand == "for" {
		diffCards, err = ddf.cardOperationsService.GetCardDifferences(ctx, params.UserID, params.TargetUserID, "for")
	} else {
		diffCards, err = ddf.cardOperationsService.GetCardDifferences(ctx, params.UserID, params.TargetUserID, "from")
	}

	if err != nil {
		return nil, err
	}

	// Apply search filters if query exists
	if params.Query != "" {
		filters := utils.ParseSearchQuery(params.Query)
		diffCards = ddf.cardOperationsService.SearchCardsInCollection(ctx, diffCards, filters)
	} else {
		sort.Slice(diffCards, func(i, j int) bool {
			if diffCards[i].Level != diffCards[j].Level {
				return diffCards[i].Level > diffCards[j].Level
			}
			return strings.ToLower(diffCards[i].Name) < strings.ToLower(diffCards[j].Name)
		})
	}

	// Convert to display items using service
	cardDisplayService := services.NewCardDisplayService(ddf.bot.CardRepository, ddf.bot.SpacesService)
	displayItems := cardDisplayService.ConvertCardsToDiffDisplayItemsSimple(diffCards)

	items := make([]interface{}, len(displayItems))
	for i, item := range displayItems {
		items[i] = item
	}

	return items, nil
}

// DiffFormatter implements ItemFormatter for diff pagination
type DiffFormatter struct {
	bot *bottemplate.Bot
}

func (df *DiffFormatter) FormatItems(items []interface{}, page, totalPages int, params utils.PaginationParams) (discord.Embed, error) {
	// Parse the target user ID to get the title
	targetSnowflake, err := snowflake.Parse(params.TargetUserID)
	if err != nil {
		return discord.Embed{}, fmt.Errorf("invalid user ID")
	}

	targetUser, err := df.bot.Client.Rest().GetUser(targetSnowflake)
	if err != nil {
		return discord.Embed{}, fmt.Errorf("failed to fetch target user")
	}

	var title string
	if params.SubCommand == "for" {
		title = fmt.Sprintf("Cards you have that %s doesn't", targetUser.Username)
	} else {
		title = fmt.Sprintf("Cards %s has that you don't", targetUser.Username)
	}

	displayItems := make([]services.CardDisplayItem, len(items))
	for i, item := range items {
		displayItems[i] = item.(services.CardDisplayItem)
	}

	cardDisplayService := services.NewCardDisplayService(df.bot.CardRepository, df.bot.SpacesService)
	return cardDisplayService.CreatePaginatedCardsEmbed(
		context.Background(),
		title,
		displayItems,
		page,
		params.Query,
		config.BackgroundColor,
	)
}

func (df *DiffFormatter) FormatCopy(items []interface{}, params utils.PaginationParams) string {
	// Parse the target user ID to get the title
	targetSnowflake, err := snowflake.Parse(params.TargetUserID)
	if err != nil {
		return "Error: invalid user ID"
	}

	targetUser, err := df.bot.Client.Rest().GetUser(targetSnowflake)
	if err != nil {
		return "Error: failed to fetch target user"
	}

	var title string
	if params.SubCommand == "for" {
		title = fmt.Sprintf("Cards you have that %s doesn't", targetUser.Username)
	} else {
		title = fmt.Sprintf("Cards %s has that you don't", targetUser.Username)
	}

	displayItems := make([]services.CardDisplayItem, len(items))
	for i, item := range items {
		displayItems[i] = item.(services.CardDisplayItem)
	}
	
	cardDisplayService := services.NewCardDisplayService(df.bot.CardRepository, df.bot.SpacesService)
	copyText, err := cardDisplayService.FormatCopyText(context.Background(), displayItems, title)
	if err != nil {
		return "Error formatting copy text"
	}
	return copyText
}

// DiffValidator implements UserValidator for diff pagination
type DiffValidator struct{}

func (dv *DiffValidator) ValidateUser(eventUserID string, params utils.PaginationParams) bool {
	return eventUserID == params.UserID
}

