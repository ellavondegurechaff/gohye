package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/internal/domain/cards"
	"github.com/disgoorg/bot-template/internal/gateways/database/models"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/disgoorg/bot-template/bottemplate/utils"

	"github.com/disgoorg/bot-template/bottemplate/cardleveling"
)

var LevelUp = &discord.SlashCommandCreate{
	Name:        "levelup",
	Description: "Level up or combine your cards",
	Options: []discord.ApplicationCommandOption{
		&discord.ApplicationCommandOptionInt{
			Name:        "card_id",
			Description: "The ID of the card to level up",
			Required:    true,
			MinValue:    intPtr(1),
		},
		&discord.ApplicationCommandOptionInt{
			Name:        "combine_with",
			Description: "Optional: ID of another card to combine with",
			Required:    false,
			MinValue:    intPtr(1),
		},
	},
}

type LevelUpCommand struct {
	levelingService *cardleveling.Service
	cardRepo        cards.Repository
	bot             *bottemplate.Bot
}

func NewLevelUpCommand(levelingService *cardleveling.Service, cardRepo cards.Repository, bot *bottemplate.Bot) *LevelUpCommand {
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

	cardID := event.SlashCommandInteractionData().Int("card_id")
	userCard, err := c.cardRepo.GetUserCard(ctx, event.User().ID.String(), int64(cardID))
	if err != nil {
		return createErrorEmbed(event, "Card Not Found", "Could not find the specified card. Please check the card ID.")
	}

	if combineWith := event.SlashCommandInteractionData().Int("combine_with"); combineWith != 0 {
		return c.handleCombine(event, userCard, int64(combineWith))
	}

	result, err := c.levelingService.GainExp(ctx, userCard)
	if err != nil {
		if err.Error() == "exp gain on cooldown" {
			return createCooldownEmbed(event, userCard)
		}
		return createErrorEmbed(event, "Error", err.Error())
	}

	// Get card details for name display
	card, err := c.cardRepo.GetByID(ctx, userCard.CardID)
	if err != nil {
		return createErrorEmbed(event, "Error", "Failed to fetch card details")
	}

	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		result.NewLevel,
		utils.GetGroupType(card.Tags),
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

func (c *LevelUpCommand) handleCombine(event *handler.CommandEvent, mainCard *models.UserCard, fodderCardID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fodderCard, err := c.cardRepo.GetUserCard(ctx, event.User().ID.String(), fodderCardID)
	if err != nil {
		return createErrorEmbed(event, "Error", "Failed to find fodder card")
	}

	result, err := c.levelingService.CombineCards(ctx, mainCard, fodderCard)
	if err != nil {
		return createErrorEmbed(event, "Combination Failed", err.Error())
	}

	// Get card details for display
	card, err := c.cardRepo.GetByID(ctx, mainCard.CardID)
	if err != nil {
		return createErrorEmbed(event, "Error", "Failed to fetch card details")
	}

	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		result.NewLevel,
		utils.GetGroupType(card.Tags),
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
			Color:       0xFF0000,
		}},
	})
	return err
}

func createCooldownEmbed(event *handler.CommandEvent, card *models.UserCard) error {
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
