package commands

import (
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
)

var Cards = discord.SlashCommandCreate{
	Name:        "cards",
	Description: "View your card collection",
	Options: append(utils.CommonFilterOptions,
		discord.ApplicationCommandOptionBool{
			Name:        "favorites",
			Description: "Show only favorite cards",
			Required:    false,
		},
	),
}
