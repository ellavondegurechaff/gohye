package cards

import "github.com/disgoorg/disgo/discord"

var Commands = []discord.ApplicationCommandCreate{
	Summon,
	Draw,
	SearchCards,
	Cards,
	Claim,
	LevelUp,
	Forge,
	LimitedCards,
	LimitedStats,
	CollectionList,
	CollectionInfo,
	CollectionProgress,
}
