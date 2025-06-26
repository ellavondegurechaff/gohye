package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/bot-template/bottemplate/commands/admin"
	"github.com/disgoorg/bot-template/bottemplate/commands/cards"
	"github.com/disgoorg/bot-template/bottemplate/commands/economy"
	"github.com/disgoorg/bot-template/bottemplate/commands/social"
	"github.com/disgoorg/bot-template/bottemplate/commands/system"
)

var Commands = []discord.ApplicationCommandCreate{}

func init() {
	Commands = append(Commands, admin.Commands...)
	Commands = append(Commands, cards.Commands...)
	Commands = append(Commands, economy.Commands...)
	Commands = append(Commands, social.Commands...)
	Commands = append(Commands, system.Commands...)
}
