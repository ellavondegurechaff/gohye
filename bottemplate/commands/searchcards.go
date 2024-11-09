package commands

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/paginator"
)

var SearchCards = discord.SlashCommandCreate{
	Name:        "searchcards",
	Description: "🔍 Search through the card collection with various filters",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "name",
			Description: "Search by card name",
			Required:    false,
		},
		discord.ApplicationCommandOptionInt{
			Name:        "id",
			Description: "Search by card ID",
			Required:    false,
		},
		discord.ApplicationCommandOptionInt{
			Name:        "level",
			Description: "Filter by card level (1-5)",
			Required:    false,
			Choices: []discord.ApplicationCommandOptionChoiceInt{
				{Name: "1", Value: 1},
				{Name: "2", Value: 2},
				{Name: "3", Value: 3},
				{Name: "4", Value: 4},
				{Name: "5", Value: 5},
			},
		},
		discord.ApplicationCommandOptionString{
			Name:        "collection",
			Description: "Filter by collection ID",
			Required:    false,
		},
		discord.ApplicationCommandOptionString{
			Name:        "type",
			Description: "Filter by card type",
			Required:    false,
			Choices: []discord.ApplicationCommandOptionChoiceString{
				{Name: "👯‍♀️ Girl Groups", Value: "girlgroups"},
				{Name: "👯‍♂️ Boy Groups", Value: "boygroups"},
			},
		},
		discord.ApplicationCommandOptionBool{
			Name:        "animated",
			Description: "Filter animated cards only",
			Required:    false,
		},
	},
}

const (
	cardsPerPage    = 10
	searchTimeout   = 10 * time.Second
	cacheExpiration = 5 * time.Minute
)

type searchCache struct {
	mu    sync.RWMutex
	cache map[string]*cacheEntry
}

type cacheEntry struct {
	results    []*models.Card
	totalCount int
	timestamp  time.Time
}

var cardSearchCache = &searchCache{
	cache: make(map[string]*cacheEntry),
}

func (sc *searchCache) get(key string) (*cacheEntry, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	entry, exists := sc.cache[key]
	if !exists {
		return nil, false
	}

	// Check if cache entry has expired
	if time.Since(entry.timestamp) > cacheExpiration {
		delete(sc.cache, key)
		return nil, false
	}

	return entry, true
}

func (sc *searchCache) set(key string, cards []*models.Card, totalCount int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.cache[key] = &cacheEntry{
		results:    cards,
		totalCount: totalCount,
		timestamp:  time.Now(),
	}
}

func SearchCardsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		// Extract search filters from command options
		filters := repositories.SearchFilters{
			Name:       strings.TrimSpace(e.SlashCommandInteractionData().String("name")),
			ID:         int64(e.SlashCommandInteractionData().Int("id")),
			Level:      int(e.SlashCommandInteractionData().Int("level")),
			Collection: strings.TrimSpace(e.SlashCommandInteractionData().String("collection")),
			Type:       e.SlashCommandInteractionData().String("type"),
			Animated:   e.SlashCommandInteractionData().Bool("animated"),
		}

		// Generate cache key
		cacheKey := generateCacheKey(filters)

		// Try to get results from cache first
		if entry, exists := cardSearchCache.get(cacheKey); exists {
			return createPaginator(b, e, entry.results, entry.totalCount, filters)
		}

		// Set timeout context
		ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
		defer cancel()

		// Use a channel for handling timeouts gracefully
		resultChan := make(chan struct {
			cards []*models.Card
			count int
			err   error
		})

		go func() {
			cards, totalCount, err := b.CardRepository.Search(ctx, filters, 0, cardsPerPage)
			resultChan <- struct {
				cards []*models.Card
				count int
				err   error
			}{cards, totalCount, err}
		}()

		// Wait for results or timeout
		select {
		case result := <-resultChan:
			if result.err != nil {
				return sendErrorEmbed(e, "Search Failed", result.err)
			}

			if len(result.cards) == 0 {
				return sendNoResultsEmbed(e)
			}

			// Cache the results
			cardSearchCache.set(cacheKey, result.cards, result.count)

			return createPaginator(b, e, result.cards, result.count, filters)

		case <-ctx.Done():
			return sendErrorEmbed(e, "Search Timeout", fmt.Errorf("search took too long to complete"))
		}
	}
}

// Helper function to generate cache key
func generateCacheKey(filters repositories.SearchFilters) string {
	return fmt.Sprintf("%s:%d:%d:%s:%s:%v",
		filters.Name,
		filters.ID,
		filters.Level,
		filters.Collection,
		filters.Type,
		filters.Animated,
	)
}

// Separate paginator creation logic
func createPaginator(b *bottemplate.Bot, e *handler.CommandEvent, initialCards []*models.Card, totalCount int, filters repositories.SearchFilters) error {
	totalPages := int(math.Ceil(float64(totalCount) / float64(cardsPerPage)))

	return b.Paginator.Create(e.Respond, paginator.Pages{
		ID:      e.ID().String(),
		Creator: e.User().ID,
		PageFunc: func(page int, embed *discord.EmbedBuilder) {
			// Try to get page from cache first
			cacheKey := fmt.Sprintf("%s:page:%d", generateCacheKey(filters), page)
			if entry, exists := cardSearchCache.get(cacheKey); exists {
				description := buildSearchDescription(entry.results, filters, page+1, totalCount, totalPages)
				embed.
					SetTitle("🔍 Card Search Results").
					SetDescription(description).
					SetColor(0x00FF00).
					SetFooter("Use the buttons below to navigate or refine your search", "")
				return
			}

			// If not in cache, fetch from database
			offset := page * cardsPerPage
			pageCards, _, _ := b.CardRepository.Search(context.Background(), filters, offset, cardsPerPage)

			// Cache the page results
			cardSearchCache.set(cacheKey, pageCards, totalCount)

			description := buildSearchDescription(pageCards, filters, page+1, totalCount, totalPages)
			embed.
				SetTitle("🔍 Card Search Results").
				SetDescription(description).
				SetColor(0x00FF00).
				SetFooter("Use the buttons below to navigate or refine your search", "")
		},
		Pages:      totalPages,
		ExpireMode: paginator.ExpireModeAfterLastUsage,
	}, false)
}

func buildSearchDescription(cards []*models.Card, filters repositories.SearchFilters, currentPage, totalCount, totalPages int) string {
	var description strings.Builder
	description.WriteString("```md\n# Search Results\n")

	// Add active filters section
	if hasActiveFilters(filters) {
		description.WriteString("\n## Active Filters\n")
		if filters.Name != "" {
			description.WriteString(fmt.Sprintf("* Name: %s\n", filters.Name))
		}
		if filters.ID != 0 {
			description.WriteString(fmt.Sprintf("* ID: %d\n", filters.ID))
		}
		if filters.Level != 0 {
			description.WriteString(fmt.Sprintf("* Level: %d ⭐\n", filters.Level))
		}
		if filters.Collection != "" {
			description.WriteString(fmt.Sprintf("* Collection: %s\n", filters.Collection))
		}
		if filters.Type != "" {
			description.WriteString(fmt.Sprintf("* Type: %s\n", formatCardType(filters.Type)))
		}
		if filters.Animated {
			description.WriteString("* Animated Only: Yes\n")
		}
	}

	description.WriteString("\n## Cards\n")
	for _, card := range cards {
		// Format level with stars and remove double brackets and card ID
		description.WriteString(fmt.Sprintf("* %d ⭐ %s [%s]\n",
			card.Level,
			utils.FormatCardName(card.Name),
			strings.Trim(utils.FormatCollectionName(card.ColID), "[]"), // Remove double brackets
		))
	}

	description.WriteString(fmt.Sprintf("\n> Page %d of %d (%d total cards)\n", currentPage, totalPages, totalCount))
	description.WriteString("```")

	return description.String()
}

func sendErrorEmbed(e *handler.CommandEvent, title string, err error) error {
	_, err2 := e.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds: &[]discord.Embed{
			{
				Title:       "❌ " + title,
				Description: fmt.Sprintf("```diff\n- Error: %v\n```", err),
				Color:       0xFF0000,
			},
		},
	})
	return err2
}

func sendNoResultsEmbed(e *handler.CommandEvent) error {
	_, err := e.UpdateInteractionResponse(discord.MessageUpdate{
		Embeds: &[]discord.Embed{
			{
				Title:       "❌ No Results Found",
				Description: "```diff\n- No cards match your search criteria\n```",
				Color:       0xFF0000,
				Footer: &discord.EmbedFooter{
					Text: "Try different search terms or filters",
				},
			},
		},
	})
	return err
}

func hasActiveFilters(filters repositories.SearchFilters) bool {
	return filters.Name != "" ||
		filters.ID != 0 ||
		filters.Level != 0 ||
		filters.Collection != "" ||
		filters.Type != "" ||
		filters.Animated
}

func formatCardType(cardType string) string {
	switch cardType {
	case "girlgroups":
		return "👯‍♀️ Girl Groups"
	case "boygroups":
		return "👯‍♂️ Boy Groups"
	case "soloist":
		return "👤 Solo Artist"
	default:
		return cardType
	}
}
