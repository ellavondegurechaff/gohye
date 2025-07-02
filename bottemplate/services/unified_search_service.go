package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/sahilm/fuzzy"
)

// CardSearchItems implements fuzzy.Source for card searching
type CardSearchItems []CardSearchItem

// CardSearchItem represents a single searchable card
type CardSearchItem struct {
	Card *models.Card
	Name string
}

// String returns the searchable string for fuzzy matching
func (c CardSearchItem) String() string {
	return c.Name
}

// Len returns the length of the collection
func (items CardSearchItems) Len() int {
	return len(items)
}

// String returns the searchable string at index i
func (items CardSearchItems) String(i int) string {
	return items[i].Name
}

// UnifiedSearchService provides improved card search functionality
type UnifiedSearchService struct {
	cardOperationsService *CardOperationsService
}

// NewUnifiedSearchService creates a new unified search service
func NewUnifiedSearchService(cardOps *CardOperationsService) *UnifiedSearchService {
	return &UnifiedSearchService{
		cardOperationsService: cardOps,
	}
}

// SearchCards performs intelligent card search using fuzzy matching + filters
func (s *UnifiedSearchService) SearchCards(ctx context.Context, cards []*models.Card, query string, filters utils.SearchFilters) []*models.Card {
	log.Printf("[DEBUG] SearchCards: query='%s', total_cards=%d", query, len(cards))

	if len(cards) == 0 {
		log.Printf("[DEBUG] SearchCards: no cards provided")
		return nil
	}

	// If no query, use existing weighted search for filters
	if query == "" {
		log.Printf("[DEBUG] SearchCards: empty query, using WeightedSearch")
		return utils.WeightedSearch(cards, filters)
	}

	// Step 1: Apply collection/level/type filters first to reduce search space
	filteredCards := s.applyBasicFilters(cards, filters)
	log.Printf("[DEBUG] SearchCards: after basic filters: %d cards remain", len(filteredCards))

	// Special debug logging for halloween taeha search
	if strings.Contains(strings.ToLower(query), "halloween") && strings.Contains(strings.ToLower(query), "taeha") {
		log.Printf("[DEBUG] HALLOWEEN TAEHA SEARCH DEBUG:")
		log.Printf("[DEBUG] Original cards with 'halloween':")
		count := 0
		for i, card := range cards {
			if strings.Contains(strings.ToLower(card.Name), "halloween") {
				log.Printf("[DEBUG]   [%d] %s (ID=%d)", i, card.Name, card.ID)
				count++
				if count >= 10 { // Limit to first 10 for readability
					break
				}
			}
		}
		log.Printf("[DEBUG] Found %d total cards with 'halloween'", count)

		log.Printf("[DEBUG] Filtered cards with 'halloween':")
		count = 0
		for i, card := range filteredCards {
			if strings.Contains(strings.ToLower(card.Name), "halloween") {
				log.Printf("[DEBUG]   [%d] %s (ID=%d)", i, card.Name, card.ID)
				count++
			}
		}
		log.Printf("[DEBUG] Found %d filtered cards with 'halloween'", count)
	}

	if len(filteredCards) == 0 {
		log.Printf("[DEBUG] SearchCards: no cards after basic filtering")
		return nil
	}

	// Step 2: Create searchable items with normalized names
	searchItems := make(CardSearchItems, len(filteredCards))
	for i, card := range filteredCards {
		// Normalize card name for better matching
		normalizedName := s.normalizeCardName(card.Name)
		searchItems[i] = CardSearchItem{
			Card: card,
			Name: normalizedName,
		}
		// Log first few cards for debugging
		if i < 5 {
			log.Printf("[DEBUG] SearchCards: card[%d] original='%s', normalized='%s'", i, card.Name, normalizedName)
		}
	}

	// Step 3: Perform fuzzy search
	normalizedQuery := s.normalizeQuery(query)
	log.Printf("[DEBUG] SearchCards: normalized query='%s'", normalizedQuery)

	matches := fuzzy.FindFrom(normalizedQuery, searchItems)
	log.Printf("[DEBUG] SearchCards: fuzzy search found %d matches", len(matches))

	// Step 4: Extract cards from fuzzy results (already sorted by relevance)
	results := make([]*models.Card, len(matches))
	for i, match := range matches {
		results[i] = searchItems[match.Index].Card
		// Log first few matches for debugging
		if i < 3 {
			log.Printf("[DEBUG] SearchCards: match[%d] score=%d, card='%s'", i, match.Score, results[i].Name)
		}
	}

	// Step 5: Apply additional sorting if specified
	if filters.SortBy != "" && filters.SortBy != utils.SortByLevel {
		s.applySorting(results, filters)
	}

	log.Printf("[DEBUG] SearchCards: returning %d results", len(results))
	return results
}

// SearchSingleCard finds the best matching card for a query
func (s *UnifiedSearchService) SearchSingleCard(ctx context.Context, cards []*models.Card, query string) (*models.Card, error) {
	log.Printf("[DEBUG] SearchSingleCard: query='%s', total_cards=%d", query, len(cards))

	// Use default filters for single card search - ENABLE PROMO ACCESS
	filters := utils.SearchFilters{
		Query:     query,
		SortBy:    utils.SortByLevel,
		SortDesc:  true,
		PromoOnly: false, // Don't require promo only
	}

	results := s.SearchCards(ctx, cards, query, filters)
	log.Printf("[DEBUG] SearchSingleCard: found %d results", len(results))

	if len(results) == 0 {
		log.Printf("[DEBUG] SearchSingleCard: NO MATCHES for query='%s'", query)
		return nil, nil
	}

	log.Printf("[DEBUG] SearchSingleCard: best match='%s' (ID=%d)", results[0].Name, results[0].ID)
	return results[0], nil
}

// SearchSingleCardAdmin finds the best matching card for admin commands (bypasses all filters)
func (s *UnifiedSearchService) SearchSingleCardAdmin(ctx context.Context, cards []*models.Card, query string) (*models.Card, error) {
	log.Printf("[DEBUG] SearchSingleCardAdmin: query='%s', total_cards=%d (ADMIN MODE - no filters)", query, len(cards))

	if len(cards) == 0 {
		log.Printf("[DEBUG] SearchSingleCardAdmin: no cards provided")
		return nil, nil
	}

	if query == "" {
		log.Printf("[DEBUG] SearchSingleCardAdmin: empty query")
		return nil, nil
	}

	// Create searchable items directly without any filtering
	searchItems := make(CardSearchItems, len(cards))
	for i, card := range cards {
		normalizedName := s.normalizeCardName(card.Name)
		searchItems[i] = CardSearchItem{
			Card: card,
			Name: normalizedName,
		}
	}

	// Perform fuzzy search without filters
	normalizedQuery := s.normalizeQuery(query)
	log.Printf("[DEBUG] SearchSingleCardAdmin: normalized query='%s'", normalizedQuery)

	matches := fuzzy.FindFrom(normalizedQuery, searchItems)
	log.Printf("[DEBUG] SearchSingleCardAdmin: fuzzy search found %d matches", len(matches))

	if len(matches) == 0 {
		log.Printf("[DEBUG] SearchSingleCardAdmin: NO MATCHES for query='%s'", query)
		return nil, nil
	}

	bestMatch := searchItems[matches[0].Index].Card
	log.Printf("[DEBUG] SearchSingleCardAdmin: best match='%s' (ID=%d) score=%d", bestMatch.Name, bestMatch.ID, matches[0].Score)
	return bestMatch, nil
}

// BackwardCompatibleSearch maintains compatibility with existing WeightedSearch
func (s *UnifiedSearchService) BackwardCompatibleSearch(cards []*models.Card, filters utils.SearchFilters) []*models.Card {
	// For backward compatibility, use the query from filters.Name or filters.Query
	query := filters.Name
	if query == "" {
		query = filters.Query
	}

	// If no name/query, fall back to original WeightedSearch
	if query == "" {
		return utils.WeightedSearch(cards, filters)
	}

	// Use fuzzy search with filters
	return s.SearchCards(context.Background(), cards, query, filters)
}

// normalizeCardName converts card names to searchable format
func (s *UnifiedSearchService) normalizeCardName(cardName string) string {
	// Replace underscores and hyphens with spaces
	normalized := strings.ReplaceAll(cardName, "_", " ")
	normalized = strings.ReplaceAll(normalized, "-", " ")

	// Convert to lowercase for case-insensitive matching
	normalized = strings.ToLower(normalized)

	// Clean up multiple spaces
	normalized = strings.Join(strings.Fields(normalized), " ")

	return normalized
}

// normalizeQuery prepares the search query
func (s *UnifiedSearchService) normalizeQuery(query string) string {
	// Similar normalization as card names
	normalized := strings.ToLower(query)
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

// applyBasicFilters applies collection, level, and type filters before fuzzy search
func (s *UnifiedSearchService) applyBasicFilters(cards []*models.Card, filters utils.SearchFilters) []*models.Card {
	var filtered []*models.Card
	excludedCount := 0
	excludedReasons := make(map[string]int)

	for _, card := range cards {
		excluded := false
		excludeReason := ""

		// Apply collection filters
		if len(filters.Collections) > 0 {
			found := false
			cardColID := strings.ToLower(card.ColID)
			for _, collection := range filters.Collections {
				if strings.Contains(cardColID, strings.ToLower(collection)) {
					found = true
					break
				}
			}
			if !found {
				excluded = true
				excludeReason = "collection_filter"
			}
		}

		// Apply level filters
		if !excluded && len(filters.Levels) > 0 {
			found := false
			for _, level := range filters.Levels {
				if card.Level == level {
					found = true
					break
				}
			}
			if !found {
				excluded = true
				excludeReason = "level_filter"
			}
		}

		// Apply group type filters
		if !excluded && filters.BoyGroups {
			hasTag := false
			for _, tag := range card.Tags {
				if tag == "boygroups" {
					hasTag = true
					break
				}
			}
			if !hasTag {
				excluded = true
				excludeReason = "boygroups_filter"
			}
		}

		if !excluded && filters.GirlGroups {
			hasTag := false
			for _, tag := range card.Tags {
				if tag == "girlgroups" {
					hasTag = true
					break
				}
			}
			if !hasTag {
				excluded = true
				excludeReason = "girlgroups_filter"
			}
		}

		// Apply promo filter - THIS IS CRITICAL
		if !excluded {
			if filters.PromoOnly {
				if colInfo, exists := utils.GetCollectionInfo(card.ColID); !exists || !colInfo.IsPromo {
					excluded = true
					excludeReason = fmt.Sprintf("promo_only_filter(colInfo_exists=%v)", exists)
				}
			} else {
				// No exclusions - all cards are searchable
			}
		}

		if excluded {
			excludedCount++
			excludedReasons[excludeReason]++
			// Log excluded halloween cards specifically
			if strings.Contains(strings.ToLower(card.Name), "halloween") {
				log.Printf("[DEBUG] applyBasicFilters: EXCLUDED halloween card '%s' (ID=%d) reason=%s", card.Name, card.ID, excludeReason)
			}
		} else {
			filtered = append(filtered, card)
		}
	}

	log.Printf("[DEBUG] applyBasicFilters: excluded %d cards, kept %d cards", excludedCount, len(filtered))
	for reason, count := range excludedReasons {
		log.Printf("[DEBUG] applyBasicFilters: excluded %d cards due to %s", count, reason)
	}

	return filtered
}

// applySorting applies custom sorting to results
func (s *UnifiedSearchService) applySorting(cards []*models.Card, filters utils.SearchFilters) {
	// Implementation depends on sorting requirements
	// For now, keep fuzzy search order (already relevance-sorted)
}
