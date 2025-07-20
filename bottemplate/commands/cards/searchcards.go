package cards

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/paginator"
)

var SearchCards = discord.SlashCommandCreate{
	Name:        "searchcards",
	Description: "🔍 Search through the card collection with advanced query syntax",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionString{
			Name:        "query",
			Description: "Search query (supports level=3, !promo, #girlgroups, etc.)",
			Required:    false,
		},
	},
}

type cacheEntry struct {
	results    []*models.Card
	totalCount int
	timestamp  time.Time
}

type searchCache struct {
	mu    sync.RWMutex
	cache map[string]*cacheEntry
}

var cardSearchCache = &searchCache{
	cache: make(map[string]*cacheEntry),
}

func (sc *searchCache) get(key string) (*cacheEntry, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	entry, exists := sc.cache[key]
	if !exists {
		log.Printf("Cache miss for key: %s", key)
		return nil, false
	}

	// Check if cache entry has expired
	if time.Since(entry.timestamp) > utils.CacheExpiration {
		log.Printf("Cache expired for key: %s", key)
		delete(sc.cache, key)
		return nil, false
	}

	log.Printf("Cache hit for key: %s (expires in: %v)", key,
		utils.CacheExpiration-time.Since(entry.timestamp))
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
	log.Printf("Cached value for key: %s (expires: %s)", key,
		time.Now().Add(utils.CacheExpiration))
}

func SearchCardsHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(event *handler.CommandEvent) error {
		// Get single query parameter
		query := strings.TrimSpace(event.SlashCommandInteractionData().String("query"))

		// Use enhanced search filters for parsing
		var repoFilters repositories.SearchFilters
		if query != "" {
			// Parse using enhanced parser for advanced query syntax
			enhancedFilters := utils.ParseSearchQuery(query)

			// Convert enhanced filters to repository filters
			repoFilters = repositories.SearchFilters{
				Name:       enhancedFilters.Name,
				Level:      getFirstLevel(enhancedFilters.Levels),
				Collection: getFirstCollection(enhancedFilters.Collections),
				Animated:   enhancedFilters.Animated,
			}
		} else {
			// Empty query - show all cards with basic pagination
			repoFilters = repositories.SearchFilters{}
		}

		// Generate cache key
		cacheKey := generateCacheKey(repoFilters)

		// Try to get results from cache first
		if entry, exists := cardSearchCache.get(cacheKey); exists {
			return createPaginator(b, event, entry.results, entry.totalCount, repoFilters)
		}

		// Set timeout context
		ctx, cancel := context.WithTimeout(context.Background(), utils.SearchTimeout)
		defer cancel()

		// Use a channel for handling timeouts gracefully
		resultChan := make(chan struct {
			cards []*models.Card
			count int
			err   error
		})

		go func() {
			cards, totalCount, err := b.CardRepository.Search(ctx, repoFilters, 0, utils.CardsPerPage)
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
				return utils.EH.UpdateInteractionResponse(event, "Search Failed", result.err.Error())
			}

			if len(result.cards) == 0 {
				return utils.EH.UpdateInteractionResponse(event, "No Results Found", "No cards match your search criteria")
			}

			// Sort results by relevance if name filter is present
			if repoFilters.Name != "" {
				sortCardsByRelevance(result.cards, repoFilters.Name)
			}

			// Cache the results
			cardSearchCache.set(cacheKey, result.cards, result.count)

			return createPaginator(b, event, result.cards, result.count, repoFilters)

		case <-ctx.Done():
			return utils.EH.UpdateInteractionResponse(event, "Search Timeout", "Search took too long to complete")
		}
	}
}

// getFirstLevel extracts the first level from enhanced filters, or 0 if none
func getFirstLevel(levels []int) int {
	if len(levels) > 0 {
		return levels[0]
	}
	return 0
}

// getFirstCollection extracts the first collection from enhanced filters, or empty string if none
func getFirstCollection(collections []string) string {
	if len(collections) > 0 {
		return collections[0]
	}
	return ""
}

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

func createPaginator(b *bottemplate.Bot, e *handler.CommandEvent, initialCards []*models.Card, totalCount int, filters repositories.SearchFilters) error {
	// Ensure totalCount is at least the length of initial cards
	if totalCount < len(initialCards) {
		totalCount = len(initialCards)
	}

	// Calculate total pages, ensuring at least 1 page if there are results
	totalPages := int(math.Max(1, math.Ceil(float64(totalCount)/float64(utils.CardsPerPage))))

	return b.Paginator.Create(e.Respond, paginator.Pages{
		ID:      e.ID().String(),
		Creator: e.User().ID,
		PageFunc: func(page int, embed *discord.EmbedBuilder) {
			// Ensure page number is valid
			if page >= totalPages {
				page = totalPages - 1
			}

			// Try to get page from cache first
			cacheKey := fmt.Sprintf("%s:page:%d", generateCacheKey(filters), page)
			if entry, exists := cardSearchCache.get(cacheKey); exists {
				description := buildSearchDescription(entry.results, filters, page+1, totalCount, totalPages)
				embed.
					SetTitle("🔍 Card Search Results").
					SetDescription(description).
					SetColor(0x000000).
					SetFooter(fmt.Sprintf("Page %d/%d • Total: %d", page+1, totalPages, totalCount), "")
				return
			}

			// If not in cache, fetch from database
			offset := page * utils.CardsPerPage
			pageCards, _, _ := b.CardRepository.Search(context.Background(), filters, offset, utils.CardsPerPage)

			// Sort results by relevance if name filter is present
			if filters.Name != "" {
				sortCardsByRelevance(pageCards, filters.Name)
			}

			// Cache the page results
			cardSearchCache.set(cacheKey, pageCards, totalCount)

			description := buildSearchDescription(pageCards, filters, page+1, totalCount, totalPages)
			embed.
				SetTitle("🔍 Card Search Results").
				SetDescription(description).
				SetColor(0x000000).
				SetFooter(fmt.Sprintf("Page %d/%d • Total: %d", page+1, totalPages, totalCount), "")
		},
		Pages:      totalPages,
		ExpireMode: paginator.ExpireModeAfterLastUsage,
	}, false)
}

func buildSearchDescription(cards []*models.Card, filters repositories.SearchFilters, _, _, _ int) string {
	var description strings.Builder
	description.WriteString("```md\n")

	// Add active filters section
	if hasActiveFilters(filters) {
		description.WriteString("## Active Filters\n")
		if filters.Name != "" {
			description.WriteString(fmt.Sprintf("* Name: %s\n", filters.Name))
		}
		if filters.ID != 0 {
			description.WriteString(fmt.Sprintf("* ID: %d\n", filters.ID))
		}
		if filters.Level != 0 {
			description.WriteString(fmt.Sprintf("* Level: %s\n", strings.Repeat("⭐", filters.Level)))
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
		description.WriteString("\n")
	}

	description.WriteString("## Cards\n")
	if len(cards) == 0 {
		description.WriteString("* No cards found matching your search criteria\n")
	} else {
		for _, card := range cards {
			animatedIcon := ""
			if card.Animated {
				animatedIcon = "✨"
			}

			description.WriteString(fmt.Sprintf("* %s %s%s [%s]\n",
				utils.GetPromoRarityPlainText(card.ColID, card.Level),
				utils.FormatCardName(card.Name),
				animatedIcon,
				strings.Trim(utils.FormatCollectionName(card.ColID), "[]"),
			))
		}
	}

	description.WriteString("```")
	return description.String()
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

// sortCardsByRelevance sorts cards by relevance using unified search for improved accuracy
func sortCardsByRelevance(cards []*models.Card, searchTerm string) {
	// Skip sorting if no search term or no cards
	if searchTerm == "" || len(cards) == 0 {
		return
	}

	// Use UnifiedSearchService for improved search accuracy and speed
	cardOpsService := services.NewCardOperationsService(nil, nil) // repositories not needed for this operation
	unifiedSearchService := services.NewUnifiedSearchService(cardOpsService)

	filters := utils.SearchFilters{
		Name:     searchTerm,
		SortBy:   utils.SortByLevel,
		SortDesc: true,
	}

	// Use unified search for better relevance scoring
	sortedCards := unifiedSearchService.SearchCards(context.Background(), cards, searchTerm, filters)

	// Replace contents of original slice with sorted results (only up to original length)
	if len(sortedCards) > 0 {
		copyLen := len(sortedCards)
		if copyLen > len(cards) {
			copyLen = len(cards)
		}
		copy(cards, sortedCards[:copyLen])
	}
}
