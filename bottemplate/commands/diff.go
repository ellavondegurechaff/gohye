package commands

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
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
	return func(e *handler.CommandEvent) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
			diffCards, err = getDiffForCards(ctx, b, e.User().ID.String(), &targetUser)
			title = fmt.Sprintf("Cards you have that %s doesn't", targetUser.Username)
		case "from":
			diffCards, err = getDiffFromCards(ctx, b, e.User().ID.String(), &targetUser)
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
			diffCards = utils.WeightedSearch(diffCards, filters)
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

		totalPages := int(math.Ceil(float64(len(diffCards)) / float64(utils.CardsPerPage)))
		startIdx := 0
		endIdx := min(utils.CardsPerPage, len(diffCards))

		// Create initial embed
		embed := discord.NewEmbedBuilder().
			SetTitle(title).
			SetDescription(formatDiffCardsDescription(diffCards[startIdx:endIdx], b, query)).
			SetColor(0x2B2D31).
			SetFooter(fmt.Sprintf("Page 1/%d • Total: %d", totalPages, len(diffCards)), "")

		// Create navigation buttons
		components := []discord.ContainerComponent{
			discord.NewActionRow(
				discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/diff/prev/%s/0/%s/%s/%s", e.User().ID, subCmd, targetUser.ID, query)),
				discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/diff/next/%s/0/%s/%s/%s", e.User().ID, subCmd, targetUser.ID, query)),
				discord.NewSecondaryButton("📋 Copy Page", fmt.Sprintf("/diff/copy/%s/0/%s/%s/%s", e.User().ID, subCmd, targetUser.ID, query)),
			),
		}

		return e.CreateMessage(discord.MessageCreate{
			Embeds:     []discord.Embed{embed.Build()},
			Components: components,
		})
	}
}

func formatDiffCardsDescription(cards []*models.Card, b *bottemplate.Bot, query string) string {
	var description strings.Builder

	if query != "" {
		description.WriteString(fmt.Sprintf("🔍`%s`\n\n", query))
	}

	for _, card := range cards {
		displayInfo := utils.GetCardDisplayInfo(
			card.Name,
			card.ColID,
			card.Level,
			utils.GetGroupType(card.Tags),
			b.SpacesService.GetSpacesConfig(),
		)

		entry := utils.FormatCardEntry(
			displayInfo,
			false, // not favorite for diff
			card.Animated,
			0, // amount is 0 for diff
		)

		description.WriteString(entry + "\n")
	}

	return description.String()
}

func formatDiffCopyText(cards []*models.Card, title string) string {
	var sb strings.Builder
	sb.WriteString(title + "\n")

	for _, card := range cards {
		stars := strings.Repeat("★", card.Level)
		sb.WriteString(fmt.Sprintf("%s %s [%s]\n", stars, utils.FormatCardName(card.Name), card.ColID))
	}

	return sb.String()
}

// Add component handler for diff command
func DiffComponentHandler(b *bottemplate.Bot) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		data := e.Data.(discord.ButtonInteractionData)
		customID := data.CustomID()

		parts := strings.Split(customID, "/")
		if len(parts) < 7 {
			return nil
		}

		userID := parts[3]
		currentPage, err := strconv.Atoi(parts[4])
		if err != nil {
			return nil
		}

		subCmd := parts[5]
		targetUserID := parts[6]
		query := strings.Join(parts[7:], "/")

		// Only the original user can interact
		if e.User().ID.String() != userID {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Only the command user can navigate through these cards.",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Parse the target user ID using snowflake package
		targetSnowflake, err := snowflake.Parse(targetUserID)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Invalid user ID",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		targetUser, err := b.Client.Rest().GetUser(targetSnowflake)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Failed to fetch target user",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		ctx := context.Background()
		var diffCards []*models.Card
		var title string

		// Get diff cards based on subcommand
		if subCmd == "for" {
			diffCards, err = getDiffForCards(ctx, b, userID, targetUser)
			title = fmt.Sprintf("Cards you have that %s doesn't", targetUser.Username)
		} else {
			diffCards, err = getDiffFromCards(ctx, b, userID, targetUser)
			title = fmt.Sprintf("Cards %s has that you don't", targetUser.Username)
		}

		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: "Failed to fetch diff cards",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Apply search filters if query exists
		if query != "" {
			filters := utils.ParseSearchQuery(query)
			diffCards = utils.WeightedSearch(diffCards, filters)
		} else {
			sort.Slice(diffCards, func(i, j int) bool {
				if diffCards[i].Level != diffCards[j].Level {
					return diffCards[i].Level > diffCards[j].Level
				}
				return strings.ToLower(diffCards[i].Name) < strings.ToLower(diffCards[j].Name)
			})
		}

		totalPages := int(math.Ceil(float64(len(diffCards)) / float64(utils.CardsPerPage)))

		// Handle copy button
		if strings.HasPrefix(customID, "/diff/copy/") {
			startIdx := currentPage * utils.CardsPerPage
			endIdx := min(startIdx+utils.CardsPerPage, len(diffCards))

			copyText := formatDiffCopyText(diffCards[startIdx:endIdx], title)

			return e.CreateMessage(discord.MessageCreate{
				Content: "```\n" + copyText + "```",
				Flags:   discord.MessageFlagEphemeral,
			})
		}

		// Calculate new page
		newPage := currentPage
		if strings.HasPrefix(customID, "/diff/next/") {
			newPage = (currentPage + 1) % totalPages
		} else if strings.HasPrefix(customID, "/diff/prev/") {
			newPage = (currentPage - 1 + totalPages) % totalPages
		}

		startIdx := newPage * utils.CardsPerPage
		endIdx := min(startIdx+utils.CardsPerPage, len(diffCards))

		// Update the embed
		embed := e.Message.Embeds[0]
		embed.Description = formatDiffCardsDescription(diffCards[startIdx:endIdx], b, query)
		embed.Footer.Text = fmt.Sprintf("Page %d/%d • Total: %d", newPage+1, totalPages, len(diffCards))

		// Update navigation buttons
		components := []discord.ContainerComponent{
			discord.NewActionRow(
				discord.NewSecondaryButton("◀ Previous", fmt.Sprintf("/diff/prev/%s/%d/%s/%s/%s", userID, newPage, subCmd, targetUserID, query)),
				discord.NewSecondaryButton("Next ▶", fmt.Sprintf("/diff/next/%s/%d/%s/%s/%s", userID, newPage, subCmd, targetUserID, query)),
				discord.NewSecondaryButton("📋 Copy Page", fmt.Sprintf("/diff/copy/%s/%d/%s/%s/%s", userID, newPage, subCmd, targetUserID, query)),
			),
		}

		return e.UpdateMessage(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &components,
		})
	}
}

func getDiffForCards(ctx context.Context, b *bottemplate.Bot, userID string, targetUser *discord.User) ([]*models.Card, error) {
	userCards, err := b.UserCardRepository.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch your cards")
	}

	targetCards, err := b.UserCardRepository.GetAllByUserID(ctx, targetUser.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target user's cards")
	}

	targetOwned := make(map[int64]bool)
	for _, tc := range targetCards {
		targetOwned[tc.CardID] = true
	}

	allCards, err := b.CardRepository.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cards")
	}

	cardMap := make(map[int64]*models.Card)
	for _, card := range allCards {
		cardMap[card.ID] = card
	}

	var diffCards []*models.Card
	for _, uc := range userCards {
		if !targetOwned[uc.CardID] {
			if card, exists := cardMap[uc.CardID]; exists {
				diffCards = append(diffCards, card)
			}
		}
	}

	return diffCards, nil
}

func getDiffFromCards(ctx context.Context, b *bottemplate.Bot, userID string, targetUser *discord.User) ([]*models.Card, error) {
	userCards, err := b.UserCardRepository.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch your cards")
	}

	targetCards, err := b.UserCardRepository.GetAllByUserID(ctx, targetUser.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target user's cards")
	}

	userOwned := make(map[int64]bool)
	for _, uc := range userCards {
		userOwned[uc.CardID] = true
	}

	allCards, err := b.CardRepository.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cards")
	}

	cardMap := make(map[int64]*models.Card)
	for _, card := range allCards {
		cardMap[card.ID] = card
	}

	var diffCards []*models.Card
	for _, tc := range targetCards {
		if !userOwned[tc.CardID] {
			if card, exists := cardMap[tc.CardID]; exists {
				diffCards = append(diffCards, card)
			}
		}
	}

	return diffCards, nil
}
