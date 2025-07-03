package auction

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

type AuctionNotifier struct {
	client      bot.Client
	channelID   snowflake.ID
	mu          sync.RWMutex
	initialized bool
}

func NewAuctionNotifier(client bot.Client) *AuctionNotifier {
	return &AuctionNotifier{
		client:      client,
		channelID:   snowflake.ID(1301232741697851395),
		initialized: true,
	}
}

func (n *AuctionNotifier) SetClient(client bot.Client) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.client = client
	n.initialized = true
}

func (n *AuctionNotifier) NotifyBid(auctionID int64, bidderID string, amount int64) {
	message := fmt.Sprintf("[BID] <@%s> placed a bid of %d 💰 on Auction #%d", bidderID, amount, auctionID)
	n.logNotification(message)
}

func (n *AuctionNotifier) NotifyOutbid(auctionID int64, outbidUserID string, newBidderID string, amount int64) {
	message := fmt.Sprintf("[OUTBID] User %s was outbid on Auction #%d by <@%s> with %d 💰",
		outbidUserID, auctionID, newBidderID, amount)
	n.logNotification(message)
}

func (n *AuctionNotifier) NotifyAuctionEnd(ctx context.Context, auction *models.Auction, card *models.Card) error {
	n.mu.RLock()
	if !n.initialized || n.client == nil {
		n.mu.RUnlock()
		return fmt.Errorf("auction notifier not properly initialized: initialized=%v, client=%v",
			n.initialized, n.client != nil)
	}
	client := n.client
	n.mu.RUnlock()

	// Format card name for display
	cardName := card.Name
	if card.Level >= 1 && card.Level <= 5 {
		// Add star rating to card name
		stars := ""
		for i := 0; i < card.Level; i++ {
			stars += "★"
		}
		cardName = fmt.Sprintf("%s %s [%s]", stars, card.Name, card.ColID)
	}

	// Create DM for seller
	sellerEmbed := discord.NewEmbedBuilder().
		SetTitle("🏛️ Auction Completed").
		SetColor(0x2b2d31)

	if auction.TopBidderID != "" {
		sellerEmbed.SetDescription(fmt.Sprintf("Your auction for **%s** has ended with a final price of %d flakes!",
			cardName,
			auction.CurrentPrice))
	} else {
		sellerEmbed.SetDescription(fmt.Sprintf("Your auction for **%s** has ended with no bids. The card has been returned to your inventory.",
			cardName))
	}

	// Try to DM the seller
	dmChannel, err := client.Rest().CreateDMChannel(snowflake.MustParse(auction.SellerID))
	if err != nil {
		slog.Error("Failed to create DM channel with seller",
			slog.String("seller_id", auction.SellerID),
			slog.String("error", err.Error()))
		return err
	}

	_, err = client.Rest().CreateMessage(dmChannel.ID(), discord.MessageCreate{
		Embeds: []discord.Embed{sellerEmbed.Build()},
	})
	if err != nil {
		slog.Error("Failed to send message to seller",
			slog.String("seller_id", auction.SellerID),
			slog.String("error", err.Error()))
	}

	// If there's a winner, notify them too
	if auction.TopBidderID != "" {
		winnerEmbed := discord.NewEmbedBuilder().
			SetTitle("🏛️ Auction Won!").
			SetDescription(fmt.Sprintf("You won the auction for **%s** with a final price of %d flakes!",
				cardName,
				auction.CurrentPrice)).
			SetColor(0x2b2d31)

		winnerDMChannel, err := client.Rest().CreateDMChannel(snowflake.MustParse(auction.TopBidderID))
		if err != nil {
			slog.Error("Failed to create DM channel with winner",
				slog.String("winner_id", auction.TopBidderID),
				slog.String("error", err.Error()))
			return err
		}

		_, err = client.Rest().CreateMessage(winnerDMChannel.ID(), discord.MessageCreate{
			Embeds: []discord.Embed{winnerEmbed.Build()},
		})
		if err != nil {
			slog.Error("Failed to send message to winner",
				slog.String("winner_id", auction.TopBidderID),
				slog.String("error", err.Error()))
		}
	}

	return nil
}

func (n *AuctionNotifier) logNotification(message string) {
	slog.Info(message)

	if n.client != nil {
		_, err := n.client.Rest().CreateMessage(n.channelID, discord.NewMessageCreateBuilder().
			SetContent(message).
			Build())
		if err != nil {
			slog.Error("Failed to send to Discord",
				slog.String("error", err.Error()))
		}
	}
}
