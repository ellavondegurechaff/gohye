package commands

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Claim = discord.SlashCommandCreate{
	Name:        "claim",
	Description: "✨ Claim a random card from the collection!",
}

func getCardImageURL(card *models.Card, bot *bottemplate.Bot) string {
	// Default to girlgroups if no tags or invalid tag
	groupType := "girlgroups"
	if len(card.Tags) > 0 && card.Tags[0] == "boygroups" {
		groupType = "boygroups"
	}

	cardInfo := utils.GetCardDisplayInfo(
		card.Name,
		card.ColID,
		card.Level,
		groupType,
		utils.SpacesConfig{
			Bucket:   bot.SpacesService.GetBucket(),
			Region:   bot.SpacesService.GetRegion(),
			CardRoot: bot.SpacesService.GetCardRoot(),
		},
	)
	return cardInfo.ImageURL
}

func ClaimHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		userID := e.User().ID.String()

		// Add timeout context for the entire claim operation
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Check for existing session first
		if b.ClaimManager.HasActiveSession(userID) {
			return utils.EH.CreateError(e, "Error",
				"You already have an active claim session. Please finish or cancel it first.")
		}

		// Check if user can claim
		if canClaim, cooldown := b.ClaimManager.CanClaim(userID); !canClaim {
			return utils.EH.CreateError(e, "Cooldown",
				fmt.Sprintf("Please wait %s before claiming again",
					cooldown.Round(time.Second)))
		}

		// Try to acquire lock - DON'T use defer here
		if !b.ClaimManager.LockClaim(userID) {
			return utils.EH.CreateError(e, "Error",
				"Another claim is already in progress. Please wait.")
		}

		// If we fail after this point, we should release the lock
		if err := e.DeferCreateMessage(false); err != nil {
			b.ClaimManager.ReleaseClaim(userID) // Release lock on error
			return fmt.Errorf("failed to defer message: %w", err)
		}

		// Use the timeout context for database operations
		cards, err := b.CardRepository.GetAll(ctx)
		if err != nil {
			b.ClaimManager.ReleaseClaim(userID) // Release lock on error
			return utils.EH.CreateError(e, "Error", "Failed to fetch cards")
		}

		if len(cards) == 0 {
			b.ClaimManager.ReleaseClaim(userID) // Release lock on error
			return utils.EH.CreateError(e, "Error", "No cards available")
		}

		randomCard := selectRandomCard(cards)

		embed := createClaimEmbed(randomCard, b)
		components := createClaimComponents(randomCard.ID)

		// After sending the message, register the message ownership
		resp, err := e.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &[]discord.ContainerComponent{components},
		})
		if err != nil {
			b.ClaimManager.ReleaseClaim(userID)
			return err
		}

		// Register the message owner
		b.ClaimManager.RegisterMessageOwner(resp.ID.String(), userID)
		return nil
	}
}

func selectRandomCard(cards []*models.Card) *models.Card {
	weights := map[int]int{
		1: 50, // Common
		2: 25, // Uncommon
		3: 15, // Rare
		4: 7,  // Epic
		5: 3,  // Legendary
	}

	totalWeight := 0
	cardsByRarity := make(map[int][]*models.Card)

	for _, card := range cards {
		totalWeight += weights[card.Level]
		cardsByRarity[card.Level] = append(cardsByRarity[card.Level], card)
	}

	roll := rand.Intn(totalWeight)
	currentWeight := 0

	for rarity := 1; rarity <= 5; rarity++ {
		currentWeight += weights[rarity]
		if roll < currentWeight && len(cardsByRarity[rarity]) > 0 {
			cards := cardsByRarity[rarity]
			return cards[rand.Intn(len(cards))]
		}
	}

	// Fallback to random card if something goes wrong
	return cards[rand.Intn(len(cards))]
}

func createClaimComponents(cardID int64) discord.ContainerComponent {
	return discord.NewActionRow(
		discord.NewPrimaryButton("✨ Claim", fmt.Sprintf("/claim/%d", cardID)),
		discord.NewSecondaryButton("🎲 Reroll", "/claim/reroll"),
		discord.NewDangerButton("❌ Cancel", "/claim/cancel"),
	)
}

func createClaimEmbed(card *models.Card, b *bottemplate.Bot) discord.Embed {
	return discord.NewEmbedBuilder().
		SetTitle(utils.FormatCardName(card.Name)).
		SetDescription(fmt.Sprintf("```md\n"+
			"# Card Information\n"+
			"* Collection: %s\n"+
			"* Level: %s\n"+
			"* ID: #%d\n"+
			"%s\n"+
			"```\n"+
			"> Quick! Claim this card before someone else does!",
			utils.FormatCollectionName(card.ColID),
			utils.GetStarsDisplay(card.Level),
			card.ID,
			utils.GetAnimatedTag(card.Animated))).
		SetColor(utils.GetColorByLevel(card.Level)).
		SetImage(getCardImageURL(card, b)).
		Build()
}

func handleReroll(e *handler.ComponentEvent, b *bottemplate.Bot) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cards, err := b.CardRepository.GetAll(ctx)
	if err != nil {
		return e.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{{
				Title:       "❌ Error",
				Description: "Failed to fetch cards",
				Color:       utils.ErrorColor,
			}},
		})
	}

	randomCard := selectRandomCard(cards)
	embed := createClaimEmbed(randomCard, b)
	components := createClaimComponents(randomCard.ID)

	return e.UpdateMessage(discord.MessageUpdate{
		Embeds:     &[]discord.Embed{embed},
		Components: &[]discord.ContainerComponent{components},
	})
}

func handleCancel(e *handler.ComponentEvent, b *bottemplate.Bot) error {
	userID := e.User().ID.String()
	defer b.ClaimManager.ReleaseClaim(userID) // Release lock when cancelled

	return e.UpdateMessage(discord.MessageUpdate{
		Components: &[]discord.ContainerComponent{
			discord.NewActionRow(
				discord.NewPrimaryButton("✨ Claim", "claimed").WithDisabled(true),
				discord.NewSecondaryButton("🎲 Reroll", "rerolled").WithDisabled(true),
				discord.NewDangerButton("❌ Cancel", "cancelled").WithDisabled(true),
			),
		},
		Embeds: &[]discord.Embed{{
			Description: "Claim cancelled",
			Color:       utils.ErrorColor,
		}},
	})
}

func ClaimButtonHandler(b *bottemplate.Bot) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		data, ok := e.Data.(discord.ButtonInteractionData)
		if !ok {
			return fmt.Errorf("invalid interaction type")
		}

		userID := e.User().ID.String()
		messageID := e.Message.ID.String()

		// Check if this user owns this specific claim message
		if !b.ClaimManager.IsMessageOwner(messageID, userID) {
			return e.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Description: "This claim session belongs to another user.",
					Color:       utils.ErrorColor,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}

		// Don't try to acquire a new lock for button interactions
		// Instead, verify the user has an active session
		if !b.ClaimManager.HasActiveSession(userID) {
			return e.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Description: "No active claim session found. Please start a new claim.",
					Color:       utils.ErrorColor,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}

		action := strings.TrimPrefix(data.CustomID(), "/claim/")
		var err error
		switch action {
		case "reroll":
			err = handleReroll(e, b)
		case "cancel":
			err = handleCancel(e, b) // Pass bot instance to access ClaimManager
		default:
			err = handleClaim(e, b, action)
		}

		if err != nil {
			// Release lock on error
			b.ClaimManager.ReleaseClaim(userID)
			return e.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Description: fmt.Sprintf("Error processing claim: %v", err),
					Color:       utils.ErrorColor,
				}},
				Flags: discord.MessageFlagEphemeral,
			})
		}
		return nil
	}
}

func handleClaim(e *handler.ComponentEvent, b *bottemplate.Bot, cardIDStr string) error {
	userID := e.User().ID.String()
	defer b.ClaimManager.ReleaseClaim(userID)

	log.Printf("[DEBUG] [CLAIM] Attempting to claim card: %s", cardIDStr)

	cardID, err := strconv.ParseInt(cardIDStr, 10, 64)
	if err != nil {
		log.Printf("[ERROR] [CLAIM] Invalid card ID format: %v", err)
		return e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{{
				Description: "Invalid card ID",
				Color:       utils.ErrorColor,
			}},
			Components: &[]discord.ContainerComponent{},
		})
	}

	if canClaim, cooldown := b.ClaimManager.CanClaim(userID); !canClaim {
		log.Printf("[DEBUG] [CLAIM] User %s on cooldown for %s", userID, cooldown)
		return e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{{
				Description: fmt.Sprintf("Please wait %s before claiming again", cooldown.Round(time.Second)),
				Color:       utils.ErrorColor,
			}},
			Components: &[]discord.ContainerComponent{},
		})
	}

	ctx := context.Background()

	// First, check if user already has this card
	userCard, err := b.UserCardRepository.GetUserCard(ctx, userID, cardID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("[ERROR] [CLAIM] Failed to check existing card: %v", err)
		return e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{{
				Description: "Failed to process claim",
				Color:       utils.ErrorColor,
			}},
			Components: &[]discord.ContainerComponent{},
		})
	}

	// Start transaction
	tx, err := b.DB.BunDB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var amount int64
	if userCard != nil {
		// Update existing card
		amount = userCard.Amount + 1
		_, err = tx.NewUpdate().
			Model((*models.UserCard)(nil)).
			Set("amount = ?", amount).
			Set("updated_at = ?", time.Now()).
			Where("user_id = ? AND card_id = ?", userID, cardID).
			Exec(ctx)
	} else {
		// Create new card entry
		amount = 1
		userCard = &models.UserCard{
			UserID:    userID,
			CardID:    cardID,
			Amount:    1,
			Obtained:  time.Now(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_, err = tx.NewInsert().Model(userCard).Exec(ctx)
	}

	if err != nil {
		log.Printf("[ERROR] [CLAIM] Failed to update user card: %v", err)
		return e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{{
				Description: "Failed to process claim",
				Color:       utils.ErrorColor,
			}},
			Components: &[]discord.ContainerComponent{},
		})
	}

	// Create claim record
	claim := &models.Claim{
		CardID:    cardID,
		UserID:    userID,
		ClaimedAt: time.Now(),
		Expires:   time.Now().Add(24 * time.Hour),
	}

	_, err = tx.NewInsert().Model(claim).Exec(ctx)
	if err != nil {
		log.Printf("[ERROR] [CLAIM] Failed to create claim record: %v", err)
		return e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{{
				Description: "Failed to process claim",
				Color:       utils.ErrorColor,
			}},
			Components: &[]discord.ContainerComponent{},
		})
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[ERROR] [CLAIM] Failed to commit transaction: %v", err)
		return e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{{
				Description: "Failed to process claim",
				Color:       utils.ErrorColor,
			}},
			Components: &[]discord.ContainerComponent{},
		})
	}

	// After successful claim, fetch the card details
	card, err := b.CardRepository.GetByID(ctx, cardID)
	if err != nil {
		log.Printf("[ERROR] [CLAIM] Failed to fetch card details: %v", err)
		return e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{{
				Description: "Failed to fetch card details",
				Color:       utils.ErrorColor,
			}},
			Components: &[]discord.ContainerComponent{},
		})
	}

	log.Printf("[INFO] [CLAIM] Successfully claimed card #%d for user %s (Total: %d)", cardID, userID, amount)
	b.ClaimManager.SetClaimCooldown(userID)

	// Update the original message to show it's been claimed
	timestamp := fmt.Sprintf("<t:%d:R>", time.Now().Unix())
	return e.UpdateMessage(discord.MessageUpdate{
		Embeds: &[]discord.Embed{{
			Title: utils.FormatCardName(card.Name),
			Description: fmt.Sprintf("``%s`` ``%s`` has been claimed by <@%s>!\n\n"+
				"```md\n"+
				"# Card Information\n"+
				"* Collection: %s\n"+
				"* Level: %s\n"+
				"* ID: #%d\n"+
				"* Amount: %dx\n"+ // Added amount display
				"%s\n"+
				"```\n"+
				"> ✨ Successfully added to your collection!",
				utils.FormatCollectionName(card.ColID),
				utils.GetStarsDisplay(card.Level),
				userID,
				utils.FormatCollectionName(card.ColID),
				utils.GetStarsDisplay(card.Level),
				card.ID,
				amount,
				utils.GetAnimatedTag(card.Animated)),
			Color: utils.SuccessColor,
			Image: &discord.EmbedResource{
				URL: getCardImageURL(card, b),
			},
			Footer: &discord.EmbedFooter{
				Text:    fmt.Sprintf("Claimed by %s • %s", e.User().Username, timestamp),
				IconURL: e.User().EffectiveAvatarURL(),
			},
		}},
		Components: &[]discord.ContainerComponent{
			discord.NewActionRow(
				discord.NewPrimaryButton("✨ Claim", fmt.Sprintf("/claim/%d", cardID)).WithDisabled(true),
				discord.NewSecondaryButton("🎲 Reroll", "/claim/reroll").WithDisabled(true),
				discord.NewDangerButton("❌ Cancel", "/claim/cancel").WithDisabled(true),
			),
		},
	})
}
