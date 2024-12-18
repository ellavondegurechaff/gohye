package commands

import "github.com/disgoorg/disgo/discord"

var Commands = []discord.ApplicationCommandCreate{
	test,
	version,
	dbtest,
	DeleteCard,
	Summon,
	SearchCards,
	Init,
	Cards,
	priceStats,
	AuctionCommand,
	metrics,
	Claim,
	FixDuplicates,
	LevelUp,
	Balance,
	AnalyzeEconomy,
	ManageImages,
	Daily,
	Wish,
	Has,
	Miss,
	Diff,
	Liquefy,
	Forge,
	Work,
	Shop,
}
