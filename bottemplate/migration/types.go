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
