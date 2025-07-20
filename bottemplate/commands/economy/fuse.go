package economy

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Fuse = discord.SlashCommandCreate{
	Name:        "fuse",
	Description: "🔮 Fuse materials to create an album card",
}

type FuseHandler struct {
	bot *bottemplate.Bot
}

func NewFuseHandler(b *bottemplate.Bot) *FuseHandler {
	return &FuseHandler{bot: b}
}

func (h *FuseHandler) Handle(e *handler.CommandEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
	defer cancel()

	userID := e.User().ID.String()

	// Get user's items
	userItems, err := h.bot.ItemRepository.GetUserItems(ctx, userID)
	if err != nil {
		return utils.EH.CreateErrorEmbed(e, "Failed to fetch your items")
	}

	// Check if user has required materials
	requirements := map[string]int{
		models.ItemBrokenDisc:    1,
		models.ItemMicrophone:    1,
		models.ItemForgottenSong: 1,
	}

	hasAll, missing := checkRequirements(userItems, requirements)

	if !hasAll {
		return h.showMissingItemsEmbed(e, userItems, missing)
	}

	// Show fusion confirmation
	return h.showFusionConfirmation(e, userItems)
}

func checkRequirements(userItems []*models.UserItem, requirements map[string]int) (bool, []string) {
	userItemMap := make(map[string]int)
	for _, ui := range userItems {
		if ui.Item != nil {
			userItemMap[ui.ItemID] = ui.Quantity
		}
	}

	var missing []string
	for itemID, required := range requirements {
		if userItemMap[itemID] < required {
			missing = append(missing, itemID)
		}
	}

	return len(missing) == 0, missing
}

func (h *FuseHandler) showMissingItemsEmbed(e *handler.CommandEvent, userItems []*models.UserItem, missing []string) error {
	itemInfo := map[string]struct {
		Name  string
		Emoji string
	}{
		models.ItemBrokenDisc:    {"Broken Disc", "💿"},
		models.ItemMicrophone:    {"Microphone", "🎤"},
		models.ItemForgottenSong: {"Forgotten Song", "📜"},
	}

	var description strings.Builder
	description.WriteString("```ansi\n")
	description.WriteString("\u001b[1;31m❌ Missing Materials\u001b[0m\n\n")
	description.WriteString("You need 1 of each material to fuse an album card:\n\n")

	// Show what user has and what's missing
	for itemID, info := range itemInfo {
		hasQuantity := 0
		for _, ui := range userItems {
			if ui.ItemID == itemID {
				hasQuantity = ui.Quantity
				break
			}
		}

		isMissing := false
		for _, missingID := range missing {
			if missingID == itemID {
				isMissing = true
				break
			}
		}

		if isMissing {
			description.WriteString(fmt.Sprintf("\u001b[1;31m✗ %s %s (%d/1)\u001b[0m\n", info.Emoji, info.Name, hasQuantity))
		} else {
			description.WriteString(fmt.Sprintf("\u001b[1;32m✓ %s %s (%d/1)\u001b[0m\n", info.Emoji, info.Name, hasQuantity))
		}
	}

	description.WriteString("\n\u001b[1;33m💡 Tip:\u001b[0m Use /work to earn materials!")
	description.WriteString("\n```")

	embed := discord.NewEmbedBuilder().
		SetTitle("🔮 Fusion Requirements Not Met").
		SetDescription(description.String()).
		SetColor(config.ErrorColor).
		Build()

	return e.CreateMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
}

func (h *FuseHandler) showFusionConfirmation(e *handler.CommandEvent, userItems []*models.UserItem) error {
	var description strings.Builder
	description.WriteString("```ansi\n")
	description.WriteString("\u001b[1;36m🔮 Album Card Fusion\u001b[0m\n\n")
	description.WriteString("You have all required materials:\n\n")
	description.WriteString("\u001b[1;32m✓ 💿 Broken Disc x1\u001b[0m\n")
	description.WriteString("\u001b[1;32m✓ 🎤 Microphone x1\u001b[0m\n")
	description.WriteString("\u001b[1;32m✓ 📜 Forgotten Song x1\u001b[0m\n\n")
	description.WriteString("\u001b[1;33mFuse these materials to receive:\u001b[0m\n")
	description.WriteString("• 1 Random Album Card (ggalbum or bgalbum)\n\n")
	description.WriteString("\u001b[1;31m⚠️ Warning:\u001b[0m Materials will be consumed!")
	description.WriteString("\n```")

	embed := discord.NewEmbedBuilder().
		SetTitle("🔮 Confirm Fusion").
		SetDescription(description.String()).
		SetColor(config.InfoColor).
		Build()

	components := []discord.ContainerComponent{
		discord.NewActionRow(
			discord.NewSuccessButton("✨ Fuse", "fuse/confirm"),
			discord.NewDangerButton("Cancel", "fuse/cancel"),
		),
	}

	return e.CreateMessage(discord.MessageCreate{
		Embeds:     []discord.Embed{embed},
		Components: components,
	})
}

func (h *FuseHandler) HandleComponent(e *handler.ComponentEvent) error {
	parts := strings.Split(e.Data.CustomID(), "/")
	if len(parts) < 2 {
		return e.UpdateMessage(discord.MessageUpdate{
			Content: utils.Ptr("❌ Invalid interaction"),
		})
	}

	action := parts[1]
	switch action {
	case "confirm":
		return h.handleFusionConfirm(e)
	case "cancel":
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("🔮 Fusion cancelled."),
			Components: &[]discord.ContainerComponent{},
		})
	default:
		return e.UpdateMessage(discord.MessageUpdate{
			Content: utils.Ptr("❌ Invalid action"),
		})
	}
}

func (h *FuseHandler) handleFusionConfirm(e *handler.ComponentEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultQueryTimeout)
	defer cancel()

	userID := e.User().ID.String()

	// Consume items
	requirements := map[string]int{
		models.ItemBrokenDisc:    1,
		models.ItemMicrophone:    1,
		models.ItemForgottenSong: 1,
	}

	err := h.bot.ItemRepository.ConsumeItems(ctx, userID, requirements)
	if err != nil {
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Failed to consume materials. You may not have enough."),
			Components: &[]discord.ContainerComponent{},
		})
	}

	// Get album collections
	var albumCard *models.Card
	albumType := "ggalbums"
	if rand.Intn(2) == 0 {
		albumType = "bgalbums"
	}

	// First check if the album collections exist
	collection, err := h.bot.CollectionRepository.GetByID(ctx, albumType)
	if err != nil || collection == nil {
		fmt.Printf("Album collection %s not found: %v\n", albumType, err)
		// Try the other album type
		if albumType == "ggalbums" {
			albumType = "bgalbums"
		} else {
			albumType = "ggalbums"
		}
		collection, err = h.bot.CollectionRepository.GetByID(ctx, albumType)
		if err != nil || collection == nil {
			// Rollback by giving items back
			for itemID, quantity := range requirements {
				h.bot.ItemRepository.AddUserItem(ctx, userID, itemID, quantity)
			}
			return e.UpdateMessage(discord.MessageUpdate{
				Content:    utils.Ptr("❌ Album collections not found in database. Please contact an admin."),
				Components: &[]discord.ContainerComponent{},
			})
		}
	}

	fmt.Printf("Found album collection: %s (%s)\n", collection.ID, collection.Name)

	// Get ALL cards and filter manually since album collections are excluded in search utils
	allCards, err := h.bot.CardRepository.GetAll(ctx)
	if err != nil {
		fmt.Printf("Error fetching all cards: %v\n", err)
		// Rollback by giving items back
		for itemID, quantity := range requirements {
			h.bot.ItemRepository.AddUserItem(ctx, userID, itemID, quantity)
		}
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Failed to fetch cards. Your materials have been returned."),
			Components: &[]discord.ContainerComponent{},
		})
	}

	// Filter cards by collection ID manually
	var cards []*models.Card
	for _, card := range allCards {
		if card.ColID == albumType {
			cards = append(cards, card)
		}
	}

	fmt.Printf("Found %d cards in %s collection after filtering\n", len(cards), albumType)

	if len(cards) == 0 {
		// Rollback by giving items back
		for itemID, quantity := range requirements {
			h.bot.ItemRepository.AddUserItem(ctx, userID, itemID, quantity)
		}
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr(fmt.Sprintf("❌ No cards found in %s collection. Album collections may be empty.", collection.Name)),
			Components: &[]discord.ContainerComponent{},
		})
	}

	// Select random card
	albumCard = cards[rand.Intn(len(cards))]

	if albumCard == nil {
		// Rollback by giving items back
		for itemID, quantity := range requirements {
			h.bot.ItemRepository.AddUserItem(ctx, userID, itemID, quantity)
		}
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Failed to select a valid album card. Your materials have been returned."),
			Components: &[]discord.ContainerComponent{},
		})
	}

	fmt.Printf("Selected card: ID=%d, Name=%s, ColID=%s\n", albumCard.ID, albumCard.Name, albumCard.ColID)

	// Grant card to user
	userCard, err := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, albumCard.ID)
	if err != nil || userCard == nil {
		// Create new user card
		now := time.Now()
		userCard = &models.UserCard{
			UserID:    userID,
			CardID:    albumCard.ID,
			Amount:    1,
			Level:     1,
			Exp:       0,
			Favorite:  false,
			Locked:    false,
			Rating:    0,
			Obtained:  now,
			CreatedAt: now,
			UpdatedAt: now,
		}
		err = h.bot.UserCardRepository.Create(ctx, userCard)
		if err != nil {
			fmt.Printf("Error creating user card: %v\n", err)
		} else {
			fmt.Printf("Successfully created user card for user %s with card ID %d\n", userID, albumCard.ID)
		}
	} else {
		// Update existing card count
		userCard.Amount++
		userCard.UpdatedAt = time.Now()
		err = h.bot.UserCardRepository.Update(ctx, userCard)
		if err != nil {
			fmt.Printf("Error updating user card: %v\n", err)
		} else {
			fmt.Printf("Successfully updated user card amount to %d for user %s with card ID %d\n", userCard.Amount, userID, albumCard.ID)
		}
	}

	if err != nil {
		// Rollback
		for itemID, quantity := range requirements {
			h.bot.ItemRepository.AddUserItem(ctx, userID, itemID, quantity)
		}
		return e.UpdateMessage(discord.MessageUpdate{
			Content:    utils.Ptr("❌ Failed to grant album card. Your materials have been returned."),
			Components: &[]discord.ContainerComponent{},
		})
	}

	// Verify the card was actually saved
	verifyCard, verifyErr := h.bot.UserCardRepository.GetByUserIDAndCardID(ctx, userID, albumCard.ID)
	if verifyErr != nil || verifyCard == nil || verifyCard.Amount == 0 {
		fmt.Printf("Verification failed - card not found after save. Error: %v, Card: %v\n", verifyErr, verifyCard)
		// Try to grant it again
		if verifyCard == nil {
			now := time.Now()
			newCard := &models.UserCard{
				UserID:    userID,
				CardID:    albumCard.ID,
				Amount:    1,
				Level:     1,
				Exp:       0,
				Favorite:  false,
				Locked:    false,
				Rating:    0,
				Obtained:  now,
				CreatedAt: now,
				UpdatedAt: now,
			}
			retryErr := h.bot.UserCardRepository.Create(ctx, newCard)
			if retryErr != nil {
				fmt.Printf("Retry failed: %v\n", retryErr)
				// Final rollback
				for itemID, quantity := range requirements {
					h.bot.ItemRepository.AddUserItem(ctx, userID, itemID, quantity)
				}
				return e.UpdateMessage(discord.MessageUpdate{
					Content:    utils.Ptr("❌ Failed to grant album card after retry. Your materials have been returned."),
					Components: &[]discord.ContainerComponent{},
				})
			}
		}
	} else {
		fmt.Printf("Verification successful - user %s now has %d of card ID %d\n", userID, verifyCard.Amount, albumCard.ID)
	}

	// Create success embed
	return h.showFusionResult(e, albumCard, albumType)
}

func (h *FuseHandler) showFusionResult(e *handler.ComponentEvent, card *models.Card, albumType string) error {
	// Get collection info
	collection, _ := h.bot.CollectionRepository.GetByID(context.Background(), albumType)
	collectionName := "Album"
	if collection != nil {
		collectionName = collection.Name
	}

	var description strings.Builder
	description.WriteString("```ansi\n")
	description.WriteString("\u001b[1;32m✨ Fusion Successful!\u001b[0m\n\n")
	description.WriteString("You received:\n\n")
	description.WriteString(fmt.Sprintf("\u001b[1;36m📀 %s\u001b[0m\n", strings.Title(strings.ReplaceAll(card.Name, "_", " "))))
	description.WriteString(fmt.Sprintf("Collection: %s\n", collectionName))
	description.WriteString(fmt.Sprintf("ID: #%d\n", card.ID))
	description.WriteString("\n\u001b[1;33m💡 Tip:\u001b[0m View your new card with /cards!")
	description.WriteString("\n```")

	embed := discord.NewEmbedBuilder().
		SetTitle("🔮 Fusion Complete!").
		SetDescription(description.String()).
		SetColor(config.SuccessColor)

	// Add card image if available
	collectionType := "girlgroups"
	if albumType == "bgalbums" {
		collectionType = "boygroups"
	}
	imageURL := h.bot.SpacesService.GetCardImageURL(card.Name, card.ColID, 1, collectionType)
	embed.SetImage(imageURL)

	embed.SetFooter("Materials consumed: 💿 x1, 🎤 x1, 📜 x1", "")

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed.Build()},
		Components: &[]discord.ContainerComponent{},
	})
}
