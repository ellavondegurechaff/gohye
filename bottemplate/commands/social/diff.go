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
    // Use the unified PaginationFactory for both initial and component pagination
    cardOperationsService := services.NewCardOperationsService(b.CardRepository, b.UserCardRepository)

    return func(e *handler.CommandEvent) error {
        // Defer to avoid 3s timeout (prevents 10062 Unknown interaction)
        if err := e.DeferCreateMessage(false); err != nil {
            return err
        }

        ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
        defer cancel()

        data := e.SlashCommandInteractionData()
        subCmd := *data.SubCommandName
        targetUser := data.User("user")
        query := strings.TrimSpace(data.String("query"))

        // Create factory pieces shared with component handler
        fetcher := &DiffDataFetcher{bot: b, cardOperationsService: cardOperationsService}
        formatter := &DiffFormatter{bot: b}
        validator := &DiffValidator{}

        factoryConfig := utils.PaginationFactoryConfig{
            ItemsPerPage: config.CardsPerPage,
            Prefix:       "diff",
            Parser:       utils.NewDiffParser(),
            Fetcher:      fetcher,
            Formatter:    formatter,
            Validator:    validator,
        }
        factory := utils.NewPaginationFactory(factoryConfig)

        params := utils.PaginationParams{
            UserID:       e.User().ID.String(),
            Page:         0,
            SubCommand:   subCmd,
            TargetUserID: targetUser.ID.String(),
            Query:        query,
        }

        // Build initial embed/components via factory
        embed, components, err := factory.CreateInitialPaginationEmbed(ctx, params)
        if err != nil {
            // Provide meaningful error via the deferred response
            msg := "Failed to create pagination"
            if strings.Contains(strings.ToLower(err.Error()), "no items") {
                msg = "No difference found in card collections!"
            }
            return utils.EH.UpdateInteractionResponse(e, "Diff", msg)
        }

        // Update deferred initial response
        _, updErr := e.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{embed}, Components: &components})
        return updErr
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

    // Compute totalItems from totalPages and current page size
    itemsPerPage := config.CardsPerPage
    totalItems := totalPages * itemsPerPage
    if page == totalPages-1 { // last page may be partial
        totalItems = (totalPages-1)*itemsPerPage + len(displayItems)
    }

    return cardDisplayService.CreateCardsEmbed(
        context.Background(),
        title,
        displayItems,
        page,
        totalPages,
        totalItems,
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
