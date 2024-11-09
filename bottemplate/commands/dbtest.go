package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var dbtest = discord.SlashCommandCreate{
	Name:        "dbtest",
	Description: "Test database connectivity and operations",
}

func DBTestHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Test database connection with a simple query
		var result int
		err := b.DB.GetPool().QueryRow(ctx, "SELECT 1").Scan(&result)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Database test failed: %s", err.Error()),
			})
		}

		// Create a test table if it doesn't exist
		_, err = b.DB.GetPool().Exec(ctx, `
			CREATE TABLE IF NOT EXISTS db_test (
				id SERIAL PRIMARY KEY,
				test_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Failed to create test table: %s", err.Error()),
			})
		}

		// Insert a test record
		_, err = b.DB.GetPool().Exec(ctx, "INSERT INTO db_test (test_time) VALUES (CURRENT_TIMESTAMP)")
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Failed to insert test record: %s", err.Error()),
			})
		}

		// Query the last record
		var testTime time.Time
		err = b.DB.GetPool().QueryRow(ctx, "SELECT test_time FROM db_test ORDER BY id DESC LIMIT 1").Scan(&testTime)
		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Failed to query test record: %s", err.Error()),
			})
		}

		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("✅ Database test successful!\n"+
				"- Connection: OK\n"+
				"- Table Creation: OK\n"+
				"- Insert: OK\n"+
				"- Query: OK\n"+
				"Last test record timestamp: %s", testTime.Format(time.RFC3339)),
		})
	}
}
