package models

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID              int64           `bun:"id,pk,autoincrement"`
	DiscordID       string          `bun:"discord_id,notnull,unique"`
	Username        string          `bun:"username,notnull"`
	Exp             int64           `bun:"exp,notnull,default:0"`
	PromoExp        int64           `bun:"promo_exp,notnull,default:0"`
	Joined          time.Time       `bun:"joined,notnull"`
	LastQueriedCard json.RawMessage `bun:"last_queried_card,type:jsonb"`
	LastKofiClaim   time.Time       `bun:"last_kofi_claim"`

	// Daily Stats
	DailyStats DailyStats `bun:"daily_stats,type:jsonb"`

	// Effect Use Count
	EffectUseCount EffectUseCount `bun:"effect_use_count,type:jsonb"`

	// Arrays stored as JSONB
	Cards         []string `bun:"cards,type:jsonb"`
	Inventory     []string `bun:"inventory,type:jsonb"`
	CompletedCols []string `bun:"completed_cols,type:jsonb"`
	CloutedCols   []string `bun:"clouted_cols,type:jsonb"`
	Achievements  []string `bun:"achievements,type:jsonb"`
	Effects       []string `bun:"effects,type:jsonb"`
	Wishlist      []string `bun:"wishlist,type:jsonb"`

	// Timestamps
	LastDaily    time.Time `bun:"last_daily,notnull"`
	LastTrain    time.Time `bun:"last_train,notnull"`
	LastWork     time.Time `bun:"last_work,notnull"`
	LastVote     time.Time `bun:"last_vote,notnull"`
	LastAnnounce time.Time `bun:"last_announce,notnull"`
	LastMsg      string    `bun:"last_msg"`

	// Notifications
	DailyNotified bool `bun:"daily_notified,notnull,default:true"`
	VoteNotified  bool `bun:"vote_notified,notnull,default:false"`

	// Hero related
	HeroSlots    []string  `bun:"hero_slots,type:jsonb"`
	HeroCooldown []string  `bun:"hero_cooldown,type:jsonb"`
	Hero         string    `bun:"hero"`
	HeroChanged  time.Time `bun:"hero_changed"`
	HeroSubmits  int       `bun:"hero_submits,notnull,default:0"`

	// User status
	Roles []string `bun:"roles,type:jsonb"`
	Ban   BanInfo  `bun:"ban,type:jsonb"`

	// Stats
	LastCard int64 `bun:"last_card,notnull,default:-1"`
	XP       int64 `bun:"xp,notnull,default:0"`
	Vials    int64 `bun:"vials,notnull,default:0"`
	Lemons   int64 `bun:"lemons,notnull,default:0"`
	Votes    int64 `bun:"votes,notnull,default:0"`

	// Quests
	DailyQuests []string `bun:"daily_quests,type:jsonb"`
	QuestLines  []string `bun:"quest_lines,type:jsonb"`

	// Streaks
	Streaks Streaks `bun:"streaks,type:jsonb"`

	// Preferences
	Preferences Preferences `bun:"preferences,type:jsonb"`

	// Premium
	Premium        bool      `bun:"premium,notnull,default:false"`
	PremiumExpires time.Time `bun:"premium_expires"`

	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}

type DailyStats struct {
	Claims         int `json:"claims"`
	PromoClaims    int `json:"promoclaims"`
	TotalRegClaims int `json:"totalregclaims"`
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

type EffectUseCount struct {
	MemoryXmas int  `json:"memoryxmas"`
	MemoryHall int  `json:"memoryhall"`
	MemoryBday int  `json:"memorybday"`
	MemoryVal  int  `json:"memoryval"`
	XmasSpace  bool `json:"xmasspace"`
	HallSpace  bool `json:"hallspace"`
	BdaySpace  bool `json:"bdayspace"`
	ValSpace   bool `json:"valspace"`
}

type BanInfo struct {
	Full    bool `json:"full"`
	Embargo bool `json:"embargo"`
	Report  bool `json:"report"`
	Tags    int  `json:"tags"`
}

type Streaks struct {
	Votes struct {
		TopGG int `json:"topgg"`
		DBL   int `json:"dbl"`
	} `json:"votes"`
	Daily int `json:"daily"`
	Kofi  int `json:"kofi"`
}

type Preferences struct {
	Notifications struct {
		AucBidMe  bool `json:"aucbidme"`
		AucOutBid bool `json:"aucoutbid"`
		AucNewBid bool `json:"aucnewbid"`
		AucEnd    bool `json:"aucend"`
		Announce  bool `json:"announce"`
		Daily     bool `json:"daily"`
		Vote      bool `json:"vote"`
		Completed bool `json:"completed"`
		EffectEnd bool `json:"effectend"`
	} `json:"notifications"`
	Interactions struct {
		CanHas  bool `json:"canhas"`
		CanDiff bool `json:"candiff"`
		CanSell bool `json:"cansell"`
	} `json:"interactions"`
	Profile struct {
		Bio         string `json:"bio"`
		Title       string `json:"title"`
		Color       string `json:"color"`
		Card        string `json:"card"`
		FavComplete string `json:"favcomplete"`
		FavClout    string `json:"favclout"`
		Image       string `json:"image"`
		Reputation  int    `json:"reputation"`
	} `json:"profile"`
}
