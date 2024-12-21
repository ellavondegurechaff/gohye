package utils

import (
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
)

type SearchMode int

const (
	SearchModeExact   SearchMode = iota // For price-stats (exact matches)
	SearchModePartial                   // For searchcards (partial matches)
)

// SearchResult represents a weighted search result
type SearchResult struct {
	Card   *models.Card
	Weight int
}

// Add new weight constants
const (
	WeightExactMatch      = 1000
	WeightNameMatch       = 500
	WeightCollectionMatch = 200
	WeightLevelMatch      = 100
	WeightTypeMatch       = 50
	WeightPrefixMatch     = 25
	WeightPartialMatch    = 10
)

// SearchFilters represents all possible search criteria
type SearchFilters struct {
	Query       string
	Name        string
	Levels      []int
	Collections []string
	Animated    bool
	Favorites   bool
	SortBy      string
	SortDesc    bool
	PromoOnly   bool
	MultiOnly   bool
	UserID      string
	BoyGroups   bool
	GirlGroups  bool
}

// SortOptions defines available sorting methods
const (
	SortByLevel = "level"
	SortByName  = "name"
	SortByCol   = "collection"
	SortByDate  = "date"
)

// CollectionInfo stores collection metadata for efficient lookups
type CollectionInfo struct {
	IsPromo bool
}

var collectionCache sync.Map

// InitializeCollectionInfo caches collection information for efficient searching
func InitializeCollectionInfo(collections []*models.Collection) {
	for _, collection := range collections {
		collectionCache.Store(collection.ID, CollectionInfo{
			IsPromo: collection.Promo,
		})
	}
}

// getCollectionInfo retrieves cached collection information
func getCollectionInfo(colID string) (CollectionInfo, bool) {
	if info, ok := collectionCache.Load(colID); ok {
		return info.(CollectionInfo), true
	}
	return CollectionInfo{}, false
}

// ParseSearchQuery parses a user's search query into structured filters
func ParseSearchQuery(query string) SearchFilters {
	filters := SearchFilters{
		Query:       query,
		SortBy:      SortByLevel,
		SortDesc:    true,
		Levels:      make([]int, 0),
		Collections: make([]string, 0),
	}

	terms := strings.Fields(strings.ToLower(query))
	for i := 0; i < len(terms); i++ {
		term := terms[i]

		// Handle sort commands
		if strings.HasPrefix(term, "<") || strings.HasPrefix(term, ">") {
			sortType := strings.TrimPrefix(strings.TrimPrefix(term, "<"), ">")
			switch sortType {
			case "level", "star":
				filters.SortBy = SortByLevel
			case "name":
				filters.SortBy = SortByName
			case "col":
				filters.SortBy = SortByCol
			case "date":
				filters.SortBy = SortByDate
			}
			filters.SortDesc = strings.HasPrefix(term, ">")
			continue
		}

		// Handle special filters
		switch {
		case term == "-multi":
			filters.MultiOnly = true
			continue // Skip adding to collections
		case term == "-promo":
			filters.PromoOnly = true
		case term == "-gif":
			filters.Animated = false
		case term == "gif":
			filters.Animated = true
		case term == "-boygroups":
			filters.BoyGroups = true
			continue // Skip adding to collections
		case term == "-girlgroups":
			filters.GirlGroups = true
			continue // Skip adding to collections
		case strings.HasPrefix(term, "-"):
			// Check if it's a level filter
			levelStr := strings.TrimPrefix(term, "-")
			if level, err := strconv.Atoi(levelStr); err == nil && level >= 1 && level <= 5 {
				filters.Levels = append(filters.Levels, level)
				continue
			}
			// If not a level, treat as collection filter
			filters.Collections = append(filters.Collections, strings.TrimPrefix(term, "-"))
		case term[0] >= '1' && term[0] <= '5' && len(term) == 1:
			// Single digit level filter without dash
			level, _ := strconv.Atoi(term)
			filters.Levels = append(filters.Levels, level)
		case term == "fav":
			filters.Favorites = true
		case term == "-fav":
			filters.Favorites = false
		default:
			// Add to name search if not a special term
			if filters.Name == "" {
				filters.Name = term
			} else {
				filters.Name += " " + term
			}
		}
	}

	return filters
}

// WeightedSearch performs an enhanced search with better matching
func WeightedSearch(cards []*models.Card, filters SearchFilters) []*models.Card {
	if len(cards) == 0 {
		return nil
	}

	var results []SearchResult
	searchTerms := strings.Fields(strings.ToLower(filters.Name))

	for _, card := range cards {
		// Check tag filters first
		if filters.BoyGroups {
			hasTag := false
			for _, tag := range card.Tags {
				if tag == "boygroups" {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		if filters.GirlGroups {
			hasTag := false
			for _, tag := range card.Tags {
				if tag == "girlgroups" {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		// Check collection filters first
		if len(filters.Collections) > 0 {
			collectionMatch := false
			cardColID := strings.ToLower(card.ColID)
			for _, collection := range filters.Collections {
				if strings.Contains(cardColID, strings.ToLower(collection)) {
					collectionMatch = true
					break
				}
			}
			if !collectionMatch {
				continue
			}
		}

		// Check promo filter if enabled
		if filters.PromoOnly {
			// Get collection info from cache
			if colInfo, exists := getCollectionInfo(card.ColID); !exists || !colInfo.IsPromo {
				continue
			}
		}

		// Check level filter
		if len(filters.Levels) > 0 {
			levelMatch := false
			for _, level := range filters.Levels {
				if card.Level == level {
					levelMatch = true
					break
				}
			}
			if !levelMatch {
				continue
			}
		}

		weight := calculateEnhancedWeight(card, searchTerms)
		if len(searchTerms) == 0 {
			// If no search terms, include all cards that passed filters
			results = append(results, SearchResult{Card: card, Weight: WeightPartialMatch})
		} else if weight > 0 {
			if weight == WeightExactMatch {
				return []*models.Card{card}
			}
			results = append(results, SearchResult{Card: card, Weight: weight})
		}
	}

	// Sort results
	sortResults(results, filters.SortBy, filters.SortDesc)

	// Convert to card slice
	sortedCards := make([]*models.Card, len(results))
	for i, result := range results {
		sortedCards[i] = result.Card
	}

	return sortedCards
}

func calculateEnhancedWeight(card *models.Card, terms []string) int {
	if len(terms) == 0 {
		return WeightPartialMatch // Return all cards when no search terms
	}

	weight := 0
	cardName := strings.ToLower(card.Name)
	cardNameNorm := strings.NewReplacer("_", " ", "-", " ").Replace(cardName)

	searchQuery := strings.ToLower(strings.Join(terms, " "))

	if cardNameNorm == searchQuery {
		return WeightExactMatch
	}

	if strings.Contains(cardNameNorm, searchQuery) {
		weight += WeightNameMatch
	}

	matchedTerms := 0
	for _, term := range terms {
		if strings.Contains(cardNameNorm, term) {
			weight += WeightPartialMatch
			matchedTerms++
		}
	}

	if matchedTerms == len(terms) {
		weight += WeightNameMatch / 2
	}

	if strings.HasPrefix(cardNameNorm, terms[0]) {
		weight += WeightPrefixMatch
	}

	return weight
}

func sortResults(results []SearchResult, sortBy string, desc bool) {
	sort.Slice(results, func(i, j int) bool {
		var less bool

		// Primary sort by level (descending)
		if results[i].Card.Level != results[j].Card.Level {
			less = results[i].Card.Level < results[j].Card.Level
			return !less // Descending order for levels
		}

		// Secondary sort by name (ascending)
		less = strings.ToLower(results[i].Card.Name) < strings.ToLower(results[j].Card.Name)

		// If explicit sort criteria is provided, use that instead
		if sortBy != SortByLevel {
			switch sortBy {
			case SortByName:
				less = strings.ToLower(results[i].Card.Name) < strings.ToLower(results[j].Card.Name)
			case SortByCol:
				less = strings.ToLower(results[i].Card.ColID) < strings.ToLower(results[j].Card.ColID)
			default:
				less = results[i].Weight < results[j].Weight
			}

			if desc {
				return !less
			}
		}

		return less
	})
}

// WeightedSearchWithMulti performs a search that can filter for cards with multiple copies
func WeightedSearchWithMulti(cards []*models.Card, filters SearchFilters, userCards map[int64]*models.UserCard) []*models.Card {
	if len(cards) == 0 {
		return nil
	}

	// Use existing WeightedSearch first
	baseResults := WeightedSearch(cards, filters)

	// If multi filter is not enabled, return base results
	if !filters.MultiOnly {
		return baseResults
	}

	// Filter for cards with multiple copies
	var multiResults []*models.Card
	for _, card := range baseResults {
		if userCard, exists := userCards[card.ID]; exists && userCard.Amount > 1 {
			multiResults = append(multiResults, card)
		}
	}

	return multiResults
}
