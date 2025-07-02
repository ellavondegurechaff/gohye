package economy

import "github.com/disgoorg/disgo/discord"

var Commands = []discord.ApplicationCommandCreate{
	Balance,
	Daily,
	Work,
	Shop,
	Liquefy,
	AuctionCommand,
	PriceStats,
	Fuse,
}