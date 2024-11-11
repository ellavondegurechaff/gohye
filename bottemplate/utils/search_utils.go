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

// WeightedSearch performs a weighted search on cards based on search terms
func WeightedSearch(cards []*models.Card, searchTerm string, mode SearchMode) []*models.Card {
	if searchTerm == "" {
		return cards
	}

	searchTermLower := strings.ToLower(searchTerm)
	searchWords := strings.FieldsFunc(searchTermLower, func(r rune) bool {
		return r == ' ' || r == '_'
	})

	var results []SearchResult
	for _, card := range cards {
		weight := calculateSearchWeight(card, searchTermLower, searchWords, mode)
		if weight > 0 {
			results = append(results, SearchResult{Card: card, Weight: weight})
		}
	}

	sortSearchResults(results)

	if mode == SearchModeExact {
		var highConfidenceResults []SearchResult
		for _, result := range results {
			if result.Weight >= WeightNameMatch {
				highConfidenceResults = append(highConfidenceResults, result)
			}
		}
		results = highConfidenceResults
	}

	sortedCards := make([]*models.Card, len(results))
	for i, result := range results {
		sortedCards[i] = result.Card
	}

	return sortedCards
}

func calculateSearchWeight(card *models.Card, fullTerm string, terms []string, mode SearchMode) int {
	weight := 0

	nameLower := strings.ToLower(card.Name)
	nameNormalized := strings.ReplaceAll(nameLower, "_", " ")
	fullTermNormalized := strings.ReplaceAll(fullTerm, "_", " ")

	colIDLower := strings.ToLower(card.ColID)

	switch mode {
	case SearchModeExact:
		if nameNormalized == fullTermNormalized {
			weight += WeightExactMatch
		} else if strings.Contains(nameNormalized, fullTermNormalized) {
			weight += WeightNameMatch
		}

		if colIDLower == fullTermNormalized {
			weight += WeightCollectionMatch
		}

	case SearchModePartial:
		matchedTerms := 0
		for _, term := range terms {
			termNorm := strings.TrimSpace(term)
			if termNorm == "" {
				continue
			}

			if strings.Contains(nameNormalized, termNorm) {
				weight += WeightPartialMatch
				matchedTerms++
			}

			if strings.Contains(colIDLower, termNorm) {
				weight += WeightPartialMatch / 2
				matchedTerms++
			}

			if level, err := strconv.Atoi(termNorm); err == nil && level == card.Level {
				weight += WeightLevelMatch
				matchedTerms++
			}
		}

		if matchedTerms == len(terms) {
			weight += WeightExactMatch / 2
		}
	}

	if len(terms) > 0 && strings.HasPrefix(nameNormalized, terms[0]) {
		weight += WeightPrefixMatch
	}

	return weight
}

func sortSearchResults(results []SearchResult) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Weight != results[j].Weight {
			return results[i].Weight > results[j].Weight
		}
		// If weights are equal, sort alphabetically
		return strings.ToLower(results[i].Card.Name) < strings.ToLower(results[j].Card.Name)
	})
}
