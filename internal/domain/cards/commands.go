package cards

import (
	"fmt"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/paginator"
)

type Commands interface {
	Cards(event *handler.CommandEvent) error
}

type commands struct {
	svc       Service
	paginator *paginator.Manager
}

func NewCommands(svc Service, paginator *paginator.Manager) *commands {
	return &commands{
		svc:       svc,
		paginator: paginator,
	}
}

func (c *commands) Cards(event *handler.CommandEvent) error {
	// Encapsulate this in a function
	filters := utils.FilterInfo{
		Name:       strings.TrimSpace(event.SlashCommandInteractionData().String("name")),
		Level:      int(event.SlashCommandInteractionData().Int("level")),
		Tags:       strings.TrimSpace(event.SlashCommandInteractionData().String("tags")),
		Collection: strings.TrimSpace(event.SlashCommandInteractionData().String("collection")),
		Animated:   event.SlashCommandInteractionData().Bool("animated"),
		Favorites:  event.SlashCommandInteractionData().Bool("favorites"),
	}

	cards, pages, err := c.svc.GetUserCards(event.User().ID.String(), filters)
	if err != nil {
		return err
	}

	return c.paginator.Create(event.Respond, paginator.Pages{
		ID:      event.ID().String(),
		Creator: event.User().ID,
		PageFunc: func(page int, embed *discord.EmbedBuilder) {
			startIdx := page * utils.CardsPerPage
			endIdx := min(startIdx+utils.CardsPerPage, len(cards))

			description := c.formatCardsDescription(cards[startIdx:endIdx])

			if utils.HasActiveFilters(filters) {
				description = utils.BuildFilterDescription(filters) + "\n\n" + description
			}

			embed.
				SetTitle("My Collection").
				SetDescription(description).
				SetColor(0x2B2D31).
				SetFooter(fmt.Sprintf("Page %d/%d • Total: %d", page+1, pages, len(cards)), "")
		},
		Pages:      pages,
		ExpireMode: paginator.ExpireModeAfterLastUsage,
	}, false)

}

func (c commands) formatCardsDescription(cards []Card) string {
	var description strings.Builder
	description.WriteString("```md\n")

	for _, card := range cards {

		// Format the card entry
		starRating := strings.Repeat("⭐", card.Level)
		favoriteIcon := ""
		if card.Favorite {
			favoriteIcon = "❤️"
		}

		animatedIcon := ""
		if card.Animated {
			animatedIcon = "✨"
		}

		// Add amount if more than 1
		amountText := ""
		if card.Amount > 1 {
			amountText = fmt.Sprintf(" x%d", card.Amount)
		}

		description.WriteString(fmt.Sprintf("* %s %s %s%s%s [%s]\n",
			starRating,
			utils.FormatCardName(card.Name),
			favoriteIcon,
			animatedIcon,
			amountText,
			strings.Trim(utils.FormatCollectionName(card.ColID), "[]"),
		))
	}

	description.WriteString("```")
	return description.String()
}
