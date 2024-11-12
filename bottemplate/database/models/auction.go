package models

import (
	"time"

	"github.com/uptrace/bun"
)

type AuctionStatus string

const (
	AuctionStatusActive    AuctionStatus = "active"
	AuctionStatusCompleted AuctionStatus = "completed"
	AuctionStatusCancelled AuctionStatus = "cancelled"
)

type Auction struct {
	bun.BaseModel `bun:"table:auctions,alias:a"`

	ID           int64         `bun:"id,pk,autoincrement"`
	AuctionID    string        `bun:"auction_id,notnull,unique"`
	CardID       int64         `bun:"card_id,notnull"`
	SellerID     string        `bun:"seller_id,notnull"`
	StartPrice   int64         `bun:"start_price,notnull"`
	CurrentPrice int64         `bun:"current_price,notnull"`
	MinIncrement int64         `bun:"min_increment,notnull"`
	TopBidderID  string        `bun:"top_bidder_id"`
	Status       AuctionStatus `bun:"status,notnull"`
	StartTime    time.Time     `bun:"start_time,notnull"`
	EndTime      time.Time     `bun:"end_time,notnull"`
	MessageID    string        `bun:"message_id"`
	ChannelID    string        `bun:"channel_id"`

	// Anti-manipulation fields
	LastBidTime time.Time `bun:"last_bid_time"`
	BidCount    int       `bun:"bid_count"`

	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull,default:current_timestamp"`
}

type AuctionBid struct {
	bun.BaseModel `bun:"table:auction_bids,alias:ab"`

	ID        int64     `bun:"id,pk,autoincrement"`
	AuctionID int64     `bun:"auction_id,notnull"`
	BidderID  string    `bun:"bidder_id,notnull"`
	Amount    int64     `bun:"amount,notnull"`
	Timestamp time.Time `bun:"timestamp,notnull"`

	CreatedAt time.Time `bun:"created_at,notnull"`
}
