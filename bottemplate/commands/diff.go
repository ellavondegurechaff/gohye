package commands

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/bot-template/internal/gateways/database/models"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/paginator"
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
					Name:        "card_query",
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
					Name:        "card_query",
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
		query := strings.TrimSpace(data.String("card_query"))

		var diffCards []*models.Card
		var title string
		var err error

		switch subCmd {
		case "for":
			diffCards, err = getDiffForCards(ctx, b, e, targetUser)
			title = fmt.Sprintf("Cards you have that %s doesn't", targetUser.Username)
		case "from":
			diffCards, err = getDiffFromCards(ctx, b, e, targetUser)
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
			filteredCards := utils.WeightedSearch(diffCards, query, utils.SearchModePartial)
			if len(filteredCards) == 0 {
				return utils.EH.CreateErrorEmbed(e, fmt.Sprintf("No cards match the query: %s", query))
			}
			diffCards = filteredCards
		}

		// Sort cards by level (descending) and name (ascending)
		sort.Slice(diffCards, func(i, j int) bool {
			if diffCards[i].Level != diffCards[j].Level {
				return diffCards[i].Level > diffCards[j].Level
			}
			return diffCards[i].Name < diffCards[j].Name
		})

		totalPages := int(math.Ceil(float64(len(diffCards)) / float64(utils.CardsPerPage)))

		return b.Paginator.Create(e.Respond, paginator.Pages{
			ID:      e.ID().String(),
			Creator: e.User().ID,
			PageFunc: func(page int, embed *discord.EmbedBuilder) {
				startIdx := page * utils.CardsPerPage
				endIdx := min(startIdx+utils.CardsPerPage, len(diffCards))
				pageCards := diffCards[startIdx:endIdx]

				var description strings.Builder
				description.WriteString("```ansi\n")

				if query != "" {
					description.WriteString(fmt.Sprintf("Filtering by: \x1b[33m%s\x1b[0m\n\n", query))
				}

				for _, card := range pageCards {
					animatedIcon := ""
					if card.Animated {
						animatedIcon = "✨"
					}

					description.WriteString(fmt.Sprintf("%s \x1b[32m%s\x1b[0m%s [%s]\n",
						strings.Repeat("⭐", card.Level),
						utils.FormatCardName(card.Name),
						animatedIcon,
						strings.Trim(utils.FormatCollectionName(card.ColID), "[]"),
					))
				}

				description.WriteString("```")

				embed.
					SetTitle(title).
					SetDescription(description.String()).
					SetColor(0x2B2D31).
					SetFooter(fmt.Sprintf("Page %d/%d • Total Cards: %d", page+1, totalPages, len(diffCards)), "")
			},
			Pages:      totalPages,
			ExpireMode: paginator.ExpireModeAfterLastUsage,
		}, false)
	}
}

func getDiffForCards(ctx context.Context, b *bottemplate.Bot, e *handler.CommandEvent, targetUser discord.User) ([]*models.Card, error) {
	userCards, err := b.UserCardRepository.GetAllByUserID(ctx, e.User().ID.String())
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

func getDiffFromCards(ctx context.Context, b *bottemplate.Bot, e *handler.CommandEvent, targetUser discord.User) ([]*models.Card, error) {
	userCards, err := b.UserCardRepository.GetAllByUserID(ctx, e.User().ID.String())
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
