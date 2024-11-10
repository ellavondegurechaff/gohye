package commands

import "github.com/disgoorg/disgo/discord"

var Commands = []discord.ApplicationCommandCreate{
	test,
	version,
	dbtest,
	MigrateCards,
	DeleteCard,
	SearchCards,
	AnalyzeUsers,
	Init,
}
