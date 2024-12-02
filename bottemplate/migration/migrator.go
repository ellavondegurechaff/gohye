package migration

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
	"go.mongodb.org/mongo-driver/bson"
)

type Migrator struct {
	pgDB      *bun.DB
	usersPath string
	cardsPath string
	batchSize int
}

func NewMigrator(pgDB *bun.DB, usersPath, cardsPath string) *Migrator {
	return &Migrator{
		pgDB:      pgDB,
		usersPath: usersPath,
		cardsPath: cardsPath,
		batchSize: 1000,
	}
}

func (m *Migrator) MigrateAll(ctx context.Context) error {
	// First migrate cards from JSON
	if err := m.MigrateCardsFromJSON(ctx); err != nil {
		return fmt.Errorf("failed to migrate cards: %w", err)
	}

	// Then migrate users
	if err := m.MigrateUsers(ctx); err != nil {
		return fmt.Errorf("failed to migrate users: %w", err)
	}

	// Finally migrate user cards since cards table now exists
	if err := m.MigrateUserCards(ctx); err != nil {
		return fmt.Errorf("failed to migrate user cards: %w", err)
	}

	return nil
}

func (m *Migrator) MigrateUsers(ctx context.Context) error {
	slog.Info("Starting user migration",
		"usersPath", m.usersPath,
		"batchSize", m.batchSize)

	file, err := os.Open(m.usersPath)
	if err != nil {
		slog.Error("Failed to open users BSON file", "error", err)
		return fmt.Errorf("failed to open users BSON file: %w", err)
	}
	defer file.Close()

	var mongoUsers []MongoUser

	reader := bufio.NewReader(file)
	for {
		// Each BSON document starts with an int32 length
		lengthBytes := make([]byte, 4)
		_, err := io.ReadFull(reader, lengthBytes)
		if err == io.EOF {
			break // End of file reached
		}
		if err != nil {
			slog.Error("Failed to read document length", "error", err)
			return fmt.Errorf("failed to read document length: %w", err)
		}

		length := int32(binary.LittleEndian.Uint32(lengthBytes))
		if length <= 0 {
			slog.Error("Invalid document length", "length", length)
			return fmt.Errorf("invalid document length: %d", length)
		}

		// The length includes the 4 bytes of the length itself, so we've already read 4 bytes
		docBytes := make([]byte, length-4)
		_, err = io.ReadFull(reader, docBytes)
		if err != nil {
			slog.Error("Failed to read document bytes", "error", err)
			return fmt.Errorf("failed to read document bytes: %w", err)
		}

		// Prepend the lengthBytes to the docBytes
		fullDocBytes := append(lengthBytes, docBytes...)

		var mu MongoUser
		err = bson.Unmarshal(fullDocBytes, &mu)
		if err != nil {
			slog.Error("Failed to decode users BSON", "error", err)
			return fmt.Errorf("failed to decode users BSON: %w", err)
		}
		mongoUsers = append(mongoUsers, mu)
	}

	slog.Info("Loaded users from BSON file", "count", len(mongoUsers))

	// Proceed with your migration logic
	// For example, batch insert the users
	return m.processUsers(ctx, mongoUsers)
}

func (m *Migrator) processUsers(ctx context.Context, mongoUsers []MongoUser) error {
	batchSize := m.batchSize
	discordIDUserMap := make(map[string]*models.User)

	// Build a map of unique discord_id to User
	for _, mongoUser := range mongoUsers {
		pgUser := m.convertUser(mongoUser)

		if pgUser.DiscordID == "" {
			continue // Skip if discord_id is empty
		}

		// Log duplicates if necessary
		if _, exists := discordIDUserMap[pgUser.DiscordID]; exists {
			slog.Warn("Duplicate discord_id found", "discord_id", pgUser.DiscordID)
		}

		// Keep the latest occurrence or merge data if necessary
		discordIDUserMap[pgUser.DiscordID] = pgUser
	}

	// Convert the map to a slice
	var users []*models.User
	for _, user := range discordIDUserMap {
		users = append(users, user)
	}

	// Process users in batches
	totalUsers := len(users)
	for i := 0; i < totalUsers; i += batchSize {
		end := i + batchSize
		if end > totalUsers {
			end = totalUsers
		}
		batch := users[i:end]

		slog.Info("Inserting batch of users",
			"batchSize", len(batch),
			"progress", fmt.Sprintf("%d/%d", end, totalUsers))

		if err := m.batchInsertUsers(ctx, batch); err != nil {
			slog.Error("Failed to insert user batch",
				"error", err,
				"batchSize", len(batch))
			return err
		}
	}

	slog.Info("User migration completed successfully")
	return nil
}

func (m *Migrator) MigrateUserCards(ctx context.Context) error {
	slog.Info("Starting user cards migration",
		"cardsPath", m.cardsPath,
		"batchSize", m.batchSize)

	file, err := os.Open(m.cardsPath)
	if err != nil {
		slog.Error("Failed to open user cards BSON file", "error", err)
		return fmt.Errorf("failed to open user cards BSON file: %w", err)
	}
	defer file.Close()

	var mongoCards []MongoUserCard

	reader := bufio.NewReader(file)
	for {
		lengthBytes := make([]byte, 4)
		_, err := io.ReadFull(reader, lengthBytes)
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("Failed to read document length", "error", err)
			return fmt.Errorf("failed to read document length: %w", err)
		}

		length := int32(binary.LittleEndian.Uint32(lengthBytes))
		if length <= 0 {
			slog.Error("Invalid document length", "length", length)
			return fmt.Errorf("invalid document length: %d", length)
		}

		docBytes := make([]byte, length-4)
		_, err = io.ReadFull(reader, docBytes)
		if err != nil {
			slog.Error("Failed to read document bytes", "error", err)
			return fmt.Errorf("failed to read document bytes: %w", err)
		}

		fullDocBytes := append(lengthBytes, docBytes...)

		var mc MongoUserCard
		err = bson.Unmarshal(fullDocBytes, &mc)
		if err != nil {
			slog.Error("Failed to decode user cards BSON", "error", err)
			return fmt.Errorf("failed to decode user cards BSON: %w", err)
		}
		mongoCards = append(mongoCards, mc)
	}

	slog.Info("Loaded user cards from BSON file", "count", len(mongoCards))

	// Proceed with your migration logic
	return m.processUserCards(ctx, mongoCards)
}

func (m *Migrator) processUserCards(ctx context.Context, mongoCards []MongoUserCard) error {
	// First, get all valid card IDs from the cards table
	var validCardIDs []int64
	err := m.pgDB.NewSelect().
		Model((*models.Card)(nil)).
		Column("id").
		Scan(ctx, &validCardIDs)
	if err != nil {
		return fmt.Errorf("failed to get valid card IDs: %w", err)
	}

	// Create a map for O(1) lookups
	validCardIDsMap := make(map[int64]bool)
	for _, id := range validCardIDs {
		validCardIDsMap[id] = true
	}

	// Create a file for logging skipped cards
	skippedFile, err := os.Create("skipped_cards.log")
	if err != nil {
		return fmt.Errorf("failed to create skipped cards log file: %w", err)
	}
	defer skippedFile.Close()

	// Write header to the file
	_, err = fmt.Fprintf(skippedFile, "timestamp,user_id,card_id,reason\n")
	if err != nil {
		return fmt.Errorf("failed to write header to log file: %w", err)
	}

	var userCards []*models.UserCard
	skippedCount := 0
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	for _, mongoCard := range mongoCards {
		if mongoCard.CardID == nil {
			skippedCount++
			_, err = fmt.Fprintf(skippedFile, "%s,%s,null,null_card_id\n",
				timestamp, mongoCard.UserID)
			if err != nil {
				logProgress(fmt.Sprintf("Failed to write to log file: %v", err))
			}
			continue
		}

		cardID := int64(*mongoCard.CardID)

		// Skip if card ID doesn't exist in cards table
		if !validCardIDsMap[cardID] {
			skippedCount++
			_, err = fmt.Fprintf(skippedFile, "%s,%s,%d,missing_from_cards_table\n",
				timestamp, mongoCard.UserID, cardID)
			if err != nil {
				logProgress(fmt.Sprintf("Failed to write to log file: %v", err))
			}
			continue
		}

		userCards = append(userCards, &models.UserCard{
			UserID:    mongoCard.UserID,
			CardID:    cardID,
			Favorite:  mongoCard.Fav,
			Locked:    mongoCard.Locked,
			Amount:    int64(mongoCard.Amount),
			Rating:    int64(mongoCard.Rating),
			Obtained:  mongoCard.Obtained,
			Exp:       int64(mongoCard.Exp),
			Mark:      mongoCard.Mark,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})

		if len(userCards) >= m.batchSize {
			if err := m.batchInsertUserCards(ctx, userCards); err != nil {
				return err
			}
			logProgress(fmt.Sprintf("Processed %d user cards, skipped %d so far", len(userCards), skippedCount))
			userCards = userCards[:0]
		}
	}

	// Insert remaining user cards
	if len(userCards) > 0 {
		if err := m.batchInsertUserCards(ctx, userCards); err != nil {
			return err
		}
	}

	// Write summary to log file
	_, err = fmt.Fprintf(skippedFile, "\nSummary:\nTotal skipped: %d\nTimestamp: %s\n",
		skippedCount, timestamp)
	if err != nil {
		logProgress(fmt.Sprintf("Failed to write summary to log file: %v", err))
	}

	logProgress(fmt.Sprintf("Migration completed. Skipped %d invalid/missing card IDs. Check skipped_cards.log for details", skippedCount))
	return nil
}

func (m *Migrator) batchInsertUsers(ctx context.Context, users []*models.User) error {
	startTime := time.Now()
	slog.Info("Starting batch insert of users", "count", len(users))

	_, err := m.pgDB.NewInsert().
		Model(&users).
		On("CONFLICT (discord_id) DO UPDATE").
		Set("username = EXCLUDED.username").
		Set("user_stats = EXCLUDED.user_stats").
		Set("promo_exp = EXCLUDED.promo_exp").
		Set("joined = EXCLUDED.joined").
		Set("last_queried_card = EXCLUDED.last_queried_card").
		Set("last_kofi_claim = EXCLUDED.last_kofi_claim").
		Set("daily_stats = EXCLUDED.daily_stats").
		Set("effect_stats = EXCLUDED.effect_stats").
		Set("cards = EXCLUDED.cards").
		Set("inventory = EXCLUDED.inventory").
		Set("completed_cols = EXCLUDED.completed_cols").
		Set("clouted_cols = EXCLUDED.clouted_cols").
		Set("achievements = EXCLUDED.achievements").
		Set("effects = EXCLUDED.effects").
		Set("wishlist = EXCLUDED.wishlist").
		Set("preferences = EXCLUDED.preferences").
		Set("last_daily = EXCLUDED.last_daily").
		Set("last_train = EXCLUDED.last_train").
		Set("last_work = EXCLUDED.last_work").
		Set("last_vote = EXCLUDED.last_vote").
		Set("last_announce = EXCLUDED.last_announce").
		Set("last_msg = EXCLUDED.last_msg").
		Set("hero_slots = EXCLUDED.hero_slots").
		Set("hero_cooldown = EXCLUDED.hero_cooldown").
		Set("hero = EXCLUDED.hero").
		Set("hero_changed = EXCLUDED.hero_changed").
		Set("hero_submits = EXCLUDED.hero_submits").
		Set("roles = EXCLUDED.roles").
		Set("ban = EXCLUDED.ban").
		Set("premium = EXCLUDED.premium").
		Set("premium_expires = EXCLUDED.premium_expires").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)

	if err != nil {
		slog.Error("Batch insert of users failed",
			"error", err,
			"duration", time.Since(startTime))
		return fmt.Errorf("batch insert failed: %w", err)
	}

	slog.Info("Batch insert of users completed",
		"count", len(users),
		"duration", time.Since(startTime))
	return nil
}

func (m *Migrator) batchInsertUserCards(ctx context.Context, userCards []*models.UserCard) error {
	startTime := time.Now()
	logProgress(fmt.Sprintf("Starting batch insert of user cards: %d", len(userCards)))

	_, err := m.pgDB.NewInsert().
		Model(&userCards).
		Exec(ctx)

	if err != nil {
		logProgress(fmt.Sprintf("Batch insert failed: %v", err))
		return fmt.Errorf("failed to insert user cards batch: %w", err)
	}

	logProgress(fmt.Sprintf("Batch insert completed successfully in %v", time.Since(startTime)))
	return nil
}
