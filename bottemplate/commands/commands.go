package commands

import "github.com/disgoorg/disgo/discord"

var Commands = []discord.ApplicationCommandCreate{
	test,
	version,
	dbtest,
	usertest,
	usercardtest,
	MigrateCards,
	DeleteCard,
	SearchCards,
}
