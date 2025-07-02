package admin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/uptrace/bun"
)

var Gift = discord.SlashCommandCreate{
	Name:        "gift",
	Description: "🎁 Give balance, cards, and/or items to a user",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionUser{
			Name:        "user",
			Description: "The user to give gifts to",
			Required:    true,
		},
		discord.ApplicationCommandOptionInt{
			Name:        "balance",
			Description: "Amount of balance to add",
			Required:    false,
			MinValue:    &[]int{1}[0],
		},
		discord.ApplicationCommandOptionString{
			Name:        "card_name",
			Description: "Card name to add to user's collection (supports partial names)",
			Required:    false,
		},
		discord.ApplicationCommandOptionInt{
			Name:        "card_amount",
			Description: "Number of copies of the card to add (default: 1)",
			Required:    false,
			MinValue:    &[]int{1}[0],
			MaxValue:    &[]int{100}[0],
		},
		discord.ApplicationCommandOptionString{
			Name:        "item_name",
			Description: "Item to give (disc/microphone/song)",
			Required:    false,
		},
		discord.ApplicationCommandOptionInt{
			Name:        "item_quantity",
			Description: "Quantity of the item to give (default: 1)",
			Required:    false,
			MinValue:    &[]int{1}[0],
			MaxValue:    &[]int{999}[0],
		},
	},
}

func GiftHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		ctx := context.Background()
		
		// Get command parameters
		targetUser := e.SlashCommandInteractionData().User("user")
		
		// Get optional parameters
		var balance int64 = 0
		var cardName string = ""
		var cardAmount int64 = 1
		var itemName string = ""
		var itemQuantity int = 1
		
		if balanceOpt, ok := e.SlashCommandInteractionData().OptInt("balance"); ok {
			balance = int64(balanceOpt)
		}
		
		if cardNameOpt, ok := e.SlashCommandInteractionData().OptString("card_name"); ok {
			cardName = strings.TrimSpace(cardNameOpt)
		}
		
		if cardAmountOpt, ok := e.SlashCommandInteractionData().OptInt("card_amount"); ok {
			cardAmount = int64(cardAmountOpt)
		}
		
		if itemNameOpt, ok := e.SlashCommandInteractionData().OptString("item_name"); ok {
			itemName = strings.TrimSpace(strings.ToLower(itemNameOpt))
		}
		
		if itemQuantityOpt, ok := e.SlashCommandInteractionData().OptInt("item_quantity"); ok {
			itemQuantity = int(itemQuantityOpt)
		}
		
		// Validate at least one gift is provided
		if balance <= 0 && cardName == "" && itemName == "" {
			return e.CreateMessage(discord.MessageCreate{
				Content: "❌ You must provide either balance, card_name, or item_name (or any combination).",
			})
		}
		
		// Validate target user exists in database
		targetUserID := targetUser.ID.String()
		_, err := b.UserRepository.GetByDiscordID(ctx, targetUserID)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ User %s not found in database. They need to use a bot command first.", targetUser.Username),
			})
		}
		
		// Find card by name if card_name provided
		var card *models.Card
		if cardName != "" {
			card, err = findCardByName(ctx, b, cardName)
			if err != nil {
				return e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Card not found: %v", err),
				})
			}
		}
		
		// Find item by name if item_name provided
		var itemID string
		var itemInfo *itemDisplayInfo
		if itemName != "" {
			itemID, itemInfo, err = findItemByName(itemName)
			if err != nil {
				return e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Item not found: %v", err),
				})
			}
		}
		
		// Start transaction
		tx, err := b.DB.BunDB().BeginTx(ctx, nil)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: "❌ Failed to start database transaction.",
			})
		}
		defer tx.Rollback()
		
		var messages []string
		
		// Add balance if provided
		if balance > 0 {
			err = addBalanceToUser(ctx, tx, targetUserID, balance)
			if err != nil {
				return e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Failed to add balance: %v", err),
				})
			}
			messages = append(messages, fmt.Sprintf("💰 Added %d balance", balance))
		}
		
		// Add card if provided
		if card != nil {
			err = addCardToUser(ctx, tx, targetUserID, card.ID, cardAmount)
			if err != nil {
				return e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Failed to add card: %v", err),
				})
			}
			displayName := strings.Title(strings.ReplaceAll(card.Name, "_", " "))
			if cardAmount == 1 {
				messages = append(messages, fmt.Sprintf("🎴 Added 1x %s (ID: %d)", displayName, card.ID))
			} else {
				messages = append(messages, fmt.Sprintf("🎴 Added %dx %s (ID: %d)", cardAmount, displayName, card.ID))
			}
		}
		
		// Add item if provided
		if itemID != "" {
			err = b.ItemRepository.AddUserItem(ctx, targetUserID, itemID, itemQuantity)
			if err != nil {
				return e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Failed to add item: %v", err),
				})
			}
			if itemQuantity == 1 {
				messages = append(messages, fmt.Sprintf("%s Added 1x %s", itemInfo.emoji, itemInfo.name))
			} else {
				messages = append(messages, fmt.Sprintf("%s Added %dx %s", itemInfo.emoji, itemQuantity, itemInfo.name))
			}
		}
		
		// Commit transaction
		if err = tx.Commit(); err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: "❌ Failed to commit transaction.",
			})
		}
		
		// Create success message
		successMessage := fmt.Sprintf("🎁 **Gifts sent to %s:**\n%s", 
			targetUser.Username, 
			"• " + fmt.Sprintf("%s", messages[0]))
		
		if len(messages) > 1 {
			for i := 1; i < len(messages); i++ {
				successMessage += "\n• " + messages[i]
			}
		}
		
		return e.CreateMessage(discord.MessageCreate{
			Content: successMessage,
		})
	}
}

// addBalanceToUser adds balance to a user (following UserRepository.UpdateBalance pattern)
func addBalanceToUser(ctx context.Context, tx bun.Tx, userID string, amount int64) error {
	_, err := tx.NewUpdate().
		Model((*models.User)(nil)).
		Set("balance = balance + ?", amount).
		Set("updated_at = ?", time.Now()).
		Where("discord_id = ?", userID).
		Exec(ctx)
	return err
}

// addCardToUser adds cards to a user (following updateUserCard pattern from claim.go)
func addCardToUser(ctx context.Context, tx bun.Tx, userID string, cardID int64, amount int64) error {
	// Try to update existing user card
	result, err := tx.NewUpdate().
		Model((*models.UserCard)(nil)).
		Set("amount = amount + ?", amount).
		Set("updated_at = ?", time.Now()).
		Where("user_id = ? AND card_id = ?", userID, cardID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update user card: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	
	// If no rows affected, create new user card
	if rowsAffected == 0 {
		userCard := &models.UserCard{
			UserID:    userID,
			CardID:    cardID,
			Amount:    amount,
			Obtained:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_, err = tx.NewInsert().Model(userCard).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create user card: %w", err)
		}
	}
	
	return nil
}

// findCardByName finds a card by name using the same search method as /searchcards
func findCardByName(ctx context.Context, b *bottemplate.Bot, query string) (*models.Card, error) {
	fmt.Printf("[DEBUG] Gift findCardByName: query='%s'\n", query)
	
	// Handle empty query
	if query == "" {
		return nil, fmt.Errorf("please provide a card name")
	}

	// Use the same search method as /searchcards for consistency
	filters := repositories.SearchFilters{
		Name: query, // Set the name filter to use the complex search logic
	}
	
	// Use the main Search method that /searchcards uses
	cards, _, err := b.CardRepository.Search(ctx, filters, 0, 50) // Get first 50 results
	if err != nil {
		fmt.Printf("[DEBUG] Gift findCardByName: search error: %v\n", err)
		return nil, fmt.Errorf("search failed: %v", err)
	}
	
	if len(cards) == 0 {
		fmt.Printf("[DEBUG] Gift findCardByName: NO CARD FOUND for query='%s'\n", query)
		return nil, fmt.Errorf("no cards found matching '%s'", query)
	}

	// Return the best match (first result from ordered query)
	card := cards[0]
	fmt.Printf("[DEBUG] Gift findCardByName: FOUND card='%s' (ID=%d)\n", card.Name, card.ID)
	return card, nil
}

// itemDisplayInfo holds display information for items
type itemDisplayInfo struct {
	name  string
	emoji string
}

// findItemByName finds an item by name with fuzzy matching
func findItemByName(query string) (string, *itemDisplayInfo, error) {
	// Item mapping with various aliases
	itemMap := map[string]struct {
		id   string
		info itemDisplayInfo
	}{
		// Broken Disc
		"broken_disc":  {models.ItemBrokenDisc, itemDisplayInfo{"Broken Disc", "💿"}},
		"broken disc":  {models.ItemBrokenDisc, itemDisplayInfo{"Broken Disc", "💿"}},
		"disc":         {models.ItemBrokenDisc, itemDisplayInfo{"Broken Disc", "💿"}},
		"cd":           {models.ItemBrokenDisc, itemDisplayInfo{"Broken Disc", "💿"}},
		"brokendisc":   {models.ItemBrokenDisc, itemDisplayInfo{"Broken Disc", "💿"}},
		
		// Microphone
		"microphone":   {models.ItemMicrophone, itemDisplayInfo{"Microphone", "🎤"}},
		"mic":          {models.ItemMicrophone, itemDisplayInfo{"Microphone", "🎤"}},
		"mike":         {models.ItemMicrophone, itemDisplayInfo{"Microphone", "🎤"}},
		"micro":        {models.ItemMicrophone, itemDisplayInfo{"Microphone", "🎤"}},
		
		// Forgotten Song
		"forgotten_song": {models.ItemForgottenSong, itemDisplayInfo{"Forgotten Song", "📜"}},
		"forgotten song": {models.ItemForgottenSong, itemDisplayInfo{"Forgotten Song", "📜"}},
		"song":           {models.ItemForgottenSong, itemDisplayInfo{"Forgotten Song", "📜"}},
		"forgottensong":  {models.ItemForgottenSong, itemDisplayInfo{"Forgotten Song", "📜"}},
		"scroll":         {models.ItemForgottenSong, itemDisplayInfo{"Forgotten Song", "📜"}},
	}
	
	// Try exact match first
	if item, ok := itemMap[query]; ok {
		return item.id, &item.info, nil
	}
	
	// Try partial matching
	for key, item := range itemMap {
		if strings.Contains(key, query) || strings.Contains(query, key) {
			return item.id, &item.info, nil
		}
	}
	
	return "", nil, fmt.Errorf("item not found. Available items: disc/broken_disc, microphone/mic, song/forgotten_song")
}