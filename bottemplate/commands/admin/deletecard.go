package admin

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/disgoorg/bot-template/bottemplate"
    "github.com/disgoorg/bot-template/bottemplate/config"
    "github.com/disgoorg/bot-template/bottemplate/utils"
    "github.com/disgoorg/disgo/discord"
    "github.com/disgoorg/disgo/handler"
)

var DeleteCard = discord.SlashCommandCreate{
	Name:        "deletecard",
	Description: "Permanently delete a card and remove it from all users",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionInt{
			Name:        "card_id",
			Description: "The ID of the card to delete",
			Required:    true,
		},
		discord.ApplicationCommandOptionBool{
			Name:        "confirm",
			Description: "Confirm that you want to delete this card",
			Required:    true,
		},
	},
}

func getGroupType(tags []string) string {
	for _, tag := range tags {
		if tag == "girlgroups" || tag == "boygroups" {
			return tag
		}
	}
	return "unknown"
}

func DeleteCardHandler(b *bottemplate.Bot) handler.CommandHandler {
    return func(e *handler.CommandEvent) error {
        if err := e.DeferCreateMessage(false); err != nil { return err }
        cardID := int64(e.SlashCommandInteractionData().Int("card_id"))
        confirm := e.SlashCommandInteractionData().Bool("confirm")

        if !confirm {
            _, err := e.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr("⚠️ You must confirm the deletion by setting the confirm option to true.")})
            return err
        }

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

        card, err := b.CardRepository.GetByID(ctx, cardID)
        if err != nil {
            _, updErr := e.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{{
                Title:       "❌ Error",
                Description: fmt.Sprintf("Card with ID `%d` does not exist.", cardID),
                Color:       config.ErrorColor,
            }}})
            return updErr
        }

		report, err := b.CardRepository.SafeDelete(ctx, cardID)
        if err != nil {
            _, updErr := e.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{{
                Title:       "❌ Error",
                Description: "An error occurred while deleting the card. Please try again later.",
                Color:       config.ErrorColor,
            }}})
            return updErr
        }

		// Create inline value for field booleans
		inlineTrue := true

		// Get current time as Unix timestamp
		now := time.Now().Unix()
		timestampStr := fmt.Sprintf("<t:%d:f>", now)

		// Create a styled embed for the deletion report
        _, updErr := e.UpdateInteractionResponse(discord.MessageUpdate{Embeds: &[]discord.Embed{{
            Title:       "✅ Card Deletion Completed",
            Color:       config.SuccessColor,
            Description: fmt.Sprintf("Operation completed at %s", timestampStr),
            Fields: []discord.EmbedField{{
                Name:   "Card Details",
                Value:  fmt.Sprintf("**Name:** %s\n**ID:** %d\n**Level:** %d\n**Collection:** %s\n**Tags:** %s", card.Name, card.ID, card.Level, card.ColID, strings.Join(card.Tags, ", ")),
                Inline: &inlineTrue,
            },{
                Name:   "Database Changes",
                Value:  fmt.Sprintf("• Removed from **%d** user inventories\n• Card entry deleted: **%v**", report.UserCardsDeleted, report.CardDeleted),
                Inline: &inlineTrue,
            },{
                Name: "Storage Cleanup",
                Value: fmt.Sprintf("Attempted to delete:\n• `%s/%s/%s/%d_%s.jpg`\n• `%s/%s.jpg`",
                    b.SpacesService.CardRoot,
                    getGroupType(card.Tags),
                    card.ColID,
                    card.Level,
                    card.Name,
                    card.ColID,
                    card.Name),
                Inline: &inlineTrue,
            }},
            Footer: &discord.EmbedFooter{Text: "Card Deletion System"},
        }}})
        return updErr
    }
}
