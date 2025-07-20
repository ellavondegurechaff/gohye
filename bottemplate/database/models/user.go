// models/user.go
package models

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type InventoryItemModel struct {
	Time time.Time `json:"time"`
	Col  string    `json:"col"`
	ID   string    `json:"id"`
}

type CompletedColModel struct {
	ID     string `json:"id"`
	Amount int    `json:"amount,omitempty"`
}

type CloutedColModel struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID              int64     `bun:"id,pk,autoincrement"`
	DiscordID       string    `bun:"discord_id,notnull,unique"`
	Username        string    `bun:"username,notnull"`
	Balance         int64     `bun:"balance,notnull,default:0"`
	PromoExp        int64     `bun:"promo_exp,notnull,default:0"`
	Joined          time.Time `bun:"joined,notnull"`
	LastQueriedCard Card      `bun:"last_queried_card,type:jsonb"`
	LastKofiClaim   time.Time `bun:"last_kofi_claim"`

	// Stats
	DailyStats  GameStats   `bun:"daily_stats,type:jsonb"`
	EffectStats MemoryStats `bun:"effect_stats,type:jsonb"`
	UserStats   CoreStats   `bun:"user_stats,type:jsonb"`

	// Arrays stored as JSONB
	Cards         []string              `bun:"cards,type:jsonb"`
	Inventory     []InventoryItemModel  `bun:"inventory,type:jsonb"`
	CompletedCols FlexibleCompletedCols `bun:"completed_cols,type:jsonb"`
	CloutedCols   FlexibleCloutedCols   `bun:"clouted_cols,type:jsonb"`
	Achievements  []string              `bun:"achievements,type:jsonb"`
	Effects       []string              `bun:"effects,type:jsonb"`
	Wishlist      []string              `bun:"wishlist,type:jsonb"`

	// Timestamps
	LastDaily    time.Time `bun:"last_daily,notnull"`
	LastTrain    time.Time `bun:"last_train,notnull"`
	LastWork     time.Time `bun:"last_work,notnull"`
	LastVote     time.Time `bun:"last_vote,notnull"`
	LastAnnounce time.Time `bun:"last_announce,notnull"`
	LastMsg      string    `bun:"last_msg"`

	// Hero System
	HeroSlots    []string  `bun:"hero_slots,type:jsonb"`
	HeroCooldown []string  `bun:"hero_cooldown,type:jsonb"`
	Hero         string    `bun:"hero"`
	HeroChanged  time.Time `bun:"hero_changed"`
	HeroSubmits  int       `bun:"hero_submits,notnull,default:0"`

	// User Status
	Roles []string `bun:"roles,type:jsonb"`
	Ban   BanInfo  `bun:"ban,type:jsonb"`

	// Premium
	Premium        bool      `bun:"premium,notnull,default:false"`
	PremiumExpires time.Time `bun:"premium_expires"`

	// Preferences
	Preferences *Preferences `bun:"preferences,type:jsonb"`

	// Additional Fields
	Votes int64 `bun:"votes,notnull,default:0"`

	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}

type GameStats struct {
	Claims         int `json:"claims"`
	PromoClaims    int `json:"promo_claims"`
	TotalRegClaims int `json:"total_reg_claims"`
	Bids           int `json:"bids"`
	Aucs           int `json:"aucs"`
	Liquify        int `json:"liquify"`
	Liquify1       int `json:"liquify1"`
	Liquify2       int `json:"liquify2"`
	Liquify3       int `json:"liquify3"`
	Draw           int `json:"draw"`
	Draw1          int `json:"draw1"`
	Draw2          int `json:"draw2"`
	Draw3          int `json:"draw3"`
	Tags           int `json:"tags"`
	Forge1         int `json:"forge1"`
	Forge2         int `json:"forge2"`
	Forge3         int `json:"forge3"`
	Rates          int `json:"rates"`
	Store3         int `json:"store3"`
}

type MemoryStats struct {
	MemoryXmas int  `json:"memoryxmas"`
	MemoryHall int  `json:"memoryhall"`
	MemoryBday int  `json:"memorybday"`
	MemoryVal  int  `json:"memoryval"`
	XmasSpace  bool `json:"xmasspace"`
	HallSpace  bool `json:"hallspace"`
	BdaySpace  bool `json:"bdayspace"`
	ValSpace   bool `json:"valspace"`
}

type CoreStats struct {
	LastCard int64 `json:"last_card"`
	XP       int64 `json:"xp"`
	Vials    int64 `json:"vials"`
	Lemons   int64 `json:"lemons"`
	Votes    int64 `json:"votes"`
}

type BanInfo struct {
	Full    bool `json:"full"`
	Embargo bool `json:"embargo"`
	Report  bool `json:"report"`
	Tags    int  `json:"tags"`
}

type NotificationPrefs struct {
	AucBidMe  bool `json:"aucbidme"`
	AucOutBid bool `json:"aucoutbid"`
	AucNewBid bool `json:"aucnewbid"`
	AucEnd    bool `json:"aucend"`
	Announce  bool `json:"announce"`
	Daily     bool `json:"daily"`
	Vote      bool `json:"vote"`
	Completed bool `json:"completed"`
	EffectEnd bool `json:"effectend"`
}

type InteractionPrefs struct {
	CanHas  bool `json:"canhas"`
	CanDiff bool `json:"candiff"`
	CanSell bool `json:"cansell"`
}

type ProfilePrefs struct {
	Bio         string `json:"bio"`
	Title       string `json:"title"`
	Color       string `json:"color"`
	Card        string `json:"card"`
	FavComplete string `json:"favcomplete"`
	FavClout    string `json:"favclout"`
	Image       string `json:"image"`
	Reputation  int    `json:"reputation"`
}

type Streaks struct {
	Votes VoteStreaks `json:"votes"`
	Daily int         `json:"daily"`
	Kofi  int         `json:"kofi"`
}

type VoteStreaks struct {
	TopGG int `json:"topgg"`
	DBL   int `json:"dbl"`
}

// FlexibleCompletedCols is a wrapper that can handle both legacy string arrays and new object arrays
type FlexibleCompletedCols []CompletedColModel

func (f *FlexibleCompletedCols) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as the new format first
	var newFormat []CompletedColModel
	if err := json.Unmarshal(data, &newFormat); err == nil {
		*f = FlexibleCompletedCols(newFormat)
		return nil
	}

	// If that fails, try to unmarshal as legacy string array
	var legacyFormat []string
	if err := json.Unmarshal(data, &legacyFormat); err == nil {
		// Convert string array to new format
		result := make([]CompletedColModel, len(legacyFormat))
		for i, id := range legacyFormat {
			result[i] = CompletedColModel{
				ID:     id,
				Amount: 0, // Default amount for legacy data
			}
		}
		*f = FlexibleCompletedCols(result)
		return nil
	}

	// If both fail, return empty array
	*f = FlexibleCompletedCols{}
	return nil
}

// FlexibleCloutedCols is a wrapper that can handle both legacy string arrays and new object arrays
type FlexibleCloutedCols []CloutedColModel

func (f *FlexibleCloutedCols) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as the new format first
	var newFormat []CloutedColModel
	if err := json.Unmarshal(data, &newFormat); err == nil {
		*f = FlexibleCloutedCols(newFormat)
		return nil
	}

	// If that fails, try to unmarshal as legacy string array
	var legacyFormat []string
	if err := json.Unmarshal(data, &legacyFormat); err == nil {
		// Convert string array to new format
		result := make([]CloutedColModel, len(legacyFormat))
		for i, id := range legacyFormat {
			result[i] = CloutedColModel{
				ID:     id,
				Amount: 1, // Default amount for legacy data
			}
		}
		*f = FlexibleCloutedCols(result)
		return nil
	}

	// If both fail, return empty array
	*f = FlexibleCloutedCols{}
	return nil
}
