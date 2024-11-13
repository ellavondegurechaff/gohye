package commands

import (
	"context"
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

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

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

		_, err = e.UpdateInteractionResponse(discord.MessageUpdate{
			Embeds:     &[]discord.Embed{embed},
			Components: &[]discord.ContainerComponent{components},
		})
		if err != nil {
			b.ClaimManager.ReleaseClaim(userID) // Release lock on error
		}
		return err
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
	defer b.ClaimManager.ReleaseClaim(userID) // Release lock after claim is processed

	log.Printf("[DEBUG] [CLAIM] Attempting to claim card: %s", cardIDStr)

	if b.ClaimManager == nil {
		log.Printf("[ERROR] [CLAIM] ClaimManager not initialized")
		return e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{{
				Description: "Claim system not properly initialized",
				Color:       utils.ErrorColor,
			}},
			Components: &[]discord.ContainerComponent{},
		})
	}

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
	claim, err := b.ClaimRepository.CreateClaim(ctx, cardID, userID)
	if err != nil {
		log.Printf("[ERROR] [CLAIM] Failed to create claim: %v", err)
		return e.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{{
				Description: "Failed to claim card",
				Color:       utils.ErrorColor,
			}},
			Components: &[]discord.ContainerComponent{},
		})
	}

	// After creating the claim, fetch the card details
	card, err := b.CardRepository.GetByID(ctx, claim.CardID)
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

	log.Printf("[INFO] [CLAIM] Successfully claimed card #%d for user %s", claim.CardID, userID)
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
				"%s\n"+
				"```\n"+
				"> ✨ Successfully added to your collection!",
				utils.FormatCollectionName(card.ColID),
				utils.GetStarsDisplay(card.Level),
				userID,
				utils.FormatCollectionName(card.ColID),
				utils.GetStarsDisplay(card.Level),
				card.ID,
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
