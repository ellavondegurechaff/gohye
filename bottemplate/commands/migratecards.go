package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var MigrateCards = discord.SlashCommandCreate{
	Name:        "migratecards",
	Description: "Migrate cards and collections from JSON files to database",
}

type JSONCard struct {
	Name     string      `json:"name"`
	Level    int         `json:"level"`
	Animated bool        `json:"animated"`
	Col      string      `json:"col"`
	ID       int64       `json:"id"`
	Tags     interface{} `json:"tags"`
	Added    *string     `json:"added,omitempty"`
}

type JSONCollection struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Origin     string   `json:"origin"`
	Aliases    []string `json:"aliases"`
	Promo      bool     `json:"promo"`
	Compressed bool     `json:"compressed"`
	Tags       []string `json:"tags"`
}

func logProgress(message string) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] %s\n", timestamp, message)
}

func readJSONFile(filename string, v interface{}) error {
	logProgress(fmt.Sprintf("Reading %s...", filename))

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	logProgress(fmt.Sprintf("Successfully read %s", filename))
	return nil
}

func convertTags(rawTags interface{}) []string {
	if rawTags == nil {
		return []string{}
	}

	switch v := rawTags.(type) {
	case string:
		if v == "" {
			return []string{}
		}
		return []string{v}
	case []string:
		return v
	case []interface{}:
		tags := make([]string, len(v))
		for i, tag := range v {
			if str, ok := tag.(string); ok {
				tags[i] = str
			}
		}
		return tags
	default:
		return []string{}
	}
}

func MigrateCardsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		startTime := time.Now()
		logProgress("Starting migration process")

		// Send initial message
		err := e.CreateMessage(discord.MessageCreate{
			Content: "🔄 Starting migration process...",
		})
		if err != nil {
			return fmt.Errorf("failed to send initial message: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Initialize database schema
		logProgress("Initializing database schema...")
		if err := b.DB.InitializeSchema(ctx); err != nil {
			errMsg := fmt.Sprintf("❌ Failed to initialize database schema: %v", err)
			logProgress(errMsg)
			return e.CreateMessage(discord.MessageCreate{Content: errMsg})
		}

		// Initialize repositories using BunDB
		collectionRepo := repositories.NewCollectionRepository(b.DB.BunDB())
		cardRepo := repositories.NewCardRepository(b.DB.BunDB(), b.SpacesService)

		// Read and process collections
		var jsonCollections []JSONCollection
		if err := readJSONFile("collections.json", &jsonCollections); err != nil {
			errMsg := fmt.Sprintf("❌ %v", err)
			logProgress(errMsg)
			return e.CreateMessage(discord.MessageCreate{Content: errMsg})
		}

		// Convert collections
		collections := make([]*models.Collection, 0, len(jsonCollections))
		for _, jc := range jsonCollections {
			collections = append(collections, &models.Collection{
				ID:         jc.ID,
				Name:       jc.Name,
				Origin:     jc.Origin,
				Aliases:    jc.Aliases,
				Promo:      jc.Promo,
				Compressed: jc.Compressed,
				Tags:       jc.Tags,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			})
		}

		// Import collections
		if err := collectionRepo.BulkCreate(ctx, collections); err != nil {
			errMsg := fmt.Sprintf("❌ Failed to import collections: %v", err)
			logProgress(errMsg)
			return e.CreateMessage(discord.MessageCreate{Content: errMsg})
		}

		// Read and process cards
		var jsonCards []JSONCard
		if err := readJSONFile("cards.json", &jsonCards); err != nil {
			errMsg := fmt.Sprintf("❌ %v", err)
			logProgress(errMsg)
			return e.CreateMessage(discord.MessageCreate{Content: errMsg})
		}

		// Convert cards using IDs directly from JSON
		cards := make([]*models.Card, 0, len(jsonCards))
		nameCount := make(map[string]int)
		processedIDs := make(map[int64]bool)

		for _, jc := range jsonCards {
			// Skip if we've already processed this ID
			if processedIDs[jc.ID] {
				logProgress(fmt.Sprintf("Warning: Duplicate ID found for card: %s (ID: %d)", jc.Name, jc.ID))
				continue
			}

			processedIDs[jc.ID] = true
			nameCount[jc.Name]++

			cards = append(cards, &models.Card{
				ID:        jc.ID,
				Name:      jc.Name,
				Level:     jc.Level,
				Animated:  jc.Animated,
				ColID:     jc.Col,
				Tags:      convertTags(jc.Tags),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
		}

		// Process in batches to avoid memory issues
		batchSize := 1000
		totalImported := 0

		for i := 0; i < len(cards); i += batchSize {
			end := i + batchSize
			if end > len(cards) {
				end = len(cards)
			}

			batch := cards[i:end]
			importedCount, err := cardRepo.BulkCreate(ctx, batch)
			if err != nil {
				errMsg := fmt.Sprintf("❌ Failed to import cards batch %d-%d: %v", i, end, err)
				logProgress(errMsg)
				return e.CreateMessage(discord.MessageCreate{Content: errMsg})
			}

			totalImported += importedCount
			logProgress(fmt.Sprintf("Progress: Imported %d/%d cards", totalImported, len(cards)))
		}

		// Calculate duplicate statistics
		var duplicateNames int
		duplicatesList := make([]string, 0)
		for name, count := range nameCount {
			if count > 1 {
				duplicateNames++
				duplicatesList = append(duplicatesList, fmt.Sprintf("'%s' appears %d times", name, count))
			}
		}

		// Calculate duration
		duration := time.Since(startTime)

		// Build the success message
		var messageBuilder strings.Builder
		messageBuilder.WriteString("✅ Migration completed successfully!\n\n")
		messageBuilder.WriteString("📊 Statistics:\n")
		messageBuilder.WriteString(fmt.Sprintf("- Imported %d collections\n", len(collections)))
		messageBuilder.WriteString(fmt.Sprintf("- Card Migration Results:\n"))
		messageBuilder.WriteString(fmt.Sprintf("  • Total cards processed: %d\n", len(cards)))
		messageBuilder.WriteString(fmt.Sprintf("  • Successfully imported: %d\n", totalImported))
		messageBuilder.WriteString(fmt.Sprintf("  • Cards with duplicate names: %d\n", duplicateNames))
		messageBuilder.WriteString(fmt.Sprintf("⏱️ Time taken: %v\n", duration.Round(time.Millisecond)))

		// Add duplicate details if any exist
		if duplicateNames > 0 {
			messageBuilder.WriteString("\n📝 Cards with multiple versions:\n")
			maxDuplicatesToShow := 10
			for i, dupInfo := range duplicatesList {
				if i >= maxDuplicatesToShow {
					remaining := len(duplicatesList) - maxDuplicatesToShow
					messageBuilder.WriteString(fmt.Sprintf("...and %d more cards with duplicates\n", remaining))
					break
				}
				messageBuilder.WriteString("• " + dupInfo + "\n")
			}
		}

		successMsg := messageBuilder.String()
		logProgress(successMsg)

		// Split message if it's too long for Discord
		const maxMessageLength = 2000
		if len(successMsg) > maxMessageLength {
			// Send first part
			err = e.CreateMessage(discord.MessageCreate{
				Content: successMsg[:maxMessageLength],
			})
			if err != nil {
				return err
			}

			// Send remaining parts
			remaining := successMsg[maxMessageLength:]
			for len(remaining) > 0 {
				sendLength := maxMessageLength
				if len(remaining) < maxMessageLength {
					sendLength = len(remaining)
				}
				err = e.CreateMessage(discord.MessageCreate{
					Content: remaining[:sendLength],
				})
				if err != nil {
					return err
				}
				remaining = remaining[sendLength:]
			}
			return nil
		}

		// Send single message if not too long
		return e.CreateMessage(discord.MessageCreate{
			Content: successMsg,
		})
	}
}
