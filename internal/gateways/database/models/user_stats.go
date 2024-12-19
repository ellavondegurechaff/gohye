package models

import (
	"time"

	"github.com/uptrace/bun"
)

type UserStats struct {
	bun.BaseModel `bun:"table:user_stats,alias:ust"`

	ID        int64     `bun:"id,pk,autoincrement"`
	DiscordID string    `bun:"discord_id,notnull,unique"`
	Username  string    `bun:"username,notnull"`
	LastDaily time.Time `bun:"last_daily"`

	// Core Stats
	Claims         int64 `bun:"claims,notnull,default:0"`
	PromoClaims    int64 `bun:"promo_claims,notnull,default:0"`
	TotalRegClaims int64 `bun:"total_reg_claims,notnull,default:0"`
	Train          int64 `bun:"train,notnull,default:0"`
	Work           int64 `bun:"work,notnull,default:0"`

	// Auction Stats
	AucSell int64 `bun:"auc_sell,notnull,default:0"`
	AucBid  int64 `bun:"auc_bid,notnull,default:0"`
	AucWin  int64 `bun:"auc_win,notnull,default:0"`

	// Card Actions
	Liquefy  int64 `bun:"liquefy,notnull,default:0"`
	Liquefy1 int64 `bun:"liquefy1,notnull,default:0"`
	Liquefy2 int64 `bun:"liquefy2,notnull,default:0"`
	Liquefy3 int64 `bun:"liquefy3,notnull,default:0"`
	Draw     int64 `bun:"draw,notnull,default:0"`
	Draw1    int64 `bun:"draw1,notnull,default:0"`
	Draw2    int64 `bun:"draw2,notnull,default:0"`
	Draw3    int64 `bun:"draw3,notnull,default:0"`

	// Forge Stats
	Forge   int64 `bun:"forge,notnull,default:0"`
	Forge1  int64 `bun:"forge1,notnull,default:0"`
	Forge2  int64 `bun:"forge2,notnull,default:0"`
	Forge3  int64 `bun:"forge3,notnull,default:0"`
	Combine int64 `bun:"combine,notnull,default:0"`
	Ascend  int64 `bun:"ascend,notnull,default:0"`
	Ascend2 int64 `bun:"ascend2,notnull,default:0"`
	Ascend3 int64 `bun:"ascend3,notnull,default:0"`

	// Misc Stats
	Tags  int64 `bun:"tags,notnull,default:0"`
	Rates int64 `bun:"rates,notnull,default:0"`
	Wish  int64 `bun:"wish,notnull,default:0"`

	// Transaction Stats
	UserSell  int64 `bun:"user_sell,notnull,default:0"`
	BotSell   int64 `bun:"bot_sell,notnull,default:0"`
	UserBuy   int64 `bun:"user_buy,notnull,default:0"`
	TomatoIn  int64 `bun:"tomato_in,notnull,default:0"`
	TomatoOut int64 `bun:"tomato_out,notnull,default:0"`
	PromoIn   int64 `bun:"promo_in,notnull,default:0"`
	PromoOut  int64 `bun:"promo_out,notnull,default:0"`
	VialIn    int64 `bun:"vial_in,notnull,default:0"`
	VialOut   int64 `bun:"vial_out,notnull,default:0"`
	LemonIn   int64 `bun:"lemon_in,notnull,default:0"`
	LemonOut  int64 `bun:"lemon_out,notnull,default:0"`

	// Store Stats
	Store  int64 `bun:"store,notnull,default:0"`
	Store1 int64 `bun:"store1,notnull,default:0"`
	Store2 int64 `bun:"store2,notnull,default:0"`
	Store3 int64 `bun:"store3,notnull,default:0"`
	Store4 int64 `bun:"store4,notnull,default:0"`

	// Quest Stats
	T1Quests int64 `bun:"t1_quests,notnull,default:0"`
	T2Quests int64 `bun:"t2_quests,notnull,default:0"`
	T3Quests int64 `bun:"t3_quests,notnull,default:0"`
	T4Quests int64 `bun:"t4_quests,notnull,default:0"`
	T5Quests int64 `bun:"t5_quests,notnull,default:0"`
	T6Quests int64 `bun:"t6_quests,notnull,default:0"`

	CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}
