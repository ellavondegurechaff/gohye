package admin

import "github.com/disgoorg/disgo/discord"

var Commands = []discord.ApplicationCommandCreate{
	DBTest,
	DeleteCard,
	Init,
	AnalyzeEconomy,
	ManageImages,
	FixDuplicates,
	AnalyzeUsers,
	Gift,
	ResetDaily,
}
