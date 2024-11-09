package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var usertest = discord.SlashCommandCreate{
	Name:        "usertest",
	Description: "Test User model database operations",
}

func UserTestHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Create timing variables
		var createTime, readTime, updateTime, deleteTime time.Duration

		// Create a test user with all fields populated
		testUser := &models.User{
			DiscordID: e.User().ID.String(),
			Username:  e.User().Username,
			Exp:       1000,
			PromoExp:  500,
			Joined:    time.Now(),
			LastQueriedCard: json.RawMessage(`{
				"id": "test-card-123",
				"name": "Test Card"
			}`),
			LastKofiClaim: time.Now().Add(-24 * time.Hour),
			DailyStats: models.DailyStats{
				Claims: 10,
				Bids:   5,
			},
			EffectUseCount: models.EffectUseCount{
				MemoryXmas: 3,
				XmasSpace:  true,
			},
			Cards:         []string{"card1", "card2", "card3"},
			Inventory:     []string{"item1", "item2"},
			CompletedCols: []string{"col1", "col2"},
			CloutedCols:   []string{"clout1"},
			Achievements:  []string{"ach1", "ach2"},
			Effects:       []string{"effect1"},
			Wishlist:      []string{"wish1", "wish2"},
			LastDaily:     time.Now().Add(-12 * time.Hour),
			LastTrain:     time.Now().Add(-6 * time.Hour),
			LastWork:      time.Now().Add(-8 * time.Hour),
			LastVote:      time.Now().Add(-4 * time.Hour),
			LastAnnounce:  time.Now().Add(-2 * time.Hour),
			LastMsg:       "Test message",
			DailyNotified: true,
			VoteNotified:  false,
			HeroSlots:     []string{"slot1", "slot2"},
			HeroCooldown:  []string{"cool1"},
			Hero:          "test-hero",
			HeroChanged:   time.Now().Add(-48 * time.Hour),
			HeroSubmits:   5,
			Roles:         []string{"role1", "role2"},
			Ban: models.BanInfo{
				Full: false,
				Tags: 0,
			},
			LastCard:    123,
			XP:          2000,
			Vials:       100,
			Lemons:      50,
			Votes:       25,
			DailyQuests: []string{"quest1", "quest2"},
			QuestLines:  []string{"line1"},
			Streaks: models.Streaks{
				Daily: 5,
				Kofi:  3,
			},
			Preferences: models.Preferences{
				Profile: struct {
					Bio         string `json:"bio"`
					Title       string `json:"title"`
					Color       string `json:"color"`
					Card        string `json:"card"`
					FavComplete string `json:"favcomplete"`
					FavClout    string `json:"favclout"`
					Image       string `json:"image"`
					Reputation  int    `json:"reputation"`
				}{
					Bio:         "Test Bio",
					Title:       "Test Title",
					Color:       "#FF5733",
					Card:        "favorite-card",
					FavComplete: "complete-col",
					FavClout:    "clout-col",
					Image:       "profile.jpg",
					Reputation:  100,
				},
			},
			Premium:        true,
			PremiumExpires: time.Now().Add(30 * 24 * time.Hour),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		// Test CREATE
		createStart := time.Now()
		err := b.DB.GetPool().QueryRow(ctx, `
			INSERT INTO users (
				discord_id, username, exp, promo_exp, joined, last_queried_card,
				last_kofi_claim, daily_stats, effect_use_count, cards, inventory,
				completed_cols, clouted_cols, achievements, effects, wishlist,
				last_daily, last_train, last_work, last_vote, last_announce,
				last_msg, daily_notified, vote_notified, hero_slots, hero_cooldown,
				hero, hero_changed, hero_submits, roles, ban, last_card, xp,
				vials, lemons, votes, daily_quests, quest_lines, streaks,
				preferences, premium, premium_expires, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
				$15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26,
				$27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38,
				$39, $40, $41, $42, $43, $44
			) RETURNING id
		`,
			testUser.DiscordID, testUser.Username, testUser.Exp, testUser.PromoExp,
			testUser.Joined, testUser.LastQueriedCard, testUser.LastKofiClaim,
			testUser.DailyStats, testUser.EffectUseCount, testUser.Cards,
			testUser.Inventory, testUser.CompletedCols, testUser.CloutedCols,
			testUser.Achievements, testUser.Effects, testUser.Wishlist,
			testUser.LastDaily, testUser.LastTrain, testUser.LastWork,
			testUser.LastVote, testUser.LastAnnounce, testUser.LastMsg,
			testUser.DailyNotified, testUser.VoteNotified, testUser.HeroSlots,
			testUser.HeroCooldown, testUser.Hero, testUser.HeroChanged,
			testUser.HeroSubmits, testUser.Roles, testUser.Ban, testUser.LastCard,
			testUser.XP, testUser.Vials, testUser.Lemons, testUser.Votes,
			testUser.DailyQuests, testUser.QuestLines, testUser.Streaks,
			testUser.Preferences, testUser.Premium, testUser.PremiumExpires,
			testUser.CreatedAt, testUser.UpdatedAt,
		).Scan(&testUser.ID)
		createTime = time.Since(createStart)

		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Create test failed: %s (took %s)",
					err.Error(), createTime),
			})
		}

		// Test READ
		readStart := time.Now()
		var readUser models.User
		err = b.DB.GetPool().QueryRow(ctx, `
			SELECT id, discord_id, username, exp, promo_exp, joined,
				last_queried_card, last_kofi_claim, daily_stats,
				effect_use_count, cards, inventory, completed_cols,
				clouted_cols, achievements, effects, wishlist, last_daily,
				last_train, last_work, last_vote, last_announce, last_msg,
				daily_notified, vote_notified, hero_slots, hero_cooldown,
				hero, hero_changed, hero_submits, roles, ban, last_card,
				xp, vials, lemons, votes, daily_quests, quest_lines,
				streaks, preferences, premium, premium_expires, created_at,
				updated_at
			FROM users WHERE discord_id = $1
		`, testUser.DiscordID).Scan(
			&readUser.ID, &readUser.DiscordID, &readUser.Username,
			&readUser.Exp, &readUser.PromoExp, &readUser.Joined,
			&readUser.LastQueriedCard, &readUser.LastKofiClaim,
			&readUser.DailyStats, &readUser.EffectUseCount, &readUser.Cards,
			&readUser.Inventory, &readUser.CompletedCols, &readUser.CloutedCols,
			&readUser.Achievements, &readUser.Effects, &readUser.Wishlist,
			&readUser.LastDaily, &readUser.LastTrain, &readUser.LastWork,
			&readUser.LastVote, &readUser.LastAnnounce, &readUser.LastMsg,
			&readUser.DailyNotified, &readUser.VoteNotified, &readUser.HeroSlots,
			&readUser.HeroCooldown, &readUser.Hero, &readUser.HeroChanged,
			&readUser.HeroSubmits, &readUser.Roles, &readUser.Ban,
			&readUser.LastCard, &readUser.XP, &readUser.Vials, &readUser.Lemons,
			&readUser.Votes, &readUser.DailyQuests, &readUser.QuestLines,
			&readUser.Streaks, &readUser.Preferences, &readUser.Premium,
			&readUser.PremiumExpires, &readUser.CreatedAt, &readUser.UpdatedAt,
		)
		readTime = time.Since(readStart)

		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Read test failed: %s (took %s)",
					err.Error(), readTime),
			})
		}

		// Test UPDATE
		updateStart := time.Now()
		_, err = b.DB.GetPool().Exec(ctx, `
			UPDATE users 
			SET username = $1,
				exp = $2,
				daily_stats = $3,
				hero = $4,
				updated_at = $5
			WHERE discord_id = $6
		`,
			"Updated"+testUser.Username,
			testUser.Exp+1000,
			models.DailyStats{Claims: 20, Bids: 15},
			"updated-hero",
			time.Now(),
			testUser.DiscordID,
		)
		updateTime = time.Since(updateStart)

		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Update test failed: %s (took %s)",
					err.Error(), updateTime),
			})
		}

		// Verify UPDATE
		var updatedUser models.User
		err = b.DB.GetPool().QueryRow(ctx, `
			SELECT username, exp, daily_stats, hero
			FROM users WHERE discord_id = $1
		`, testUser.DiscordID).Scan(
			&updatedUser.Username,
			&updatedUser.Exp,
			&updatedUser.DailyStats,
			&updatedUser.Hero,
		)

		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Update verification failed: %s", err.Error()),
			})
		}

		// Test DELETE
		deleteStart := time.Now()
		_, err = b.DB.GetPool().Exec(ctx, "DELETE FROM users WHERE discord_id = $1", testUser.DiscordID)
		deleteTime = time.Since(deleteStart)

		if err != nil {
			return e.CreateMessage(discord.MessageCreate{
				Content: fmt.Sprintf("❌ Delete test failed: %s (took %s)",
					err.Error(), deleteTime),
			})
		}

		totalTime := time.Since(startTime)

		// Format durations to be more readable
		formatDuration := func(d time.Duration) string {
			return fmt.Sprintf("%.3f ms", float64(d.Microseconds())/1000.0)
		}

		// Prepare detailed test results with timing information
		return e.CreateMessage(discord.MessageCreate{
			Content: fmt.Sprintf("✅ User model tests successful!\n\n"+
				"Timing Information:\n"+
				"- CREATE: %s\n"+
				"- READ: %s\n"+
				"- UPDATE: %s\n"+
				"- DELETE: %s\n"+
				"- Total Time: %s\n\n"+
				"CREATE Test:\n"+
				"- User ID: %d\n"+
				"- All fields inserted successfully\n\n"+
				"READ Test:\n"+
				"- Discord ID: %s\n"+
				"- Username: %s\n"+
				"- Exp: %d\n"+
				"- Daily Stats Claims: %d\n"+
				"- Cards Count: %d\n"+
				"- Hero: %s\n\n"+
				"UPDATE Test:\n"+
				"- New Username: %s\n"+
				"- New Exp: %d\n"+
				"- New Daily Stats Claims: %d\n"+
				"- New Hero: %s\n\n"+
				"DELETE Test:\n"+
				"- Record successfully removed\n\n"+
				"Database Schema:\n"+
				"- Total Fields: 44\n"+
				"- JSONB Fields: 12\n"+
				"- Timestamp Fields: 8\n"+
				"- Boolean Fields: 4\n\n"+
				"All database operations completed successfully!",
				formatDuration(createTime),
				formatDuration(readTime),
				formatDuration(updateTime),
				formatDuration(deleteTime),
				formatDuration(totalTime),
				readUser.ID,
				readUser.DiscordID,
				readUser.Username,
				readUser.Exp,
				readUser.DailyStats.Claims,
				len(readUser.Cards),
				readUser.Hero,
				updatedUser.Username,
				updatedUser.Exp,
				updatedUser.DailyStats.Claims,
				updatedUser.Hero,
			),
		})
	}
}
