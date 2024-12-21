package utils

import (
	"sort"
	"strconv"
	"strings"

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
	Query      string // Raw search query
	Name       string // Card name
	Level      int    // Card level (1-5)
	Collection string // Collection ID
	Animated   bool   // Animated cards only
	Favorites  bool   // Favorited cards only
	SortBy     string // Sort criteria
	SortDesc   bool   // Sort direction
}

// SortOptions defines available sorting methods
const (
	SortByLevel = "level"
	SortByName  = "name"
	SortByCol   = "collection"
	SortByDate  = "date"
)

// ParseSearchQuery parses a user's search query into structured filters
func ParseSearchQuery(query string) SearchFilters {
	filters := SearchFilters{
		Query:    query,
		SortBy:   SortByLevel, // Default to level sorting
		SortDesc: true,        // Default to descending order
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
		case term == "-gif":
			filters.Animated = false
		case term == "gif":
			filters.Animated = true
		case strings.HasPrefix(term, "-"):
			// Negative collection filter
			filters.Collection = strings.TrimPrefix(term, "-")
		case term[0] >= '1' && term[0] <= '5' && len(term) == 1:
			// Level filter
			filters.Level, _ = strconv.Atoi(term)
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

	// First pass: look for exact matches
	for _, card := range cards {
		weight := calculateEnhancedWeight(card, searchTerms)
		if weight == WeightExactMatch {
			// If we find an exact match, return only that card
			return []*models.Card{card}
		}
		if weight > 0 {
			results = append(results, SearchResult{Card: card, Weight: weight})
		}
	}

	// Only proceed with partial matches if no exact match was found
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
