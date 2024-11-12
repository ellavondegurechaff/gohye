package auction

import (
	"fmt"
	"log"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

type AuctionNotifier struct {
	client    bot.Client
	channelID snowflake.ID
}

func NewAuctionNotifier() *AuctionNotifier {
	return &AuctionNotifier{}
}

func (n *AuctionNotifier) SetClient(client bot.Client) {
	n.client = client
}

func (n *AuctionNotifier) SetChannelID(channelID snowflake.ID) {
	n.channelID = channelID
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

func (n *AuctionNotifier) NotifyEnd(auctionID int64, winnerID string, finalPrice int64) {
	var message string
	if winnerID == "" {
		message = fmt.Sprintf("[END] Auction #%d has ended with no bids", auctionID)
	} else {
		message = fmt.Sprintf("[END] Auction #%d has ended! Winner: <@%s> with %d 💰",
			auctionID, winnerID, finalPrice)
	}
	n.logNotification(message)
}

func (n *AuctionNotifier) logNotification(message string) {
	log.Printf("[AUCTION] %s", message)

	if n.client != nil && n.channelID != 1301232741697851395 {
		_, err := n.client.Rest().CreateMessage(n.channelID, discord.NewMessageCreateBuilder().
			SetContent(message).
			Build())
		if err != nil {
			log.Printf("[AUCTION] Failed to send to Discord: %v", err)
		}
	}
}
