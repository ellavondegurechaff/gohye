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
	card, err := c.cardRepo.GetByQuery(ctx, cardQuery)
	if err != nil {
		return createErrorEmbed(event, "Card Not Found", fmt.Sprintf("Could not find a card matching '%s'. Please check the card name or ID.", cardQuery))
	}

	userCard, err := c.cardRepo.GetUserCard(ctx, event.User().ID.String(), card.ID)
	if err != nil {
		return createErrorEmbed(event, "Card Not Owned", fmt.Sprintf("You don't own the card '%s'. Please check your collection.", card.Name))
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

	// Check for collection completion after successful level up
	go c.bot.CompletionChecker.CheckCompletionForCards(context.Background(), event.User().ID.String(), []int64{userCard.CardID})

	// Get card details for name display
	cardDetails, err := c.cardRepo.GetByID(ctx, userCard.CardID)
	if err != nil {
		return createErrorEmbed(event, "Error", "Failed to fetch card details")
	}

	cardInfo := utils.GetCardDisplayInfo(
		cardDetails.Name,
		cardDetails.ColID,
		result.NewLevel,
		utils.GetGroupType(cardDetails.Tags),
		utils.SpacesConfig{
			Bucket:   c.bot.SpacesService.GetBucket(),
			Region:   c.bot.SpacesService.GetRegion(),
			CardRoot: c.bot.SpacesService.GetCardRoot(),
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
			result.NewLevel,
			utils.GetStarsDisplay(result.NewLevel),
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

	_, err = event.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed.Build()},
	})
	return err
}

func (c *LevelUpCommand) handleCombine(event *handler.CommandEvent, mainCard *models.UserCard, fodderCardQuery string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fodderCard, err := c.cardRepo.GetByQuery(ctx, fodderCardQuery)
	if err != nil {
		return createErrorEmbed(event, "Fodder Card Not Found", fmt.Sprintf("Could not find a card matching '%s' to combine with.", fodderCardQuery))
	}

	userFodderCard, err := c.cardRepo.GetUserCard(ctx, event.User().ID.String(), fodderCard.ID)
	if err != nil {
		return createErrorEmbed(event, "Fodder Card Not Owned", fmt.Sprintf("You don't own the card '%s' to use for combining.", fodderCard.Name))
	}

	result, err := c.levelingService.CombineCards(ctx, mainCard, userFodderCard)
	if err != nil {
		return createErrorEmbed(event, "Combination Failed", err.Error())
	}

	// Check for collection completion after successful card combination
	go c.bot.CompletionChecker.CheckCompletionForCards(context.Background(), event.User().ID.String(), []int64{mainCard.CardID})

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
		utils.GetStarsDisplay(result.NewLevel),
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

	_, err = event.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed.Build()},
	})
	return err
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
