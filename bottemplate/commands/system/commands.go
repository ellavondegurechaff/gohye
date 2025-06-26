package system

import "github.com/disgoorg/disgo/discord"

var Commands = []discord.ApplicationCommandCreate{
	Version,
	Metrics,
	Inventory,
	UseEffect,
	CraftEffect,
	Help,
}