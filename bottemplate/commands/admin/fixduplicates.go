package admin

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var FixDuplicates = discord.SlashCommandCreate{
	Name:        "fixduplicates",
	Description: "🛠️ Fix duplicate cards in all collections",
}

func FixDuplicatesHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if err := e.DeferCreateMessage(false); err != nil {
			return fmt.Errorf("failed to defer response: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()

		var userIDs []string
		err := b.DB.BunDB().NewSelect().
			ColumnExpr("DISTINCT user_id").
			TableExpr("user_cards").
			Scan(ctx, &userIDs)

		if err != nil {
			log.Printf("[ERROR] Failed to fetch user IDs: %v", err)
			_, err := e.CreateFollowupMessage(discord.MessageCreate{
				Content: "❌ Failed to fetch users",
			})
			if err != nil {
				return fmt.Errorf("failed to send error message: %w", err)
			}
			return nil
		}

		var totalUsersFixed atomic.Int64
		var totalCardsFixed atomic.Int64

		const maxWorkers = 10
		userChan := make(chan string, len(userIDs))
		var wg sync.WaitGroup

		for i := 0; i < maxWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for userID := range userChan {
					if fixed, cards := processUser(ctx, b, userID); fixed {
						totalUsersFixed.Add(1)
						totalCardsFixed.Add(cards)
					}
				}
			}()
		}

		for _, userID := range userIDs {
			userChan <- userID
		}
		close(userChan)

		wg.Wait()

		now := time.Now()
		_, err = e.CreateFollowupMessage(discord.MessageCreate{
			Embeds: []discord.Embed{
				{
					Title:       "🛠️ Global Duplicate Fix Results",
					Description: fmt.Sprintf("Fixed %d duplicate card entries across %d users!", totalCardsFixed.Load(), totalUsersFixed.Load()),
					Color:       utils.SuccessColor,
					Fields: []discord.EmbedField{
						{
							Name:   "Total Users Processed",
							Value:  fmt.Sprintf("%d", len(userIDs)),
							Inline: nil,
						},
						{
							Name:   "Users With Fixes",
							Value:  fmt.Sprintf("%d", totalUsersFixed.Load()),
							Inline: nil,
						},
						{
							Name:   "Total Cards Fixed",
							Value:  fmt.Sprintf("%d", totalCardsFixed.Load()),
							Inline: nil,
						},
					},
					Footer: &discord.EmbedFooter{
						Text: "All collections have been cleaned up!",
					},
					Timestamp: &now,
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to send success message: %w", err)
		}
		return nil
	}
}

func processUser(ctx context.Context, b *bottemplate.Bot, userID string) (bool, int64) {
	tx, err := b.DB.BunDB().BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to start transaction for user %s: %v", userID, err)
		return false, 0
	}
	defer tx.Rollback()

	var duplicates []struct {
		CardID int64 `bun:"card_id"`
		Count  int64 `bun:"count"`
	}

	err = tx.NewSelect().
		TableExpr("user_cards").
		Column("card_id").
		ColumnExpr("COUNT(*) as count").
		Where("user_id = ?", userID).
		Group("card_id").
		Having("COUNT(*) > 1").
		Scan(ctx, &duplicates)

	if err != nil {
		log.Printf("[ERROR] Failed to find duplicates for user %s: %v", userID, err)
		return false, 0
	}

	if len(duplicates) == 0 {
		return false, 0
	}

	var cardsFixed int64
	for _, dup := range duplicates {
		_, err := tx.NewRaw(`
            WITH duplicates AS (
                SELECT MIN(id) as keep_id,
                       SUM(amount) as total_amount,
                       MAX(exp) as max_exp,
                       bool_or(favorite) as is_favorite,
                       bool_or(locked) as is_locked,
                       MIN(obtained) as first_obtained,
                       MAX(rating) as max_rating,
                       string_agg(DISTINCT mark, ', ') as combined_marks
                FROM user_cards
                WHERE user_id = ? AND card_id = ?
            )
            UPDATE user_cards
            SET amount = duplicates.total_amount,
                exp = duplicates.max_exp,
                favorite = duplicates.is_favorite,
                locked = duplicates.is_locked,
                obtained = duplicates.first_obtained,
                rating = duplicates.max_rating,
                mark = duplicates.combined_marks,
                updated_at = ?
            FROM duplicates
            WHERE user_cards.id = duplicates.keep_id
        `, userID, dup.CardID, time.Now()).Exec(ctx)

		if err != nil {
			log.Printf("[ERROR] Failed to update card %d for user %s: %v", dup.CardID, userID, err)
			continue
		}

		_, err = tx.NewDelete().
			Model((*models.UserCard)(nil)).
			Where("user_id = ? AND card_id = ? AND id != (SELECT MIN(id) FROM user_cards WHERE user_id = ? AND card_id = ?)",
				userID, dup.CardID, userID, dup.CardID).
			Exec(ctx)

		if err != nil {
			log.Printf("[ERROR] Failed to delete duplicates for card %d user %s: %v", dup.CardID, userID, err)
			continue
		}

		cardsFixed++
		log.Printf("[INFO] Fixed card %d for user %s", dup.CardID, userID)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[ERROR] Failed to commit transaction for user %s: %v", userID, err)
		return false, 0
	}

	return true, cardsFixed
}
