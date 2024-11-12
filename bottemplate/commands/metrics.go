package commands

import (
	"fmt"
	"runtime"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var metrics = discord.SlashCommandCreate{
	Name:        "metrics",
	Description: "📊 View bot performance metrics and statistics",
}

func MetricsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		// Get memory statistics
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Calculate uptime
		uptime := time.Since(b.StartTime)

		// Get number of goroutines
		numGoroutines := runtime.NumGoroutine()

		// Measure command latency
		cmdStart := time.Now()
		cmdLatency := time.Since(cmdStart)

		// Get gateway latency
		gatewayLatency := b.Client.Gateway().Latency()

		// Create embed fields for different metric categories
		memoryField := fmt.Sprintf("```\n"+
			"Alloc: %.2f MB\n"+
			"Total Alloc: %.2f MB\n"+
			"Sys: %.2f MB\n"+
			"NumGC: %d\n"+
			"Goroutines: %d\n"+
			"```",
			float64(m.Alloc)/1024/1024,
			float64(m.TotalAlloc)/1024/1024,
			float64(m.Sys)/1024/1024,
			m.NumGC,
			numGoroutines,
		)

		latencyField := fmt.Sprintf("```\n"+
			"Command: %s\n"+
			"Gateway: %s\n"+
			"```",
			cmdLatency.String(),
			gatewayLatency.String(),
		)

		uptimeField := fmt.Sprintf("```\n"+
			"Days: %d\n"+
			"Hours: %d\n"+
			"Minutes: %d\n"+
			"```",
			int(uptime.Hours())/24,
			int(uptime.Hours())%24,
			int(uptime.Minutes())%60,
		)

		// Create and send embed
		embed := discord.NewEmbedBuilder().
			SetTitle("🔧 Bot Performance Metrics").
			SetDescription("Current performance statistics and metrics").
			AddField("💾 Memory Usage", memoryField, false).
			AddField("⚡ Latency", latencyField, false).
			AddField("⏰ Uptime", uptimeField, false).
			SetColor(0x00FF00).
			SetTimestamp(time.Now()).
			SetFooter("Requested by "+e.User().Username, e.User().EffectiveAvatarURL())

		return e.CreateMessage(discord.MessageCreate{
			Embeds: []discord.Embed{embed.Build()},
		})
	}
}
