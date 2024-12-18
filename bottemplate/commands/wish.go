package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Wish = discord.SlashCommandCreate{
	Name:        "wish",
	Description: "Manage your card wishlist",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionSubCommand{
			Name:        "list",
			Description: "Display your wishlist",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "card_query",
					Description: "Filter cards by name, collection, or other attributes",
					Required:    false,
				},
				discord.ApplicationCommandOptionUser{
					Name:        "user",
					Description: "View another user's wishlist",
					Required:    false,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "add",
			Description: "Add cards to your wishlist",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "card_query",
					Description: "Cards to add to wishlist (name or ID)",
					Required:    true,
				},
				discord.ApplicationCommandOptionBool{
					Name:        "exact",
					Description: "Match exact card name only",
					Required:    false,
				},
			},
		},
		discord.ApplicationCommandOptionSubCommand{
			Name:        "remove",
			Description: "Remove cards from your wishlist",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:        "card_query",
					Description: "Cards to remove from wishlist",
					Required:    true,
				},
			},
		},
	},
}

func WishHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if err := e.DeferCreateMessage(false); err != nil {
			return fmt.Errorf("failed to defer response: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		data := e.SlashCommandInteractionData()
		subCmd := data.SubCommandName

		var err error
		switch *subCmd {
		case "list":
			err = handleWishList(ctx, b, e)
		case "add":
			exact := data.Bool("exact")
			if exact {
				err = handleWishOne(ctx, b, e)
			} else {
				err = handleWishMany(ctx, b, e)
			}
		case "remove":
			err = handleWishRemove(ctx, b, e)
		default:
			return utils.EH.CreateErrorEmbed(e, "Invalid subcommand")
		}

		if err != nil {
			_, updateErr := e.UpdateInteractionResponse(discord.MessageUpdate{
				Embeds: &[]discord.Embed{{
					Description: "❌ " + err.Error(),
					Color:       utils.ErrorColor,
				}},
			})
			if updateErr != nil {
				return fmt.Errorf("failed to update error response: %w", updateErr)
			}
			return err
		}

		return nil
	}
}

func handleWishOne(ctx context.Context, b *bottemplate.Bot, e *handler.CommandEvent) error {
	query := e.SlashCommandInteractionData().String("card_query")
	userID := e.User().ID.String()

	// Try to find the card
	cards, err := b.CardRepository.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to search for cards")
	}

	searchResults := utils.WeightedSearch(cards, query, utils.SearchModeExact)
	if len(searchResults) == 0 {
		return fmt.Errorf("no cards found matching '%s'", query)
	}

	card := searchResults[0]

	// Get current wishlist to check for duplicates
	wishlist, err := b.WishlistRepository.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check wishlist: %w", err)
	}

	// Add index lookup optimization
	wishMap := make(map[int64]bool, len(wishlist))
	for _, wish := range wishlist {
		wishMap[wish.CardID] = true
	}

	// Check if card is already in wishlist using map
	if wishMap[card.ID] {
		return fmt.Errorf("card '%s' is already in your wishlist", card.Name)
	}

	err = b.WishlistRepository.Add(ctx, userID, card.ID)
	if err != nil {
		return fmt.Errorf("failed to add card to wishlist")
	}

	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		utils.GetGroupType(card.Tags),
		b.SpacesService.GetSpacesConfig(),
	)

	embed := formatWishEmbed(cardInfo, card)

	_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
	})
	return err
}

func handleWishMany(ctx context.Context, b *bottemplate.Bot, e *handler.CommandEvent) error {
	query := e.SlashCommandInteractionData().String("card_query")
	userID := e.User().ID.String()

	// Search for cards
	cards, err := b.CardRepository.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to search for cards")
	}

	searchResults := utils.WeightedSearch(cards, query, utils.SearchModePartial)
	if len(searchResults) == 0 {
		return fmt.Errorf("no cards found matching '%s'", query)
	}

	// Only process the first matching card
	card := searchResults[0]

	// Check if card is already in wishlist
	wishlist, err := b.WishlistRepository.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check wishlist: %w", err)
	}

	wishMap := make(map[int64]bool, len(wishlist))
	for _, wish := range wishlist {
		wishMap[wish.CardID] = true
	}

	if wishMap[card.ID] {
		return fmt.Errorf("card '%s' is already in your wishlist", card.Name)
	}

	if err := b.WishlistRepository.Add(ctx, userID, card.ID); err != nil {
		return fmt.Errorf("failed to add card to wishlist")
	}

	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		utils.GetGroupType(card.Tags),
		b.SpacesService.GetSpacesConfig(),
	)

	embed := formatWishEmbed(cardInfo, card)

	_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
	})
	return err
}

func handleWishRemove(ctx context.Context, b *bottemplate.Bot, e *handler.CommandEvent) error {
	query := e.SlashCommandInteractionData().String("card_query")
	userID := e.User().ID.String()

	// Get user's wishlist
	wishlist, err := b.WishlistRepository.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to fetch wishlist")
	}

	// Get all cards
	cards, err := b.CardRepository.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch cards")
	}

	// Create a map of wished cards for faster lookup
	wishedCards := make([]*models.Card, 0)
	for _, wish := range wishlist {
		for _, card := range cards {
			if card.ID == wish.CardID {
				wishedCards = append(wishedCards, card)
				break
			}
		}
	}

	// Use WeightedSearch to find the best match from wished cards
	searchResults := utils.WeightedSearch(wishedCards, query, utils.SearchModePartial)
	if len(searchResults) == 0 {
		return fmt.Errorf("no matching cards found in your wishlist for '%s'", query)
	}

	cardToRemove := searchResults[0]

	if err := b.WishlistRepository.Remove(ctx, userID, cardToRemove.ID); err != nil {
		return fmt.Errorf("failed to remove card from wishlist")
	}

	cardInfo := utils.GetCardDisplayInfo(
		cardToRemove.Name,
		cardToRemove.ColID,
		cardToRemove.Level,
		utils.GetGroupType(cardToRemove.Tags),
		b.SpacesService.GetSpacesConfig(),
	)

	description := fmt.Sprintf("```ansi\n\x1b[32m%s\x1b[0m [%s] %s\n```",
		cardInfo.FormattedName,
		strings.Repeat("⭐", cardToRemove.Level),
		cardInfo.FormattedCollection)

	embed := discord.NewEmbedBuilder().
		SetTitle("Card Removed from Wishlist").
		SetDescription(description).
		SetColor(0x2b2d31).
		Build()

	_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
	})
	return err
}

func handleWishList(ctx context.Context, b *bottemplate.Bot, e *handler.CommandEvent) error {
	targetUser := e.User()
	if user, ok := e.SlashCommandInteractionData().OptUser("user"); ok {
		targetUser = user
	}

	query := e.SlashCommandInteractionData().String("card_query")

	wishlist, err := b.WishlistRepository.GetByUserID(ctx, targetUser.ID.String())
	if err != nil {
		return fmt.Errorf("failed to fetch wishlist")
	}

	if len(wishlist) == 0 {
		return fmt.Errorf("no cards in wishlist")
	}

	var description strings.Builder
	description.WriteString("```ansi\n")

	// Get all cards for reference
	cards, err := b.CardRepository.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch cards")
	}

	// Create a map for faster card lookups
	cardMap := make(map[int64]*models.Card)
	for _, card := range cards {
		cardMap[card.ID] = card
	}

	// Get user's owned cards
	userCards, err := b.UserCardRepository.GetAllByUserID(ctx, targetUser.ID.String())
	if err != nil {
		return fmt.Errorf("failed to fetch user cards")
	}

	// Create a map of owned cards
	ownedCards := make(map[int64]bool)
	for _, uc := range userCards {
		ownedCards[uc.CardID] = true
	}

	var displayedCards []*models.Card
	for _, wish := range wishlist {
		if card, exists := cardMap[wish.CardID]; exists {
			if query == "" || strings.Contains(strings.ToLower(card.Name), strings.ToLower(query)) {
				displayedCards = append(displayedCards, card)
			}
		}
	}

	// Sort cards by level and name
	sort.Slice(displayedCards, func(i, j int) bool {
		if displayedCards[i].Level != displayedCards[j].Level {
			return displayedCards[i].Level > displayedCards[j].Level
		}
		return displayedCards[i].Name < displayedCards[j].Name
	})

	for _, card := range displayedCards {
		ownedMark := "❌"
		if ownedCards[card.ID] {
			ownedMark = "✅"
		}

		cardInfo := utils.GetCardDisplayInfo(
			card.Name,
			card.ColID,
			card.Level,
			utils.GetGroupType(card.Tags),
			b.SpacesService.GetSpacesConfig(),
		)

		description.WriteString(fmt.Sprintf("%s \x1b[32m%s\x1b[0m [%s] %s\n",
			ownedMark,
			cardInfo.FormattedName,
			strings.Repeat("⭐", card.Level),
			cardInfo.FormattedCollection))
	}

	description.WriteString("```")

	embed := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("%s's Wishlist", targetUser.Username)).
		SetDescription(description.String()).
		SetColor(0x2b2d31).
		SetFooter(fmt.Sprintf("Total Cards: %d", len(displayedCards)), "").
		Build()

	_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds: &[]discord.Embed{embed},
	})
	return err
}

func formatWishEmbed(cardInfo utils.CardDisplayInfo, card *models.Card) discord.Embed {
	var description strings.Builder
	description.WriteString("```ansi\n")

	description.WriteString(fmt.Sprintf("\x1b[32m%s\x1b[0m [%s] %s",
		cardInfo.FormattedName,
		strings.Repeat("⭐", card.Level),
		cardInfo.FormattedCollection))

	description.WriteString("\n```")

	return discord.NewEmbedBuilder().
		SetTitle("Card Added to Wishlist").
		SetDescription(description.String()).
		SetColor(0x2b2d31).
		SetFooter("Added just now", "").
		Build()
}
