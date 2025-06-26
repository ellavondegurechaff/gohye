// migration/cards.go
package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
)

// Add the convertTags function
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

func (m *Migrator) MigrateCardsFromJSON(ctx context.Context) error {
	logProgress("Starting card migration process")

	// Initialize repositories
	collectionRepo := repositories.NewCollectionRepository(m.pgDB)
	cardRepo := repositories.NewCardRepository(m.pgDB)

	// Read and process collections
	var jsonCollections []JSONCollection
	if err := readJSONFile("collections.json", &jsonCollections); err != nil {
		return fmt.Errorf("failed to read collections: %w", err)
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
			Fragments:  false, // Set default value for fragments
			Tags:       jc.Tags,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})
	}

	// Import collections
	if err := collectionRepo.BulkCreate(ctx, collections); err != nil {
		return fmt.Errorf("failed to import collections: %w", err)
	}

	// Read and process cards
	var jsonCards []JSONCard
	if err := readJSONFile("cards.json", &jsonCards); err != nil {
		return fmt.Errorf("failed to read cards: %w", err)
	}

	// Convert cards using IDs directly from JSON
	cards := make([]*models.Card, 0, len(jsonCards))
	processedIDs := make(map[int64]bool)

	for _, jc := range jsonCards {
		if processedIDs[jc.ID] {
			logProgress(fmt.Sprintf("Warning: Duplicate ID found for card: %s (ID: %d)", jc.Name, jc.ID))
			continue
		}

		processedIDs[jc.ID] = true
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

	// Process in batches
	batchSize := m.batchSize
	totalImported := 0

	for i := 0; i < len(cards); i += batchSize {
		end := i + batchSize
		if end > len(cards) {
			end = len(cards)
		}

		batch := cards[i:end]
		importedCount, err := cardRepo.BulkCreate(ctx, batch)
		if err != nil {
			return fmt.Errorf("failed to import cards batch %d-%d: %w", i, end, err)
		}

		totalImported += importedCount
		logProgress(fmt.Sprintf("Progress: Imported %d/%d cards", totalImported, len(cards)))
	}

	return nil
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
