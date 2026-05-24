package economy

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

func (h *AuctionHandler) HandleConfirmation(event *handler.ComponentEvent) error {
    // Parse the custom ID
    parts := strings.Split(event.Data.CustomID(), "/")
    if len(parts) != 7 { // /auction/confirm/{ownerID}/{cardID}/{startPrice}/{duration}
        return fmt.Errorf("invalid confirmation ID format")
    }

    // Validate only the original user can confirm
    ownerID := parts[3]
    if ownerID != event.User().ID.String() {
        return utils.EH.CreateEphemeralError(event, "Only the command user can confirm this action.")
    }

    cardID, err := strconv.ParseInt(parts[4], 10, 64)
    if err != nil {
        return err
    }

    startPrice, err := strconv.ParseInt(parts[5], 10, 64)
    if err != nil {
        return err
    }

    durationSecs, err := strconv.ParseInt(parts[6], 10, 64)
    if err != nil {
        return err
    }
	duration := time.Duration(durationSecs) * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create the auction
	auction, err := h.manager.CreateAuction(ctx, cardID, event.User().ID.String(), startPrice, duration)
	if err != nil {
		return event.UpdateMessage(discord.MessageUpdate{
			Embeds: &[]discord.Embed{
				discord.NewEmbedBuilder().
					SetTitle("❌ Auction Creation Failed").
					SetDescription(fmt.Sprintf("Failed to create auction: %s", err)).
					SetColor(config.ErrorColor).
					Build(),
			},
			Components: &[]discord.ContainerComponent{},
		})
	}

	// Get card details for the success message
	card, _ := h.cardRepo.GetByID(ctx, cardID)
	cardName := "Unknown Card"
	if card != nil {
		cardName = card.Name
	}

	// Track quest progress for auction creation
	if h.bot.QuestTracker != nil {
		go h.bot.QuestTracker.TrackAuctionCreate(context.Background(), event.User().ID.String())
	}

	// Success message
	return event.UpdateMessage(discord.MessageUpdate{
		Embeds: &[]discord.Embed{
			discord.NewEmbedBuilder().
				SetTitle("✅ Auction Created").
				SetDescription(fmt.Sprintf("Successfully created auction #%s for **%s**", auction.AuctionID, cardName)).
				AddField("Start Price", fmt.Sprintf("%d 💰", startPrice), true).
				AddField("Duration", formatDuration(duration), true).
				SetColor(config.SuccessColor).
				Build(),
		},
		Components: &[]discord.ContainerComponent{},
	})
}
