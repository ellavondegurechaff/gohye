// types.go
package migration

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// InventoryItem represents an item in the user's inventory
type InventoryItem struct {
	Time time.Time `bson:"time"`
	Col  string    `bson:"col"`
	ID   string    `bson:"id"`
}

// ColInfo represents collection info in completedcols and cloutedcols
type ColInfo struct {
	ID     string `bson:"id"`
	Amount int32  `bson:"amount,omitempty"`
}

// LastQueriedCard represents the last card queried by the user.
type LastQueriedCard struct {
	ID       string   `bson:"id"`
	Name     string   `bson:"name"`
	Level    float64  `bson:"level"` // Changed to float64
	Animated bool     `bson:"animated"`
	ColID    string   `bson:"col_id"`
	Tags     []string `bson:"tags"`
}

// DailyStats represents the daily statistics of the user.
type DailyStats struct {
	Claims         float64 `bson:"claims"`
	PromoClaims    float64 `bson:"promoclaims"`
	TotalRegClaims float64 `bson:"totalregclaims"`
	Bids           float64 `bson:"bids"`
	Aucs           float64 `bson:"aucs"`
	Liquify        float64 `bson:"liquify"`
	Liquify1       float64 `bson:"liquify1"`
	Liquify2       float64 `bson:"liquify2"`
	Liquify3       float64 `bson:"liquify3"`
	Draw           float64 `bson:"draw"`
	Draw1          float64 `bson:"draw1"`
	Draw2          float64 `bson:"draw2"`
	Draw3          float64 `bson:"draw3"`
	Tags           float64 `bson:"tags"`
	Forge1         float64 `bson:"forge1"`
	Forge2         float64 `bson:"forge2"`
	Forge3         float64 `bson:"forge3"`
	Rates          float64 `bson:"rates"`
	Store3         float64 `bson:"store3"`
}

// EffectUseCount represents the usage count of various effects.
type EffectUseCount struct {
	MemoryXmas float64 `bson:"memoryxmas"`
	MemoryHall float64 `bson:"memoryhall"`
	MemoryBday float64 `bson:"memorybday"`
	MemoryVal  float64 `bson:"memoryval"`
	XmasSpace  bool    `bson:"xmasspace"`
	HallSpace  bool    `bson:"hallspace"`
	BdaySpace  bool    `bson:"bdayspace"`
	ValSpace   bool    `bson:"valspace"`
}

// BanInfo represents the ban information of the user.
type BanInfo struct {
	Full    bool    `bson:"full"`
	Embargo bool    `bson:"embargo"`
	Report  bool    `bson:"report"`
	Tags    float64 `bson:"tags"` // Changed to float64
}

// Streaks represents various streaks the user has.
type Streaks struct {
	Votes struct {
		TopGG float64 `bson:"topgg"`
		DBL   float64 `bson:"dbl"`
	} `bson:"votes"`
	Daily float64 `bson:"daily"`
	Kofi  float64 `bson:"kofi"`
}

// Preferences represents the user's preferences.
type Preferences struct {
	Notifications struct {
		AucBidMe  bool `bson:"aucbidme"`
		AucOutBid bool `bson:"aucoutbid"`
		AucNewBid bool `bson:"aucnewbid"`
		AucEnd    bool `bson:"aucend"`
		Announce  bool `bson:"announce"`
		Daily     bool `bson:"daily"`
		Vote      bool `bson:"vote"`
		Completed bool `bson:"completed"`
		EffectEnd bool `bson:"effectend"`
	} `bson:"notifications"`
	Interactions struct {
		CanHas  bool `bson:"canhas"`
		CanDiff bool `bson:"candiff"`
		CanSell bool `bson:"cansell"`
	} `bson:"interactions"`
	Profile struct {
		Bio         string  `bson:"bio"`
		Title       string  `bson:"title"`
		Color       string  `bson:"color"`
		Card        string  `bson:"card"`
		FavComplete string  `bson:"favcomplete"`
		FavClout    string  `bson:"favclout"`
		Image       string  `bson:"image"`
		Reputation  float64 `bson:"reputation"` // Changed to float64
	} `bson:"profile"`
}

// MongoUser represents a user in MongoDB.
type MongoUser struct {
	ID              primitive.ObjectID `bson:"_id"`
	DiscordID       string             `bson:"discord_id"`
	Username        string             `bson:"username"`
	Exp             float64            `bson:"exp"`
	PromoExp        float64            `bson:"promoexp"`
	Joined          time.Time          `bson:"joined"`
	LastQueriedCard LastQueriedCard    `bson:"lastQueriedCard"`
	LastKofiClaim   time.Time          `bson:"lastkoficlaim"`
	DailyStats      DailyStats         `bson:"dailystats"`
	EffectUseCount  EffectUseCount     `bson:"effectusecount"`
	Cards           []string           `bson:"cards"`
	Inventory       []interface{}      `bson:"inventory"`     // Changed to []interface{}
	CompletedCols   []ColInfo          `bson:"completedcols"` // Updated type
	CloutedCols     []ColInfo          `bson:"cloutedcols"`   // Updated type
	Achievements    []string           `bson:"achievements"`
	Effects         []string           `bson:"effects"`
	Wishlist        []int32            `bson:"wishlist"` // Updated type
	LastDaily       time.Time          `bson:"lastdaily"`
	LastTrain       time.Time          `bson:"lasttrain"`
	LastWork        time.Time          `bson:"lastwork"`
	LastVote        time.Time          `bson:"lastvote"`
	LastAnnounce    time.Time          `bson:"lastannounce"`
	LastMsg         string             `bson:"lastmsg"`
	DailyNotified   bool               `bson:"dailynotified"`
	VoteNotified    bool               `bson:"votenotified"`
	HeroSlots       []string           `bson:"heroslots"`
	HeroCooldown    []string           `bson:"herocooldown"`
	Hero            string             `bson:"hero"`
	HeroChanged     time.Time          `bson:"herochanged"`
	HeroSubmits     int32              `bson:"herosubmits"` // Changed to int32
	Roles           []string           `bson:"roles"`
	Ban             BanInfo            `bson:"ban"`
	LastCard        int32              `bson:"lastcard"` // Changed to int32
	XP              float64            `bson:"xp"`
	Vials           float64            `bson:"vials"`  // Use float64 to handle double and int
	Lemons          float64            `bson:"lemons"` // Use float64
	Votes           int32              `bson:"votes"`  // Changed to int32
	DailyQuests     []string           `bson:"dailyquests"`
	QuestLines      []string           `bson:"questlines"`
	Streaks         Streaks            `bson:"streaks"`
	Prefs           Preferences        `bson:"prefs"`
	Premium         bool               `bson:"premium"`
	PremiumExpires  time.Time          `bson:"premiumExpires"`
	UpdatedAt       time.Time          `bson:"updated_at"`
}

// MongoUserCard represents a user's card in MongoDB.
type MongoUserCard struct {
	ID        primitive.ObjectID `bson:"_id"`
	UserID    string             `bson:"userid"`
	CardID    *int32             `bson:"cardid"` // Changed to *int32 to handle nulls
	Fav       bool               `bson:"fav"`
	Locked    bool               `bson:"locked"`
	Amount    int32              `bson:"amount"` // Changed to int32
	Rating    int32              `bson:"rating"` // Changed to int32
	Obtained  time.Time          `bson:"obtained"`
	Exp       float64            `bson:"exp"` // Use float64
	Mark      string             `bson:"mark"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

// MongoCollection represents a collection in MongoDB.
type MongoCollection struct {
	ID         primitive.ObjectID `bson:"_id"`
	ColID      string             `bson:"id"`
	Name       string             `bson:"name"`
	Origin     string             `bson:"origin"`
	Aliases    []string           `bson:"aliases"`
	Compressed bool               `bson:"compressed"`
	Promo      bool               `bson:"promo"`
	Tags       []string           `bson:"tags"`
	Fragments  bool               `bson:"fragments"`
	Added      time.Time          `bson:"added"`
	UpdatedAt  time.Time          `bson:"updatedAt"`
}

// MongoCard represents a card in MongoDB.
type MongoCard struct {
	ID        primitive.ObjectID `bson:"_id"`
	CardID    int32              `bson:"id"`
	Name      string             `bson:"name"`
	Level     int32              `bson:"level"`
	ColID     string             `bson:"col"`
	Animated  bool               `bson:"animated"`
	Tags      string             `bson:"tags"` // Tags as string in MongoDB
	Added     time.Time          `bson:"added"`
	URL       string             `bson:"url"`
	ShortURL  string             `bson:"shorturl"`
	UpdatedAt time.Time          `bson:"updatedAt"`
}

// MongoClaim represents a claim in MongoDB.
type MongoClaim struct {
	ID      primitive.ObjectID `bson:"_id"`
	ClaimID string             `bson:"id"`
	Cards   []int32            `bson:"cards"` // Array of card IDs
	Promo   bool               `bson:"promo"`
	Lock    string             `bson:"lock"`
	Date    time.Time          `bson:"date"`
	User    string             `bson:"user"`
	Guild   string             `bson:"guild"`
	Cost    int32              `bson:"cost"`
}

// MongoAuction represents an auction in MongoDB.
type MongoAuction struct {
	ID         primitive.ObjectID `bson:"_id"`
	AuctionID  string             `bson:"id"`
	Finished   bool               `bson:"finished"`
	Cancelled  bool               `bson:"cancelled"`
	Price      int64              `bson:"price"`
	HighBid    int64              `bson:"highbid"`
	Card       int32              `bson:"card"`
	Bids       []MongoAuctionBid  `bson:"bids"`
	Author     string             `bson:"author"`
	Expires    time.Time          `bson:"expires"`
	Time       time.Time          `bson:"time"`
	Guild      string             `bson:"guild"`
	LastBidder string             `bson:"lastbidder"`
}

// MongoAuctionBid represents a bid within an auction.
type MongoAuctionBid struct {
	User string    `bson:"user"`
	Bid  int64     `bson:"bid"`
	Time time.Time `bson:"time"`
}

// MongoUserEffect represents a user effect in MongoDB.
type MongoUserEffect struct {
	ID           primitive.ObjectID `bson:"_id"`
	UserID       string             `bson:"userid"`
	EffectID     string             `bson:"id"`
	Uses         int32              `bson:"uses"`
	CooldownEnds time.Time          `bson:"cooldownends"`
	Expires      time.Time          `bson:"expires"`
	Notified     bool               `bson:"notified"`
}

// MongoUserQuest represents a user quest in MongoDB.
type MongoUserQuest struct {
	ID        primitive.ObjectID `bson:"_id"`
	UserID    string             `bson:"userid"`
	QuestID   string             `bson:"questid"`
	Type      string             `bson:"type"`
	Completed bool               `bson:"completed"`
	Created   time.Time          `bson:"created"`
	Expiry    time.Time          `bson:"expiry"`
}

// MongoUserInventory represents a user inventory item in MongoDB.
type MongoUserInventory struct {
	ID       primitive.ObjectID `bson:"_id"`
	Cards    []int32            `bson:"cards"`
	UserID   string             `bson:"userid"`
	ItemID   string             `bson:"id"`
	Acquired time.Time          `bson:"acquired"`
}

// MigrationStats tracks migration progress and issues
type MigrationStats struct {
	Tables map[string]*TableStats `json:"tables"`
	StartTime time.Time `json:"start_time"`
	EndTime time.Time `json:"end_time"`
	TotalErrors int `json:"total_errors"`
	TotalSkipped int `json:"total_skipped"`
	TotalProcessed int `json:"total_processed"`
}

// TableStats tracks stats for individual tables
type TableStats struct {
	TableName string `json:"table_name"`
	Processed int `json:"processed"`
	Successful int `json:"successful"`
	Skipped int `json:"skipped"`
	Errors int `json:"errors"`
	SkippedRecords []SkippedRecord `json:"skipped_records"`
	ErrorRecords []ErrorRecord `json:"error_records"`
}

// SkippedRecord tracks why a record was skipped
type SkippedRecord struct {
	Reason string `json:"reason"`
	Data string `json:"data"` // JSON representation of the record
	Timestamp time.Time `json:"timestamp"`
}

// ErrorRecord tracks migration errors
type ErrorRecord struct {
	Error string `json:"error"`
	Data string `json:"data"` // JSON representation of the record
	Timestamp time.Time `json:"timestamp"`
}
