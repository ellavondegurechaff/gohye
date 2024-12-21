package commands

import (
	"context"
	"fmt"
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
			Name:        "name",
			Description: "Name of the card to summon from your collection",
			Required:    true,
		},
	},
}

func SummonHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		cardName := e.SlashCommandInteractionData().String("name")
		userID := e.User().ID.String()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Get user's cards
		userCards, err := b.UserCardRepository.GetAllByUserID(ctx, userID)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{
					{
						Title:       "❌ Error",
						Description: "Failed to fetch your collection",
						Color:       utils.ErrorColor,
					},
				},
			})
		}

		// Get all cards to match against
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

		// Create a map of owned card IDs for quick lookup
		ownedCards := make(map[int64]*models.UserCard)
		for _, uc := range userCards {
			if uc.Amount > 0 {
				ownedCards[uc.CardID] = uc
			}
		}

		// Use weighted search to find the best match among owned cards
		filters := utils.ParseSearchQuery(cardName)
		filters.SortBy = utils.SortByLevel
		filters.SortDesc = true

		var matchedCard *models.Card
		searchResults := utils.WeightedSearch(cards, filters)

		for _, card := range searchResults {
			if _, owned := ownedCards[card.ID]; owned {
				matchedCard = card
				break
			}
		}

		if matchedCard == nil {
			return e.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{
					{
						Title:       "❌ Card Not Found",
						Description: fmt.Sprintf("```diff\n- No matching cards found in your collection for '%s'\n```", cardName),
						Color:       utils.ErrorColor,
						Footer: &discord.EmbedFooter{
							Text: "Try /inventory to see your available cards",
						},
					},
				},
			})
		}

		return displayCard(e, matchedCard, b)
	}
}

// displayCard handles the card display logic
func displayCard(e *handler.CommandEvent, card *models.Card, b *bottemplate.Bot) error {
	// Add initial debug log
	fmt.Printf("Displaying card: ID=%d, Name=%s, Animated=%v\n", card.ID, card.Name, card.Animated)

	config := utils.SpacesConfig{
		Bucket:   b.SpacesService.GetBucket(),
		Region:   b.SpacesService.GetRegion(),
		CardRoot: b.SpacesService.GetCardRoot(),
		GetImageURL: func(cardName string, colID string, level int, groupType string) string {
			// Get base URL without any extension
			baseURL := strings.TrimSuffix(b.SpacesService.GetCardImageURL(cardName, colID, level, groupType), ".jpg")

			// Add the correct extension
			extension := ".jpg"
			if card.Animated {
				extension = ".gif"
				fmt.Printf("Card is animated, using .gif extension\n")
			}

			fullURL := baseURL + extension

			// Log URL construction details
			fmt.Printf("Image URL construction:\n"+
				"- Base URL: %s\n"+
				"- Extension: %s\n"+
				"- Full URL: %s\n",
				baseURL, extension, fullURL)

			return fullURL
		},
	}

	// Log spaces configuration
	fmt.Printf("Spaces Configuration:\n"+
		"- Bucket: %s\n"+
		"- Region: %s\n"+
		"- Card Root: %s\n",
		config.Bucket, config.Region, config.CardRoot)

	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		utils.GetGroupType(card.Tags),
		config,
	)

	// Log card display info
	fmt.Printf("Card Display Info:\n"+
		"- Formatted Name: %s\n"+
		"- Image URL: %s\n"+
		"- Collection: %s\n",
		cardInfo.FormattedName, cardInfo.ImageURL, cardInfo.FormattedCollection)

	timestamp := fmt.Sprintf("<t:%d:R>", time.Now().Unix())

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

	// Log final embed details
	fmt.Printf("Creating embed with image URL: %s\n", cardInfo.ImageURL)

	err := e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})

	if err != nil {
		fmt.Printf("Error creating message: %v\n", err)
		return err
	}

	fmt.Printf("Successfully sent card embed\n")
	return nil
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
