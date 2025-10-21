package system

import (
    "fmt"

    "github.com/disgoorg/bot-template/bottemplate"
    "github.com/disgoorg/bot-template/bottemplate/utils"
    "github.com/disgoorg/disgo/discord"
    "github.com/disgoorg/disgo/handler"
)

var Version = discord.SlashCommandCreate{
	Name:        "version",
	Description: "version command",
}

func VersionHandler(b *bottemplate.Bot) handler.CommandHandler {
    return func(e *handler.CommandEvent) error {
        if err := e.DeferCreateMessage(false); err != nil { return err }
        _, err := e.UpdateInteractionResponse(discord.MessageUpdate{Content: utils.Ptr(fmt.Sprintf("Version: %s\nCommit: %s", b.Version, b.Commit))})
        return err
    }
}
