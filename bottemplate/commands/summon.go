package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Summon = discord.SlashCommandCreate{
	Name:        "summon",
	Description: "✨ Summon a card from your collection",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "query",
			Description: "Card ID or name to summon",
			Required:    true,
		},
	},
}

func SummonHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		query := e.SlashCommandInteractionData().String("query")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Try parsing as ID first
		if cardID, err := strconv.ParseInt(query, 10, 64); err == nil {
			card, err := b.CardRepository.GetByID(ctx, cardID)
			if err == nil {
				return displayCard(e, card, b)
			}
		}

		// If not ID or card not found, search by name
		cards, err := b.CardRepository.GetAll(ctx)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{
					{
						Title:       "❌ Error",
						Description: "Failed to search for cards",
						Color:       utils.ErrorColor,
					},
				},
			})
		}

		// Use weighted search to find the best match
		searchResults := utils.WeightedSearch(cards, query, utils.SearchModeExact)
		if len(searchResults) == 0 {
			return e.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{
					{
						Title:       "❌ Card Not Found",
						Description: fmt.Sprintf("```diff\n- No cards found matching '%s'\n```", query),
						Color:       utils.ErrorColor,
						Footer: &discord.EmbedFooter{
							Text: "Try /searchcards to find valid cards",
						},
					},
				},
			})
		}

		// Use the best match
		return displayCard(e, searchResults[0], b)
	}
}

// displayCard handles the card display logic
func displayCard(e *handler.CommandEvent, card *models.Card, b *bottemplate.Bot) error {

	config := utils.SpacesConfig{
		Bucket:   b.SpacesService.GetBucket(),
		Region:   b.SpacesService.GetRegion(),
		CardRoot: b.SpacesService.GetCardRoot(),
		GetImageURL: func(cardName string, colID string, level int, groupType string) string {
			return b.SpacesService.GetCardImageURL(cardName, colID, level, groupType)
		},
	}

	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		utils.GetGroupType(card.Tags),
		config,
	)

	timestamp := fmt.Sprintf("<t:%d:R>", time.Now().Unix())

	// Create embed with image
	embed := discord.Embed{
		Title: cardInfo.FormattedName,
		Color: utils.GetColorByLevel(card.Level),
		Description: fmt.Sprintf("```md\n"+
			"# Card Information\n"+
			"* Collection: %s\n"+
			"* Level: %s\n"+
			"* ID: #%d\n"+
			"%s\n"+
			"```\n"+
			"> %s\n\n"+
			"Use `/inventory` to view your collection",
			cardInfo.FormattedCollection,
			strings.Repeat("⭐", card.Level),
			card.ID,
			utils.GetAnimatedTag(card.Animated),
			getCardQuote(card.Level)),
		Image: &discord.EmbedResource{
			URL: cardInfo.ImageURL,
		},
		Footer: &discord.EmbedFooter{
			Text:    fmt.Sprintf("Summoned by %s • %s", e.User().Username, timestamp),
			IconURL: e.User().EffectiveAvatarURL(),
		},
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
}

// Helper functions for card formatting and display
func getColorByLevel(level int) int {
	colors := map[int]int{
		1: 0x808080, // Gray for Common
		2: 0x00FF00, // Green for Uncommon
		3: 0x0000FF, // Blue for Rare
		4: 0x800080, // Purple for Epic
		5: 0xFFD700, // Gold for Legendary
	}
	if color, exists := colors[level]; exists {
		return color
	}
	return 0x000000 // Default black
}

func getRarityName(level int) string {
	rarities := map[int]string{
		1: "Common",
		2: "Uncommon",
		3: "Rare",
		4: "Epic",
		5: "Legendary",
	}
	if rarity, exists := rarities[level]; exists {
		return rarity
	}
	return "Unknown"
}

func getAnimatedTag(animated bool) string {
	if animated {
		return "* ✨ Animated Card"
	}
	return ""
}

func getCardPowerLevel(level int) string {
	return strings.Repeat("■", level) + strings.Repeat("□", 5-level)
}

func getCardQuote(level int) string {
	quotes := []string{
		"A humble beginning to your collection journey.",
		"An uncommon find, growing in power.",
		"A rare gem shines in your collection!",
		"An epic discovery that few possess!",
		"A legendary artifact of immense power!",
	}
	if level >= 1 && level <= 5 {
		return quotes[level-1]
	}
	return "A mysterious card of unknown origin."
}

func getCollectionIcon(colID string) string {
	// You can implement your own logic to return collection-specific icons
	// This is a placeholder implementation
	return fmt.Sprintf("https://your-cdn.com/icons/%s.png", colID)
}

func formatTags(tags []string) string {
	if len(tags) == 0 {
		return "No tags"
	}

	formattedTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		switch tag {
		case "girlgroups":
			formattedTags = append(formattedTags, "👯‍♀️ Girl Group")
		case "boygroups":
			formattedTags = append(formattedTags, "👯‍♂️ Boy Group")
		case "soloist":
			formattedTags = append(formattedTags, "👤 Solo Artist")
		default:
			formattedTags = append(formattedTags, tag)
		}
	}

	return strings.Join(formattedTags, ", ")
}
