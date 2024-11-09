package commands

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var usercardtest = discord.SlashCommandCreate{
	Name:        "usercardtest",
	Description: "Test UserCard model database operations",
}

func logError(operation string, err error, duration time.Duration) {
	log.Printf("[UserCardTest] %s failed: %v (took %.3f ms)\n",
		operation,
		err,
		float64(duration.Microseconds())/1000.0,
	)
}

func logSuccess(operation string, duration time.Duration) {
	log.Printf("[UserCardTest] %s successful (took %.3f ms)\n",
		operation,
		float64(duration.Microseconds())/1000.0,
	)
}

func UserCardTestHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		log.Println("[DB Test] Starting UserCard database test...")

		// First, test if table exists
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Create table if not exists
		createTableSQL := `
		CREATE TABLE IF NOT EXISTS user_cards (
			id BIGSERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			card_id BIGINT NOT NULL,
			favorite BOOLEAN NOT NULL DEFAULT false,
			locked BOOLEAN NOT NULL DEFAULT false,
			amount BIGINT NOT NULL DEFAULT 1,
			rating BIGINT NOT NULL DEFAULT 0,
			obtained TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			exp BIGINT NOT NULL DEFAULT 0,
			mark TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL
		);`

		log.Println("[DB Test] Creating table if not exists...")
		_, err := b.DB.ExecWithLog(ctx, createTableSQL)
		if err != nil {
			log.Printf("[DB Error] Failed to create table: %v", err)
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Failed to create table: %v", err),
			})
		}

		// Create indexes if they don't exist
		indexes := []string{
			"CREATE INDEX IF NOT EXISTS idx_user_cards_user_id ON user_cards(user_id);",
			"CREATE INDEX IF NOT EXISTS idx_user_cards_card_id ON user_cards(card_id);",
			"CREATE INDEX IF NOT EXISTS idx_user_cards_user_card ON user_cards(user_id, card_id);",
		}

		for _, indexSQL := range indexes {
			log.Printf("[DB Test] Creating index: %s", indexSQL)
			_, err := b.DB.ExecWithLog(ctx, indexSQL)
			if err != nil {
				log.Printf("[DB Error] Failed to create index: %v", err)
				return e.CreateMessage(discord.MessageCreate{
					Content: fmt.Sprintf("❌ Failed to create index: %v", err),
				})
			}
		}

		// Test INSERT
		testCard := &models.UserCard{
			UserID:   e.User().ID.String(),
			CardID:   12345,
			Favorite: true,
			Amount:   1,
			Exp:      100,
			Mark:     "test",
		}

		insertSQL := `
		INSERT INTO user_cards (user_id, card_id, favorite, amount, exp, mark, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id;`

		log.Println("[DB Test] Inserting test record...")
		now := time.Now()
		var id int64
		err = b.DB.GetPool().QueryRow(ctx, insertSQL,
			testCard.UserID,
			testCard.CardID,
			testCard.Favorite,
			testCard.Amount,
			testCard.Exp,
			testCard.Mark,
			now,
			now,
		).Scan(&id)

		if err != nil {
			log.Printf("[DB Error] Insert failed: %v", err)
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Insert test failed: %v", err),
			})
		}
		log.Printf("[DB Success] Inserted record with ID: %d", id)

		// Test SELECT
		log.Println("[DB Test] Testing SELECT...")
		selectSQL := `
		SELECT id, user_id, card_id, favorite, amount, exp, mark 
		FROM user_cards 
		WHERE id = $1;`

		var readCard models.UserCard
		err = b.DB.GetPool().QueryRow(ctx, selectSQL, id).Scan(
			&readCard.ID,
			&readCard.UserID,
			&readCard.CardID,
			&readCard.Favorite,
			&readCard.Amount,
			&readCard.Exp,
			&readCard.Mark,
		)

		if err != nil {
			log.Printf("[DB Error] Select failed: %v", err)
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Select test failed: %v", err),
			})
		}
		log.Printf("[DB Success] Selected record: %+v", readCard)

		// Test DELETE
		log.Println("[DB Test] Testing DELETE...")
		deleteSQL := `DELETE FROM user_cards WHERE id = $1;`
		result, err := b.DB.ExecWithLog(ctx, deleteSQL, id)
		if err != nil {
			log.Printf("[DB Error] Delete failed: %v", err)
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Delete test failed: %v", err),
			})
		}
		log.Printf("[DB Success] Deleted %d records", result.RowsAffected())

		log.Println("[DB Test] All tests completed successfully!")
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("✅ Database tests completed successfully!\n"+
				"Created table and indexes\n"+
				"Inserted record ID: %d\n"+
				"Read record successfully\n"+
				"Deleted record successfully\n"+
				"Check console for detailed logs", id),
		})
	}
}
