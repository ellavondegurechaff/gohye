package cards

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/disgoorg/bot-template/bottemplate/cardleveling"
)

func intPtr(v int) *int {
	return &v
}

var LevelUp = discord.SlashCommandCreate{
	Name:        "levelup",
	Description: "Level up or combine your cards",
	Options: []discord.ApplicationCommandOption{
		&discord.ApplicationCommandOptionString{
			Name:        "card_name",
			Description: "The name or ID of the card to level up",
			Required:    true,
		},
		&discord.ApplicationCommandOptionString{
			Name:        "combine_with",
			Description: "Optional: Name or ID of another card to combine with",
			Required:    false,
		},
	},
}

type LevelUpCommand struct {
	levelingService *cardleveling.Service
	cardRepo        repositories.CardRepository
	bot             *bottemplate.Bot
}

func NewLevelUpCommand(levelingService *cardleveling.Service, cardRepo repositories.CardRepository, bot *bottemplate.Bot) *LevelUpCommand {
	return &LevelUpCommand{
		levelingService: levelingService,
		cardRepo:        cardRepo,
		bot:             bot,
	}
}

func (c *LevelUpCommand) Handle(event *handler.CommandEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := event.DeferCreateMessage(false); err != nil {
		return fmt.Errorf("failed to defer response: %w", err)
	}

	cardQuery := event.SlashCommandInteractionData().String("card_name")

    // Search only within user's owned cards (fast join search)
    card, err := c.findCard(ctx, event.User().ID.String(), cardQuery)
	if err != nil {
		return createErrorEmbed(event, "Card Not Found", fmt.Sprintf("Could not find a card matching '%s' in your collection. Use /cards to see your cards.", cardQuery))
	}

	// Get the user's card data
	userCard, err := c.cardRepo.GetUserCard(ctx, event.User().ID.String(), card.ID)
	if err != nil {
		return createErrorEmbed(event, "Card Access Error", "Failed to access your card data.")
	}

	// Safety check to prevent nil pointer dereference
	if userCard == nil {
		return createErrorEmbed(event, "Card Data Error", "Card data is invalid.")
	}

	if combineWith := event.SlashCommandInteractionData().String("combine_with"); combineWith != "" {
		return c.handleCombine(event, userCard, combineWith)
	}

	result, err := c.levelingService.GainExp(ctx, userCard)
	if err != nil {
		if err.Error() == "exp gain on cooldown" {
			return createCooldownEmbed(event, userCard)
		}
		return createErrorEmbed(event, "Error", err.Error())
	}

	// Safety check to prevent nil pointer dereference
	if result == nil {
		return createErrorEmbed(event, "Leveling Error", "Failed to process leveling result.")
	}

	// Check for collection completion after successful level up
	go c.bot.CompletionChecker.CheckCompletionForCards(context.Background(), event.User().ID.String(), []int64{userCard.CardID})

	// Track quest progress

	// Track effect progress for Kiss Later
	if c.bot.EffectManager != nil {
		go c.bot.EffectManager.UpdateEffectProgress(context.Background(), event.User().ID.String(), "kisslater", 1)
	}

    // Use card details we already have from search (avoid extra DB call)
    cardDetails := card

	// Safety check for bot services
	if c.bot == nil || c.bot.SpacesService == nil {
		return createErrorEmbed(event, "Service Error", "Bot services are not available.")
	}

	cardInfo := utils.GetCardDisplayInfo(
		cardDetails.Name,
		cardDetails.ColID,
		userCard.Level,
		utils.GetGroupType(cardDetails.Tags),
		utils.SpacesConfig{
			Bucket:   c.bot.SpacesService.GetBucket(),
			Region:   c.bot.SpacesService.GetRegion(),
			CardRoot: c.bot.SpacesService.GetCardRoot(),
			GetImageURL: func(cardName string, colID string, level int, groupType string) string {
				return c.bot.SpacesService.GetCardImageURL(cardName, colID, level, groupType)
			},
		},
	)

	embed := discord.NewEmbedBuilder().
		SetTitle("Level Progress").
		SetColor(utils.GetDominantColor(cardInfo.ImageURL)).
		SetThumbnail(cardInfo.ImageURL)

	if result.CombinedCard {
		embed.SetDescription(fmt.Sprintf("✨ **CARDS COMBINED!** ✨\n\n"+
			"Your %s cards have merged into a powerful **Level %d** card!",
			cardInfo.FormattedName, result.NewLevel))
	} else {
		expBar := createExpBar(result.CurrentExp, result.RequiredExp)
		expPercentage := float64(result.CurrentExp) / float64(result.RequiredExp) * 100

		description := fmt.Sprintf("**%s**\n"+
			"``%s``\n\n"+
			"```ansi\n"+
			"\x1b[1;33mLevel %d %s\x1b[0m\n"+
			"\x1b[0;37mProgress Bar:\x1b[0m %s\n"+
			"\x1b[1;32m%.1f%%\x1b[0m Complete\n\n"+
			"\x1b[1;36mEXP Gained:\x1b[0m +%d\n"+
			"\x1b[1;36mCurrent EXP:\x1b[0m %d/%d\n"+
			"```",
			cardInfo.FormattedName,
			cardInfo.FormattedCollection,
			userCard.Level,
			utils.GetPromoRarityDisplay(cardDetails.ColID, userCard.Level),
			expBar,
			expPercentage,
			result.ExpGained,
			result.CurrentExp,
			result.RequiredExp)

		if len(result.Bonuses) > 0 {
			description += fmt.Sprintf("\n> %s", strings.Join(result.Bonuses, "\n> "))
		}

		embed.SetDescription(description)
	}

    // Send followup asynchronously to keep handler fast
    go func() { _, _ = event.CreateFollowupMessage(discord.MessageCreate{Embeds: []discord.Embed{embed.Build()}}) }()
    return nil
}

func (c *LevelUpCommand) handleCombine(event *handler.CommandEvent, mainCard *models.UserCard, fodderCardQuery string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Search only within user's owned cards for combine fodder
	fodderCard, err := c.findCard(ctx, event.User().ID.String(), fodderCardQuery)
	if err != nil {
		return createErrorEmbed(event, "Fodder Card Not Found", fmt.Sprintf("Could not find a card matching '%s' in your collection to combine with.", fodderCardQuery))
	}

	// Get the user's fodder card data
	userFodderCard, err := c.cardRepo.GetUserCard(ctx, event.User().ID.String(), fodderCard.ID)
	if err != nil {
		return createErrorEmbed(event, "Fodder Card Access Error", "Failed to access your fodder card data.")
	}

	// Safety checks to prevent nil pointer dereference
	if userFodderCard == nil {
		return createErrorEmbed(event, "Fodder Card Data Error", "Fodder card data is invalid.")
	}

	result, err := c.levelingService.CombineCards(ctx, mainCard, userFodderCard)
	if err != nil {
		return createErrorEmbed(event, "Combination Failed", err.Error())
	}

	// Safety check to prevent nil pointer dereference
	if result == nil {
		return createErrorEmbed(event, "Combination Error", "Failed to process combination result.")
	}

	// Check for collection completion after successful card combination
	go c.bot.CompletionChecker.CheckCompletionForCards(context.Background(), event.User().ID.String(), []int64{mainCard.CardID})

	// Track quest progress for combine
	if c.bot.QuestTracker != nil {
		// Check if card reached level 5 (max level) after combination
		oldLevel := mainCard.Level - (result.NewLevel - mainCard.Level) // Calculate old level before combine
		reachedMaxLevel := result.NewLevel == 5 && oldLevel < 5
		metadata := map[string]interface{}{
			"is_max_level": reachedMaxLevel,
			"new_level":    result.NewLevel,
			"old_level":    oldLevel,
			"action":       "combine",
		}
		go c.bot.QuestTracker.TrackCardLevelUpWithMetadata(context.Background(), event.User().ID.String(), 1, metadata)
	}

	// Get card details for display
	cardCombineDetails, err := c.cardRepo.GetByID(ctx, mainCard.CardID)
	if err != nil {
		return createErrorEmbed(event, "Error", "Failed to fetch card details")
	}

	cardInfo := utils.GetCardDisplayInfo(
		cardCombineDetails.Name,
		cardCombineDetails.ColID,
		result.NewLevel,
		utils.GetGroupType(cardCombineDetails.Tags),
		utils.SpacesConfig{
			Bucket:   c.bot.SpacesService.GetBucket(),
			Region:   c.bot.SpacesService.GetRegion(),
			CardRoot: c.bot.SpacesService.GetCardRoot(),
			GetImageURL: func(cardName string, colID string, level int, groupType string) string {
				return c.bot.SpacesService.GetCardImageURL(cardName, colID, level, groupType)
			},
		},
	)

	expBar := createExpBar(result.CurrentExp, result.RequiredExp)
	expPercentage := float64(result.CurrentExp) / float64(result.RequiredExp) * 100

	description := fmt.Sprintf("**%s**\n"+
		"``%s``\n\n"+
		"```ansi\n"+
		"\x1b[1;33mLevel %d %s\x1b[0m\n"+
		"\x1b[0;37mProgress Bar:\x1b[0m %s\n"+
		"\x1b[1;32m%.1f%%\x1b[0m Complete\n\n"+
		"\x1b[1;36mEXP Gained:\x1b[0m +%d\n"+
		"\x1b[1;36mCurrent EXP:\x1b[0m %d/%d\n"+
		"```",
		cardInfo.FormattedName,
		cardInfo.FormattedCollection,
		result.NewLevel,
		utils.GetPromoRarityDisplay(cardCombineDetails.ColID, result.NewLevel),
		expBar,
		expPercentage,
		result.ExpGained,
		result.CurrentExp,
		result.RequiredExp)

	if len(result.Bonuses) > 0 {
		description += fmt.Sprintf("\n> %s", strings.Join(result.Bonuses, "\n> "))
	}

	embed := discord.NewEmbedBuilder().
		SetTitle("Card Combination Result").
		SetColor(utils.GetDominantColor(cardInfo.ImageURL)).
		SetThumbnail(cardInfo.ImageURL).
		SetDescription(description)

    // Send followup asynchronously to keep handler fast
    go func() { _, _ = event.CreateFollowupMessage(discord.MessageCreate{Embeds: []discord.Embed{embed.Build()}}) }()
    return nil
}

func createErrorEmbed(event *handler.CommandEvent, title, description string) error {
	_, err := event.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title:       "❌ " + title,
			Description: description,
			Color:       config.ErrorColor,
		}},
	})
	return err
}

func createCooldownEmbed(event *handler.CommandEvent, _ *models.UserCard) error {
	_, err := event.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{{
			Title: "⏳ Experience Gain on Cooldown",
			Description: "```\n" +
				"This card needs time to rest!\n" +
				"Try again in a few minutes.\n" +
				"```\n" +
				"** Tip:** While waiting, you can:\n" +
				"• Level up other cards\n" +
				"• Combine cards of the same level\n" +
				"• Check your card collection",
			Color: 0xFFA500,
		}},
	})
	return err
}

func createExpBar(current, required int64) string {
	const barLength = 10
	progress := float64(current) / float64(required)
	filled := int(progress * float64(barLength))

	var bar strings.Builder
	bar.WriteString("『")
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar.WriteString("■")
		} else {
			bar.WriteString("□")
		}
	}
	bar.WriteString("』")
	return bar.String()
}

// findCard finds a card by name using enhanced search within user's collection
func (c *LevelUpCommand) findCard(ctx context.Context, userID, query string) (*models.Card, error) {
	// Handle empty query
	if query == "" {
		return nil, fmt.Errorf("please provide a card name")
	}

	// Try direct query first (optimized approach)
	card, err := c.cardRepo.GetByQuery(ctx, query)
	if err == nil {
		// Check if user owns this card
		userCard, err := c.cardRepo.GetUserCard(ctx, userID, card.ID)
		if err == nil && userCard.Amount > 0 {
			return card, nil
		}
	}

    // Fallback to single-query owned fuzzy search
    ownedCards, err := c.cardRepo.SearchOwnedByUserFuzzy(ctx, userID, query, 3)
    if err == nil && len(ownedCards) > 0 {
        return ownedCards[0], nil
    }

    // Final resilient fallback: weighted search within user's cards
    userCards, err2 := c.cardRepo.GetAllByUserID(ctx, userID)
    if err2 != nil {
        return nil, fmt.Errorf("failed to fetch user cards: %v", err2)
    }

    // Collect IDs and map
    cardIDs := make([]int64, 0, len(userCards))
    userCardMap := make(map[int64]*models.UserCard, len(userCards))
    for _, uc := range userCards {
        if uc.Amount > 0 {
            cardIDs = append(cardIDs, uc.CardID)
            userCardMap[uc.CardID] = uc
        }
    }
    if len(cardIDs) == 0 {
        return nil, fmt.Errorf("you don't have any cards available")
    }
    cards, err3 := c.cardRepo.GetByIDs(ctx, cardIDs)
    if err3 != nil {
        return nil, fmt.Errorf("failed to fetch card details: %v", err3)
    }
    filters := utils.ParseSearchQuery(query)
    results := utils.WeightedSearchWithMulti(cards, filters, userCardMap)
    if len(results) == 0 {
        return nil, fmt.Errorf("no cards found matching '%s'", query)
    }
    return results[0], nil
}

func LevelUpHandler(b *bottemplate.Bot) handler.CommandHandler {
	levelingService := cardleveling.NewService(
		cardleveling.NewDefaultConfig(),
		b.CardRepository,
	)

	cmd := NewLevelUpCommand(levelingService, b.CardRepository, b)

	return func(event *handler.CommandEvent) error {
		return cmd.Handle(event)
	}
}
