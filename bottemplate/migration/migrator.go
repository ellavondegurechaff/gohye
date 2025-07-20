package migration

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"
	"go.mongodb.org/mongo-driver/bson"
)

type Migrator struct {
	pgDB      *bun.DB
	dataDir   string
	usersPath string
	cardsPath string
	batchSize int
	// Statistics tracking
	stats MigrationStats
}

func NewMigrator(pgDB *bun.DB, dataDir string) *Migrator {
	return &Migrator{
		pgDB:      pgDB,
		dataDir:   dataDir,
		usersPath: filepath.Join(dataDir, "users.bson"),
		cardsPath: filepath.Join(dataDir, "usercards.bson"),
		batchSize: 1000,
		stats: MigrationStats{
			Tables:    make(map[string]*TableStats),
			StartTime: time.Now(),
		},
	}
}

// Legacy constructor for backward compatibility
func NewLegacyMigrator(pgDB *bun.DB, usersPath, cardsPath string) *Migrator {
	return &Migrator{
		pgDB:      pgDB,
		usersPath: usersPath,
		cardsPath: cardsPath,
		batchSize: 1000,
	}
}

func (m *Migrator) MigrateAll(ctx context.Context) error {
	logProgress("Starting comprehensive BSON migration")
	logProgress(fmt.Sprintf("Data directory: %s", m.dataDir))

	// Initialize statistics tracking
	if m.stats.Tables == nil {
		m.stats.Tables = make(map[string]*TableStats)
	}
	m.stats.StartTime = time.Now()

	// Migration order preserves referential integrity
	// Import complete datasets from JSON first, then user data from BSON
	migrationSteps := []struct {
		name     string
		fileName string
		migrate  func(context.Context) error
	}{
		{"collections_json", "collections.json", m.ImportCollectionsFromJSON},
		{"cards_json", "cards.json", m.ImportCardsFromJSON},
		{"users", "users.bson", m.MigrateUsers},
		{"user_cards", "usercards.bson", m.MigrateUserCards},
		{"claims", "claims.bson", m.MigrateClaims},
		{"auctions", "auctions.bson", m.MigrateAuctions},
		{"user_effects", "usereffects.bson", m.MigrateUserEffects},
		{"user_quests", "userquests.bson", m.MigrateUserQuests},
		{"user_recipes", "userinventories.bson", m.MigrateUserInventories},
	}

	for _, step := range migrationSteps {
		logProgress(fmt.Sprintf("Starting migration step: %s", step.name))

		if err := step.migrate(ctx); err != nil {
			return fmt.Errorf("migration failed at step %s: %w", step.name, err)
		}

		logProgress(fmt.Sprintf("Completed migration step: %s", step.name))
	}

	// Finalize stats and generate report
	m.stats.EndTime = time.Now()
	if err := m.generateMigrationReport(); err != nil {
		slog.Error("Failed to generate migration report", "error", err)
	}

	logProgress("Migration completed successfully!")
	m.logFinalStats()
	return nil
}

// ImportCollectionsFromJSON imports collections from JSON file (complete dataset)
func (m *Migrator) ImportCollectionsFromJSON(ctx context.Context) error {
	// Check if file is in cmd/migrate directory or data directory
	var filePath string
	jsonPath := filepath.Join(m.dataDir, "collections.json")
	cmdPath := filepath.Join("bottemplate", "cmd", "migrate", "collections.json")

	if _, err := os.Stat(jsonPath); err == nil {
		filePath = jsonPath
	} else if _, err := os.Stat(cmdPath); err == nil {
		filePath = cmdPath
	} else {
		logProgress("collections.json not found, skipping JSON import")
		return nil
	}

	logProgress(fmt.Sprintf("Importing collections from JSON: %s", filePath))

	// Read and parse JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read collections.json: %w", err)
	}

	var jsonCollections []map[string]interface{}
	if err := json.Unmarshal(data, &jsonCollections); err != nil {
		return fmt.Errorf("failed to parse collections.json: %w", err)
	}

	var collections []*models.Collection
	now := time.Now()
	seenIDs := make(map[string]bool) // Track collection IDs
	duplicateCount := 0
	invalidCount := 0

	for i, jsonCol := range jsonCollections {
		colID := getString(jsonCol, "id")

		// Validate collection has proper ID field
		if colID == "" {
			invalidCount++
			logProgress(fmt.Sprintf("Invalid/missing collection ID in record %d (name: %s), skipping", i, getString(jsonCol, "name")))
			continue
		}

		// Check for duplicate collection IDs
		if seenIDs[colID] {
			duplicateCount++
			logProgress(fmt.Sprintf("Duplicate collection ID found: %s (record %d, name: %s), skipping", colID, i, getString(jsonCol, "name")))
			continue
		}
		seenIDs[colID] = true

		collection := &models.Collection{
			ID:         colID,
			Name:       cleanseString(getString(jsonCol, "name")),
			Origin:     getString(jsonCol, "origin"),
			Aliases:    getStringArray(jsonCol, "aliases"),
			Promo:      getBool(jsonCol, "promo"),
			Compressed: getBool(jsonCol, "compressed"),
			Fragments:  getBool(jsonCol, "fragments"),
			Tags:       getStringArray(jsonCol, "tags"),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		collections = append(collections, collection)

		// Batch insert when reaching batch size
		if len(collections) >= m.batchSize {
			if err := m.batchInsertCollections(ctx, collections); err != nil {
				return err
			}
			logProgress(fmt.Sprintf("Processed collections batch: %d", len(collections)))
			collections = collections[:0]
		}
	}

	// Insert remaining collections
	if len(collections) > 0 {
		if err := m.batchInsertCollections(ctx, collections); err != nil {
			return err
		}
	}

	totalProcessed := len(jsonCollections)
	totalImported := len(seenIDs)
	logProgress(fmt.Sprintf("Collections JSON import completed: %d total records, %d unique collections imported, %d duplicates skipped, %d invalid records skipped",
		totalProcessed, totalImported, duplicateCount, invalidCount))
	return nil
}

// ImportCardsFromJSON imports cards from JSON file (complete dataset)
func (m *Migrator) ImportCardsFromJSON(ctx context.Context) error {
	// Check if file is in cmd/migrate directory or data directory
	var filePath string
	jsonPath := filepath.Join(m.dataDir, "cards.json")
	cmdPath := filepath.Join("bottemplate", "cmd", "migrate", "cards.json")

	if _, err := os.Stat(jsonPath); err == nil {
		filePath = jsonPath
	} else if _, err := os.Stat(cmdPath); err == nil {
		filePath = cmdPath
	} else {
		logProgress("cards.json not found, skipping JSON import")
		return nil
	}

	logProgress(fmt.Sprintf("Importing cards from JSON: %s", filePath))

	// Read and parse JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read cards.json: %w", err)
	}

	var jsonCards []map[string]interface{}
	if err := json.Unmarshal(data, &jsonCards); err != nil {
		return fmt.Errorf("failed to parse cards.json: %w", err)
	}

	var cards []*models.Card
	now := time.Now()
	seenIDs := make(map[int64]bool)  // Track IDs across all batches
	batchIDs := make(map[int64]bool) // Track IDs within current batch
	duplicateCount := 0
	assignedIDCount := 0

	// Collect all existing IDs and find gaps to fill sequentially
	existingIDs := make(map[int64]bool)
	var maxID int64 = -1
	for _, jsonCard := range jsonCards {
		if hasValidID(jsonCard) {
			id := int64(getInt(jsonCard, "id"))
			existingIDs[id] = true
			if id > maxID {
				maxID = id
			}
		}
	}

	// Create list of missing IDs in sequence to fill gaps first
	var availableIDs []int64
	for i := int64(0); i <= maxID; i++ {
		if !existingIDs[i] {
			availableIDs = append(availableIDs, i)
		}
	}
	nextSequentialID := maxID + 1
	availableIDIndex := 0

	for i, jsonCard := range jsonCards {
		var cardID int64

		// Handle missing ID by filling gaps first, then sequential
		if !hasValidID(jsonCard) {
			if availableIDIndex < len(availableIDs) {
				// Fill gaps in sequence first
				cardID = availableIDs[availableIDIndex]
				availableIDIndex++
				logProgress(fmt.Sprintf("Filling gap: assigning ID %d to record %d (name: %s)", cardID, i, getString(jsonCard, "name")))
			} else {
				// Use sequential IDs after max if no more gaps
				cardID = nextSequentialID
				nextSequentialID++
				logProgress(fmt.Sprintf("Sequential assignment: assigning ID %d to record %d (name: %s)", cardID, i, getString(jsonCard, "name")))
			}
			assignedIDCount++
		} else {
			cardID = int64(getInt(jsonCard, "id"))
		}

		// Check for duplicates across all data
		if seenIDs[cardID] {
			duplicateCount++
			logProgress(fmt.Sprintf("Duplicate ID found: %d (record %d, name: %s), skipping", cardID, i, getString(jsonCard, "name")))
			continue
		}
		seenIDs[cardID] = true

		// Check for duplicates within current batch
		if batchIDs[cardID] {
			logProgress(fmt.Sprintf("Batch duplicate ID found: %d (record %d, name: %s), skipping", cardID, i, getString(jsonCard, "name")))
			continue
		}
		batchIDs[cardID] = true

		// Convert tags string to array
		var tags []string
		if tagStr := getString(jsonCard, "tags"); tagStr != "" {
			tags = []string{tagStr}
		}

		card := &models.Card{
			ID:        cardID,
			Name:      cleanseString(getString(jsonCard, "name")),
			Level:     getInt(jsonCard, "level"),
			Animated:  getBool(jsonCard, "animated"),
			ColID:     getString(jsonCard, "col"),
			Tags:      tags,
			CreatedAt: now,
			UpdatedAt: now,
		}
		cards = append(cards, card)

		// Batch insert when reaching batch size
		if len(cards) >= m.batchSize {
			logProgress(fmt.Sprintf("Inserting batch: %d cards (IDs: %d-%d)", len(cards), cards[0].ID, cards[len(cards)-1].ID))
			if err := m.batchInsertCards(ctx, cards); err != nil {
				return fmt.Errorf("batch insert failed for cards %d-%d: %w", cards[0].ID, cards[len(cards)-1].ID, err)
			}
			logProgress(fmt.Sprintf("Successfully inserted batch: %d cards", len(cards)))
			cards = cards[:0]
			batchIDs = make(map[int64]bool) // Reset batch tracking
		}
	}

	// Insert remaining cards
	if len(cards) > 0 {
		logProgress(fmt.Sprintf("Inserting final batch: %d cards (IDs: %d-%d)", len(cards), cards[0].ID, cards[len(cards)-1].ID))
		if err := m.batchInsertCards(ctx, cards); err != nil {
			return fmt.Errorf("final batch insert failed for cards %d-%d: %w", cards[0].ID, cards[len(cards)-1].ID, err)
		}
		logProgress(fmt.Sprintf("Successfully inserted final batch: %d cards", len(cards)))
	}

	totalProcessed := len(jsonCards)
	totalImported := len(seenIDs)
	gapsFilledCount := min(assignedIDCount, len(availableIDs))
	sequentialAssignedCount := assignedIDCount - gapsFilledCount

	logProgress(fmt.Sprintf("Cards JSON import completed: %d total records, %d unique cards imported, %d duplicates skipped",
		totalProcessed, totalImported, duplicateCount))
	logProgress(fmt.Sprintf("ID assignment: %d gaps filled in sequence, %d sequential IDs assigned after max",
		gapsFilledCount, sequentialAssignedCount))
	return nil
}

// Helper functions for JSON parsing
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok && val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt(data map[string]interface{}, key string) int {
	if val, ok := data[key]; ok && val != nil {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return -1 // Return -1 to indicate missing/invalid data instead of 0
}

// Helper to check if a record has a valid ID
func hasValidID(data map[string]interface{}) bool {
	id := getInt(data, "id")
	return id >= 0 // Valid IDs should be 0 or positive
}

func getBool(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok && val != nil {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func getStringArray(data map[string]interface{}, key string) []string {
	if val, ok := data[key]; ok && val != nil {
		if arr, ok := val.([]interface{}); ok {
			var result []string
			for _, item := range arr {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return []string{}
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
	duplicateUserCount := 0
	for _, mongoUser := range mongoUsers {
		pgUser := m.convertUser(mongoUser)

		if pgUser.DiscordID == "" {
			continue // Skip if discord_id is empty
		}

		// Check for duplicates and handle them
		if _, exists := discordIDUserMap[pgUser.DiscordID]; exists {
			duplicateUserCount++
			logProgress(fmt.Sprintf("Duplicate Discord ID found: %s (keeping latest record)", pgUser.DiscordID))
		}

		// Keep the latest occurrence (this is expected behavior for data deduplication)
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

	logProgress(fmt.Sprintf("User migration completed: %d total input records, %d unique users imported, %d duplicate Discord IDs handled",
		len(mongoUsers), len(users), duplicateUserCount))
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

	// Create a map for O(1) lookups and calculate stats
	validCardIDsMap := make(map[int64]bool)
	var minCardID, maxCardID int64 = 999999, 0
	for _, id := range validCardIDs {
		validCardIDsMap[id] = true
		if id < minCardID {
			minCardID = id
		}
		if id > maxCardID {
			maxCardID = id
		}
	}

	logProgress(fmt.Sprintf("Cards table stats: total=%d, range=%d-%d", len(validCardIDs), minCardID, maxCardID))

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
		for _, user := range users {
			_, singleErr := m.pgDB.NewInsert().Model(user).Exec(ctx)
			if singleErr != nil {
				slog.Error("Failed to insert user individually", "discord_id", user.DiscordID, "error", singleErr)
			}
		}
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

// logProgress logs progress messages following existing pattern
func logProgress(message string) {
	slog.Info(message, "service", "GoHYE Migration")
}

// Generic BSON file processing function following existing pattern
func (m *Migrator) processBSONFile(ctx context.Context, filePath string, processDoc func([]byte) error) error {
	// Check if file exists first
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logProgress(fmt.Sprintf("BSON file not found, skipping: %s", filePath))
		return nil // Skip missing files gracefully
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open BSON file %s: %w", filePath, err)
	}
	defer file.Close()

	// Get file size for safety checks
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()
	logProgress(fmt.Sprintf("Processing BSON file: %s (size: %d bytes)", filePath, fileSize))

	if fileSize == 0 {
		logProgress(fmt.Sprintf("File is empty, skipping: %s", filePath))
		return nil
	}

	reader := bufio.NewReader(file)
	docCount := 0
	bytesRead := int64(0)

	for bytesRead < fileSize {
		// Each BSON document starts with an int32 length
		lengthBytes := make([]byte, 4)
		n, err := io.ReadFull(reader, lengthBytes)
		if err == io.EOF {
			break // End of file reached
		}
		if err != nil {
			return fmt.Errorf("failed to read document length at byte %d: %w", bytesRead, err)
		}
		bytesRead += int64(n)

		length := int32(binary.LittleEndian.Uint32(lengthBytes))
		if length <= 4 || length > 16*1024*1024 { // Sanity check: 4 bytes minimum, 16MB maximum
			return fmt.Errorf("invalid document length: %d at byte position %d", length, bytesRead-4)
		}

		// The length includes the 4 bytes of the length itself
		docBytes := make([]byte, length-4)
		n, err = io.ReadFull(reader, docBytes)
		if err != nil {
			return fmt.Errorf("failed to read document bytes at byte %d: %w", bytesRead, err)
		}
		bytesRead += int64(n)

		// Prepend the lengthBytes to the docBytes
		fullDocBytes := append(lengthBytes, docBytes...)

		if err := processDoc(fullDocBytes); err != nil {
			logProgress(fmt.Sprintf("Warning: failed to process document %d at byte %d: %v", docCount+1, bytesRead-int64(length), err))
			// Continue processing instead of failing completely
			continue
		}
		docCount++

		// Progress logging every 1000 documents
		if docCount%1000 == 0 {
			logProgress(fmt.Sprintf("Processed %d documents from %s", docCount, filePath))
		}
	}

	logProgress(fmt.Sprintf("Completed processing %d documents from %s", docCount, filePath))
	return nil
}

// Migrate collections from BSON
func (m *Migrator) MigrateCollections(ctx context.Context) error {
	filePath := filepath.Join(m.dataDir, "collections.bson")
	logProgress(fmt.Sprintf("Starting collections migration from %s", filePath))

	var collections []*models.Collection

	processDoc := func(docBytes []byte) error {
		var mc MongoCollection
		if err := bson.Unmarshal(docBytes, &mc); err != nil {
			slog.Error("Failed to decode collection BSON", "error", err)
			return nil // Skip invalid documents
		}

		collection := m.convertCollection(mc)
		collections = append(collections, collection)

		// Batch insert when reaching batch size
		if len(collections) >= m.batchSize {
			if err := m.batchInsertCollections(ctx, collections); err != nil {
				return err
			}
			logProgress(fmt.Sprintf("Processed collections batch: %d", len(collections)))
			collections = collections[:0] // Reset slice
		}

		return nil
	}

	if err := m.processBSONFile(ctx, filePath, processDoc); err != nil {
		return err
	}

	// Insert remaining collections
	if len(collections) > 0 {
		if err := m.batchInsertCollections(ctx, collections); err != nil {
			return err
		}
	}

	logProgress("Collections migration completed")
	return nil
}

// Migrate cards from BSON
func (m *Migrator) MigrateCards(ctx context.Context) error {
	filePath := filepath.Join(m.dataDir, "cards.bson")
	logProgress(fmt.Sprintf("Starting cards migration from %s", filePath))

	var cards []*models.Card

	processDoc := func(docBytes []byte) error {
		var mc MongoCard
		if err := bson.Unmarshal(docBytes, &mc); err != nil {
			slog.Error("Failed to decode card BSON", "error", err)
			return nil // Skip invalid documents
		}

		card := m.convertMongoCard(mc)
		cards = append(cards, card)

		// Batch insert when reaching batch size
		if len(cards) >= m.batchSize {
			if err := m.batchInsertCards(ctx, cards); err != nil {
				return err
			}
			logProgress(fmt.Sprintf("Processed cards batch: %d", len(cards)))
			cards = cards[:0] // Reset slice
		}

		return nil
	}

	if err := m.processBSONFile(ctx, filePath, processDoc); err != nil {
		return err
	}

	// Insert remaining cards
	if len(cards) > 0 {
		if err := m.batchInsertCards(ctx, cards); err != nil {
			return err
		}
	}

	logProgress("Cards migration completed")
	return nil
}

// Migrate claims from BSON with array decomposition
func (m *Migrator) MigrateClaims(ctx context.Context) error {
	filePath := filepath.Join(m.dataDir, "claims.bson")
	logProgress(fmt.Sprintf("Starting claims migration from %s", filePath))

	var claims []*models.Claim

	processDoc := func(docBytes []byte) error {
		var mc MongoClaim
		if err := bson.Unmarshal(docBytes, &mc); err != nil {
			slog.Error("Failed to decode claim BSON", "error", err)
			return nil // Skip invalid documents
		}

		// Convert to PostgreSQL claims (array decomposition)
		claimRecords := m.convertClaims(mc)
		claims = append(claims, claimRecords...)

		// Batch insert when reaching batch size
		if len(claims) >= m.batchSize {
			if err := m.batchInsertClaims(ctx, claims); err != nil {
				return err
			}
			logProgress(fmt.Sprintf("Processed claims batch: %d", len(claims)))
			claims = claims[:0] // Reset slice
		}

		return nil
	}

	if err := m.processBSONFile(ctx, filePath, processDoc); err != nil {
		return err
	}

	// Insert remaining claims
	if len(claims) > 0 {
		if err := m.batchInsertClaims(ctx, claims); err != nil {
			return err
		}
	}

	logProgress("Claims migration completed")
	return nil
}

// Migrate auctions from BSON with relational enhancement
func (m *Migrator) MigrateAuctions(ctx context.Context) error {
	filePath := filepath.Join(m.dataDir, "auctions.bson")
	logProgress(fmt.Sprintf("Starting auctions migration from %s", filePath))

	var auctions []*models.Auction
	var auctionBids []*models.AuctionBid

	processDoc := func(docBytes []byte) error {
		var ma MongoAuction
		if err := bson.Unmarshal(docBytes, &ma); err != nil {
			slog.Error("Failed to decode auction BSON", "error", err)
			return nil // Skip invalid documents
		}

		// Convert to PostgreSQL auction and bids
		auction, bids := m.convertAuction(ma)
		auctions = append(auctions, auction)
		auctionBids = append(auctionBids, bids...)

		// Batch insert when reaching batch size
		if len(auctions) >= m.batchSize {
			if err := m.batchInsertAuctions(ctx, auctions); err != nil {
				return err
			}
			if err := m.batchInsertAuctionBids(ctx, auctionBids); err != nil {
				return err
			}
			logProgress(fmt.Sprintf("Processed auctions batch: %d auctions, %d bids", len(auctions), len(auctionBids)))
			auctions = auctions[:0]       // Reset slice
			auctionBids = auctionBids[:0] // Reset slice
		}

		return nil
	}

	if err := m.processBSONFile(ctx, filePath, processDoc); err != nil {
		return err
	}

	// Insert remaining auctions and bids
	if len(auctions) > 0 {
		if err := m.batchInsertAuctions(ctx, auctions); err != nil {
			return err
		}
	}
	if len(auctionBids) > 0 {
		if err := m.batchInsertAuctionBids(ctx, auctionBids); err != nil {
			return err
		}
	}

	logProgress("Auctions migration completed")
	return nil
}

// Migrate user effects from BSON
func (m *Migrator) MigrateUserEffects(ctx context.Context) error {
	filePath := filepath.Join(m.dataDir, "usereffects.bson")
	logProgress(fmt.Sprintf("Starting user effects migration from %s", filePath))

	var userEffects []*models.UserEffect

	processDoc := func(docBytes []byte) error {
		var me MongoUserEffect
		if err := bson.Unmarshal(docBytes, &me); err != nil {
			slog.Error("Failed to decode user effect BSON", "error", err)
			return nil // Skip invalid documents
		}

		effect := m.convertUserEffect(me)
		userEffects = append(userEffects, effect)

		// Batch insert when reaching batch size
		if len(userEffects) >= m.batchSize {
			if err := m.batchInsertUserEffects(ctx, userEffects); err != nil {
				return err
			}
			logProgress(fmt.Sprintf("Processed user effects batch: %d", len(userEffects)))
			userEffects = userEffects[:0] // Reset slice
		}

		return nil
	}

	if err := m.processBSONFile(ctx, filePath, processDoc); err != nil {
		return err
	}

	// Insert remaining user effects
	if len(userEffects) > 0 {
		if err := m.batchInsertUserEffects(ctx, userEffects); err != nil {
			return err
		}
	}

	logProgress("User effects migration completed")
	return nil
}

// Migrate user quests from BSON
func (m *Migrator) MigrateUserQuests(ctx context.Context) error {
	filePath := filepath.Join(m.dataDir, "userquests.bson")
	logProgress(fmt.Sprintf("Starting user quests migration from %s", filePath))

	var userQuests []*models.UserQuest

	processDoc := func(docBytes []byte) error {
		var mq MongoUserQuest
		if err := bson.Unmarshal(docBytes, &mq); err != nil {
			slog.Error("Failed to decode user quest BSON", "error", err)
			return nil // Skip invalid documents
		}

		quest := m.convertUserQuest(mq)
		userQuests = append(userQuests, quest)

		// Batch insert when reaching batch size
		if len(userQuests) >= m.batchSize {
			if err := m.batchInsertUserQuests(ctx, userQuests); err != nil {
				return err
			}
			logProgress(fmt.Sprintf("Processed user quests batch: %d", len(userQuests)))
			userQuests = userQuests[:0] // Reset slice
		}

		return nil
	}

	if err := m.processBSONFile(ctx, filePath, processDoc); err != nil {
		return err
	}

	// Insert remaining user quests
	if len(userQuests) > 0 {
		if err := m.batchInsertUserQuests(ctx, userQuests); err != nil {
			return err
		}
	}

	logProgress("User quests migration completed")
	return nil
}

// Migrate user inventories from BSON (actually user recipes)
func (m *Migrator) MigrateUserInventories(ctx context.Context) error {
	filePath := filepath.Join(m.dataDir, "userinventories.bson")
	logProgress(fmt.Sprintf("Starting user recipes migration from %s", filePath))

	var userRecipes []*models.UserRecipe
	seenKeys := make(map[string]bool)  // Track (user_id, item_id) across all batches
	batchKeys := make(map[string]bool) // Track (user_id, item_id) within current batch
	duplicateCount := 0

	processDoc := func(docBytes []byte) error {
		var mi MongoUserInventory
		if err := bson.Unmarshal(docBytes, &mi); err != nil {
			slog.Error("Failed to decode user inventory BSON", "error", err)
			return nil // Skip invalid documents
		}

		// Skip entries with no cards
		if len(mi.Cards) == 0 {
			logProgress(fmt.Sprintf("Skipping user inventory with no cards: user=%s, item=%s", mi.UserID, mi.ItemID))
			return nil
		}

		// Create unique key for (user_id, item_id) combination
		recipeKey := fmt.Sprintf("%s:%s", mi.UserID, mi.ItemID)

		// Check for duplicates across all data
		if seenKeys[recipeKey] {
			duplicateCount++
			logProgress(fmt.Sprintf("Duplicate user recipe found: user=%s, item=%s, skipping", mi.UserID, mi.ItemID))
			return nil
		}
		seenKeys[recipeKey] = true

		// Check for duplicates within current batch
		if batchKeys[recipeKey] {
			logProgress(fmt.Sprintf("Batch duplicate user recipe found: user=%s, item=%s, skipping", mi.UserID, mi.ItemID))
			return nil
		}
		batchKeys[recipeKey] = true

		// Convert to PostgreSQL user recipe (preserves specific card information)
		recipe := m.convertUserInventory(mi)
		userRecipes = append(userRecipes, recipe)

		// Batch insert when reaching batch size
		if len(userRecipes) >= m.batchSize {
			if err := m.batchInsertUserRecipes(ctx, userRecipes); err != nil {
				return err
			}
			logProgress(fmt.Sprintf("Processed user recipes batch: %d", len(userRecipes)))
			userRecipes = userRecipes[:0]     // Reset slice
			batchKeys = make(map[string]bool) // Reset batch tracking
		}

		return nil
	}

	if err := m.processBSONFile(ctx, filePath, processDoc); err != nil {
		return err
	}

	// Insert remaining user recipes
	if len(userRecipes) > 0 {
		if err := m.batchInsertUserRecipes(ctx, userRecipes); err != nil {
			return err
		}
	}

	totalImported := len(seenKeys)
	logProgress(fmt.Sprintf("User recipes migration completed: %d unique recipes imported, %d duplicates skipped",
		totalImported, duplicateCount))
	return nil
}

// Batch insert helper functions following existing patterns

func (m *Migrator) batchInsertCollections(ctx context.Context, collections []*models.Collection) error {
	_, err := m.pgDB.NewInsert().
		Model(&collections).
		On("CONFLICT (id) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("origin = EXCLUDED.origin").
		Set("aliases = EXCLUDED.aliases").
		Set("promo = EXCLUDED.promo").
		Set("compressed = EXCLUDED.compressed").
		Set("fragments = EXCLUDED.fragments").
		Set("tags = EXCLUDED.tags").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (m *Migrator) batchInsertCards(ctx context.Context, cards []*models.Card) error {
	_, err := m.pgDB.NewInsert().
		Model(&cards).
		On("CONFLICT (id) DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("level = EXCLUDED.level").
		Set("animated = EXCLUDED.animated").
		Set("col_id = EXCLUDED.col_id").
		Set("tags = EXCLUDED.tags").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (m *Migrator) batchInsertClaims(ctx context.Context, claims []*models.Claim) error {
	_, err := m.pgDB.NewInsert().
		Model(&claims).
		Exec(ctx)
	return err
}

func (m *Migrator) batchInsertAuctions(ctx context.Context, auctions []*models.Auction) error {
	_, err := m.pgDB.NewInsert().
		Model(&auctions).
		On("CONFLICT (auction_id) DO UPDATE").
		Set("card_id = EXCLUDED.card_id").
		Set("seller_id = EXCLUDED.seller_id").
		Set("start_price = EXCLUDED.start_price").
		Set("current_price = EXCLUDED.current_price").
		Set("status = EXCLUDED.status").
		Set("end_time = EXCLUDED.end_time").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (m *Migrator) batchInsertAuctionBids(ctx context.Context, auctionBids []*models.AuctionBid) error {
	_, err := m.pgDB.NewInsert().
		Model(&auctionBids).
		Exec(ctx)
	return err
}

func (m *Migrator) batchInsertUserEffects(ctx context.Context, userEffects []*models.UserEffect) error {
	_, err := m.pgDB.NewInsert().
		Model(&userEffects).
		Exec(ctx)
	return err
}

func (m *Migrator) batchInsertUserQuests(ctx context.Context, userQuests []*models.UserQuest) error {
	_, err := m.pgDB.NewInsert().
		Model(&userQuests).
		Exec(ctx)
	return err
}

func (m *Migrator) batchInsertUserRecipes(ctx context.Context, userRecipes []*models.UserRecipe) error {
	// No ON CONFLICT needed since we handle duplicates at application level
	_, err := m.pgDB.NewInsert().
		Model(&userRecipes).
		Exec(ctx)
	return err
}

func (m *Migrator) batchInsertUserInventories(ctx context.Context, userInventories []*models.UserInventory) error {
	_, err := m.pgDB.NewInsert().
		Model(&userInventories).
		On("CONFLICT (user_id, item_id) DO UPDATE").
		Set("amount = EXCLUDED.amount").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

// generateMigrationReport creates a detailed JSON report of the migration
func (m *Migrator) generateMigrationReport() error {
	timestamp := time.Now().Format("20060102_150405")
	reportFile := filepath.Join(".", fmt.Sprintf("migration_report_%s.json", timestamp))

	file, err := os.Create(reportFile)
	if err != nil {
		return fmt.Errorf("failed to create migration report file: %w", err)
	}
	defer file.Close()

	// Calculate final totals
	m.stats.TotalProcessed = 0
	m.stats.TotalSkipped = 0
	m.stats.TotalErrors = 0

	for _, tableStats := range m.stats.Tables {
		m.stats.TotalProcessed += tableStats.Processed
		m.stats.TotalSkipped += tableStats.Skipped
		m.stats.TotalErrors += tableStats.Errors
	}

	// Write JSON report
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(m.stats); err != nil {
		return fmt.Errorf("failed to write migration report: %w", err)
	}

	slog.Info("Migration report generated", "file", reportFile)
	return nil
}

// logFinalStats logs a summary of migration statistics
func (m *Migrator) logFinalStats() {
	duration := m.stats.EndTime.Sub(m.stats.StartTime)

	slog.Info("Migration completed",
		"duration", duration,
		"total_processed", m.stats.TotalProcessed,
		"total_skipped", m.stats.TotalSkipped,
		"total_errors", m.stats.TotalErrors)

	// Log table-specific stats
	for tableName, stats := range m.stats.Tables {
		slog.Info("Table migration stats",
			"table", tableName,
			"processed", stats.Processed,
			"successful", stats.Successful,
			"skipped", stats.Skipped,
			"errors", stats.Errors)
	}
}

// Helper methods for statistics tracking

func (m *Migrator) initTableStats(tableName string) {
	if m.stats.Tables == nil {
		m.stats.Tables = make(map[string]*TableStats)
	}
	m.stats.Tables[tableName] = &TableStats{
		TableName:      tableName,
		SkippedRecords: []SkippedRecord{},
		ErrorRecords:   []ErrorRecord{},
	}
}

func (m *Migrator) recordProcessed(tableName string) {
	if stats, exists := m.stats.Tables[tableName]; exists {
		stats.Processed++
	}
}

func (m *Migrator) recordSuccessful(tableName string) {
	if stats, exists := m.stats.Tables[tableName]; exists {
		stats.Successful++
	}
}

func (m *Migrator) recordSkipped(tableName, reason, data string) {
	if stats, exists := m.stats.Tables[tableName]; exists {
		stats.Skipped++
		stats.SkippedRecords = append(stats.SkippedRecords, SkippedRecord{
			Reason:    reason,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
}

func (m *Migrator) recordError(tableName, errorMsg, data string) {
	if stats, exists := m.stats.Tables[tableName]; exists {
		stats.Errors++
		stats.ErrorRecords = append(stats.ErrorRecords, ErrorRecord{
			Error:     errorMsg,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
}
