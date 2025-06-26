package admin

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var DBTest = discord.SlashCommandCreate{
	Name:        "dbtest",
	Description: "Test database connectivity and operations",
}

func DBTestHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		start := time.Now()
		defer func() {
			slog.Info("Command completed",
				slog.String("type", "cmd"),
				slog.String("name", "dbtest"),
				slog.Duration("total_time", time.Since(start)),
			)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		slog.Info("Starting database tests",
			slog.String("type", "cmd"),
			slog.String("phase", "init"),
		)

		// Test 1: Connection check
		queryStart := time.Now()
		var result int
		rows, err := b.DB.QueryWithLog(ctx, "SELECT 1")
		if err != nil {
			slog.Error("Connection test failed",
				slog.String("type", "db"),
				slog.String("phase", "connection"),
				slog.Duration("took", time.Since(queryStart)),
				slog.Any("error", err),
			)
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Database connection test failed: %s", err.Error()),
			})
		}
		defer rows.Close()

		// Scan the result
		if rows.Next() {
			err = rows.Scan(&result)
			if err != nil {
				slog.Error("Failed to scan result",
					slog.String("type", "db"),
					slog.String("phase", "connection"),
					slog.Any("error", err),
				)
				return e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Failed to scan result: %s", err.Error()),
				})
			}
		}

		// Test 2: Create table
		_, err = b.DB.ExecWithLog(ctx, `
			CREATE TABLE IF NOT EXISTS db_test (
				id SERIAL PRIMARY KEY,
				test_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			slog.Error("Table creation failed",
				slog.String("type", "db"),
				slog.String("phase", "create_table"),
				slog.Any("error", err),
			)
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Failed to create test table: %s", err.Error()),
			})
		}

		// Test 3: Insert data
		_, err = b.DB.ExecWithLog(ctx, "INSERT INTO db_test (test_time) VALUES (CURRENT_TIMESTAMP)")
		if err != nil {
			slog.Error("Data insertion failed",
				slog.String("type", "db"),
				slog.String("phase", "insert"),
				slog.Any("error", err),
			)
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Failed to insert test record: %s", err.Error()),
			})
		}

		// Test 4: Query data
		var testTime time.Time
		rows, err = b.DB.QueryWithLog(ctx, "SELECT test_time FROM db_test ORDER BY id DESC LIMIT 1")
		if err != nil {
			slog.Error("Data query failed",
				slog.String("type", "db"),
				slog.String("phase", "query"),
				slog.Any("error", err),
			)
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Failed to query test record: %s", err.Error()),
			})
		}
		defer rows.Close()

		if rows.Next() {
			err = rows.Scan(&testTime)
			if err != nil {
				slog.Error("Failed to scan test time",
					slog.String("type", "db"),
					slog.String("phase", "query"),
					slog.Any("error", err),
				)
				return e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Failed to scan test time: %s", err.Error()),
				})
			}
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
