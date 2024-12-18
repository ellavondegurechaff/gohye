package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var Balance = discord.SlashCommandCreate{
	Name:        "balance",
	Description: "💰 View your current balance and earnings",
}

func BalanceHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		start := time.Now()
		defer func() {
			slog.Info("Command completed",
				slog.String("type", "cmd"),
				slog.String("name", "balance"),
				slog.Duration("total_time", time.Since(start)),
			)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		user, err := b.UserRepository.GetByDiscordID(ctx, e.User().ID.String())
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Embeds: []discord.Embed{{
					Title:       "Error",
					Description: "Failed to fetch your balance. Please try again later.",
					Color:       utils.ErrorColor,
				}},
			})
		}

		balanceBar := createBalanceBar(user.Balance)
		vialBar := createBalanceBar(user.UserStats.Vials)

		description := fmt.Sprintf("```ansi\n"+
			"\x1b[1;36mBalance:\x1b[0m %d credits\n"+
			"\x1b[0;37m%s\x1b[0m\n"+
			"\n"+
			"\x1b[1;35mVials:\x1b[0m %d\n"+
			"\x1b[0;37m%s\x1b[0m\n"+
			"```",
			user.Balance,
			balanceBar,
			user.UserStats.Vials,
			vialBar,
		)

		now := time.Now()
		return e.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{{
				Title:       "💰 Balance",
				Description: description,
				Color:       utils.SuccessColor,
				Footer: &discord.EmbedFooter{
					Text: fmt.Sprintf("Requested by %s", e.User().Username),
				},
				Timestamp: &now,
			}},
		})
	}
}

func createBalanceBar(balance int64) string {
	const barLength = 10

	// Calculate milestone based on balance range
	var milestone int64 = 1000000 // 1M milestone for high balances
	if balance < 100000 {
		milestone = 100000 // 100k milestone for lower balances
	} else if balance < 500000 {
		milestone = 500000 // 500k milestone for medium balances
	}

	// Calculate progress
	progress := float64(balance) / float64(milestone)
	if progress > 1.0 {
		progress = 1.0
	}
	filled := int(progress * float64(barLength))

	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar.WriteString("■")
		} else {
			bar.WriteString("□")
		}
	}
	bar.WriteString(fmt.Sprintf("] %.1f%%", progress*100))

	return bar.String()
}
