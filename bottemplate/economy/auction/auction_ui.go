package auction

import (
	"fmt"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	economicUtils "github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
)

type AuctionUI struct {
	client         bot.Client
	cardRepository repositories.CardRepository
	manager        *Manager
}

func NewAuctionUI(client bot.Client, cardRepo repositories.CardRepository, manager *Manager) *AuctionUI {
	return &AuctionUI{
		client:         client,
		cardRepository: cardRepo,
		manager:        manager,
	}
}

func (ui *AuctionUI) CreateAuctionEmbed(auction *models.Auction, card *models.Card) discord.Embed {
	builder := discord.NewEmbedBuilder().
		SetTitle(fmt.Sprintf("Auction #%d: %s", auction.ID, card.Name)).
		SetDescription(fmt.Sprintf("```md\n## Auction Details\n* Auction ID: %s\n* Seller: <@%s>\n* Current Price: %d 💰\n* Min Increment: %d 💰\n* Card Level: %s\n* Collection: %s\n```",
			auction.AuctionID,
			auction.SellerID,
			auction.CurrentPrice,
			auction.MinIncrement,
			strings.Repeat("⭐", card.Level),
			card.ColID))

	// Add time remaining
	remaining := time.Until(auction.EndTime)
	if remaining > 0 {
		builder.AddField("Time Remaining", formatDuration(remaining), true)
	} else {
		builder.AddField("Status", "Ended", true)
	}

	// Add top bidder if exists
	if auction.TopBidderID != "" {
		builder.AddField("Top Bidder", fmt.Sprintf("<@%s>", auction.TopBidderID), true)
	}

	builder.SetColor(0x2b2d31) // Discord dark theme color
	builder.SetTimestamp(time.Now())

	return builder.Build()
}

func (ui *AuctionUI) CreateAuctionComponents(auctionID int64, currentPrice int64) []discord.ContainerComponent {
	// Calculate bid increments based on current price
	increment1 := int64(economicUtils.MinBidIncrement)
	increment2 := int64(economicUtils.MinBidIncrement * 5)
	increment3 := int64(economicUtils.MinBidIncrement * 10)

	if currentPrice >= 10000 {
		increment1 *= 10
		increment2 *= 10
		increment3 *= 10
	}

	// Create action row with bid buttons
	actionRow := discord.NewActionRow(
		discord.NewPrimaryButton(
			fmt.Sprintf("Bid +%d", increment1),
			fmt.Sprintf("auction:bid:%d:%d", auctionID, increment1)),
		discord.NewPrimaryButton(
			fmt.Sprintf("Bid +%d", increment2),
			fmt.Sprintf("auction:bid:%d:%d", auctionID, increment2)),
		discord.NewPrimaryButton(
			fmt.Sprintf("Bid +%d", increment3),
			fmt.Sprintf("auction:bid:%d:%d", auctionID, increment3)),
		discord.NewDangerButton("Cancel", fmt.Sprintf("auction:cancel:%d", auctionID)),
	)

	return []discord.ContainerComponent{actionRow}
}

// Helper function to format duration in a readable format
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
