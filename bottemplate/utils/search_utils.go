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

// AmountFilter represents amount-based filtering criteria
type AmountFilter struct {
	Min   int64  // >amount
	Max   int64  // <amount
	Exact int64  // =amount
}

// SearchFilters represents all possible search criteria
type SearchFilters struct {
	// Existing fields (unchanged for backward compatibility)
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
	
	// Enhanced filtering inspired by legacy JS system
	AntiCollections []string     // !collection - collections to exclude
	AntiLevels      []int        // !level - levels to exclude
	Tags            []string     // #tag - tag filters
	AntiTags        []string     // !#tag - anti-tag filters
	AmountFilter    AmountFilter // >amount, <amount, =amount
	
	// Enhanced user-specific filters
	ExcludeFavorites bool  // !fav - exclude favorites
	LockedOnly       bool  // -locked - locked cards only
	ExcludeLocked    bool  // !locked - exclude locked cards
	SingleOnly       bool  // !multi - single cards only
	ExcludePromo     bool  // !promo - exclude promo collections
	ExcludeAnimated  bool  // !gif - exclude animated cards
	
	// Special filters
	LastCard bool // . - last card filter
	
	// Legacy-inspired advanced filters
	NewOnly      bool   // -new - cards obtained since last daily
	ExcludeNew   bool   // !new - exclude new cards
	RatedOnly    bool   // -rated - only rated cards
	ExcludeRated bool   // !rated - exclude rated cards
	WishOnly     bool   // -wish - only wishlist cards
	ExcludeWish  bool   // !wish - exclude wishlist cards
	Diff         int    // -diff, !diff - difference mode (0=none, 1=diff, 2=miss)
	
	// Query type flags (from legacy system)
	UserQuery bool // indicates this query requires user-specific data
	EvalQuery bool // indicates this query requires evaluation/rating data
	
	// Multi-level sorting support (legacy firstBy().thenBy() equivalent)
	SortChain []SortCriteria // Chain of sort criteria
	
	// Additional parameters for advanced queries
	TargetUserID string // for diff queries
	LastDaily    string // timestamp for "new" filter comparisons
}

// SortOptions defines available sorting methods
const (
	SortByLevel  = "level"
	SortByName   = "name"
	SortByCol    = "collection"
	SortByDate   = "date"
	SortByAmount = "amount"
	SortByRating = "rating"
	SortByExp    = "exp"
	SortByEval   = "eval"
)

// SortCriteria represents a single sorting criterion (for multi-level sorting)
type SortCriteria struct {
	Field string // field to sort by
	Desc  bool   // sort direction (true = descending)
}

// CollectionInfo stores collection metadata for efficient lookups
type CollectionInfo struct {
	IsPromo         bool
	IsExcluded      bool // Excludes specific collections like signed, liveauction, lottery
	IsForgeExcluded bool // Excludes collections from forge operations (fragments, album, etc.)
}

var (
	collectionCache sync.Map
	cacheMutex      sync.RWMutex
	
	// List of collection IDs that should be excluded from general card operations
	excludedCollections = []string{"signed", "liveauction", "lottery"}
	
	// List of collection IDs that should be excluded from forge operations
	// Based on legacy system: fragments, album, liveauction, jackpot, birthdays, limited
	forgeExcludedCollections = []string{"fragments", "album", "liveauction", "jackpot", "birthdays", "limited"}
)

// InitializeCollectionInfo caches collection information for efficient searching
func InitializeCollectionInfo(collections []*models.Collection) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	
	for _, collection := range collections {
		// Check if collection is in excluded list
		isExcluded := false
		for _, excludedID := range excludedCollections {
			if collection.ID == excludedID {
				isExcluded = true
				break
			}
		}
		
		// Check if collection is forge-excluded
		isForgeExcluded := false
		for _, forgeExcludedID := range forgeExcludedCollections {
			if collection.ID == forgeExcludedID {
				isForgeExcluded = true
				break
			}
		}
		
		collectionCache.Store(collection.ID, CollectionInfo{
			IsPromo:         collection.Promo,
			IsExcluded:      isExcluded,
			IsForgeExcluded: isForgeExcluded,
		})
	}
}

// RefreshCollectionCache updates the collection cache with new data
func RefreshCollectionCache(collections []*models.Collection) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	
	// Clear existing cache
	collectionCache.Range(func(key, value interface{}) bool {
		collectionCache.Delete(key)
		return true
	})
	
	// Reload with new data
	for _, collection := range collections {
		// Check if collection is in excluded list
		isExcluded := false
		for _, excludedID := range excludedCollections {
			if collection.ID == excludedID {
				isExcluded = true
				break
			}
		}
		
		// Check if collection is forge-excluded
		isForgeExcluded := false
		for _, forgeExcludedID := range forgeExcludedCollections {
			if collection.ID == forgeExcludedID {
				isForgeExcluded = true
				break
			}
		}
		
		collectionCache.Store(collection.ID, CollectionInfo{
			IsPromo:         collection.Promo,
			IsExcluded:      isExcluded,
			IsForgeExcluded: isForgeExcluded,
		})
	}
}

// GetCollectionCacheSize returns the number of cached collections
func GetCollectionCacheSize() int {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	
	count := 0
	collectionCache.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// GetCollectionInfo retrieves cached collection information
func GetCollectionInfo(colID string) (CollectionInfo, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	
	if info, ok := collectionCache.Load(colID); ok {
		return info.(CollectionInfo), true
	}
	return CollectionInfo{}, false
}

// IsCardForgeEligible checks if a card can be used in forge operations
// This centralizes all forge eligibility logic based on the legacy system
func IsCardForgeEligible(card *models.Card, userCard *models.UserCard) bool {
	// Level 5+ cards cannot be forged (legendary restriction)
	if card.Level >= 5 {
		return false
	}
	
	// Locked cards cannot be forged
	if userCard != nil && userCard.Locked {
		return false
	}
	
	// Cards with amount <= 0 cannot be forged
	if userCard != nil && userCard.Amount <= 0 {
		return false
	}
	
	// Check collection-based exclusions
	if colInfo, exists := GetCollectionInfo(card.ColID); exists {
		// Forge-excluded collections cannot be forged
		if colInfo.IsForgeExcluded {
			return false
		}
	}
	
	// Favorite cards with only 1 copy cannot be forged (last copy protection)
	if userCard != nil && userCard.Favorite && userCard.Amount == 1 {
		return false
	}
	
	return true
}

// IsCardLiquefyEligible checks if a card can be liquefied
// This centralizes all liquefy eligibility logic based on the legacy system
func IsCardLiquefyEligible(card *models.Card, userCard *models.UserCard) bool {
	// Level restriction: cards above level 3 cannot be liquefied
	if card.Level > 3 {
		return false
	}
	
	// Locked cards cannot be liquefied
	if userCard != nil && userCard.Locked {
		return false
	}
	
	// Cards with amount <= 0 cannot be liquefied
	if userCard != nil && userCard.Amount <= 0 {
		return false
	}
	
	// Check collection-based exclusions for liquefy
	if colInfo, exists := GetCollectionInfo(card.ColID); exists {
		// General excluded collections cannot be liquefied
		if colInfo.IsExcluded {
			return false
		}
	}
	
	// Legacy specific exclusions: lottery and jackpot collections
	if card.ColID == "lottery" || card.ColID == "jackpot" {
		return false
	}
	
	// Favorite cards with only 1 copy cannot be liquefied (last copy protection)
	if userCard != nil && userCard.Favorite && userCard.Amount == 1 {
		return false
	}
	
	return true
}

// ParseSearchQuery parses a user's search query into structured filters
func ParseSearchQuery(query string) SearchFilters {
	filters := SearchFilters{
		Query:           query,
		SortBy:          SortByLevel,
		SortDesc:        true,
		Levels:          make([]int, 0),
		Collections:     make([]string, 0),
		AntiCollections: make([]string, 0),
		AntiLevels:      make([]int, 0),
		Tags:            make([]string, 0),
		AntiTags:        make([]string, 0),
	}

	terms := strings.Fields(strings.ToLower(query))
	for i := 0; i < len(terms); i++ {
		term := terms[i]

		// Handle special single-character terms
		if term == "." {
			filters.LastCard = true
			continue
		}

		// Handle sort commands and comparison operators
		if strings.HasPrefix(term, "<") || strings.HasPrefix(term, ">") || strings.HasPrefix(term, "=") {
			if parseComparisonOperator(term, &filters) {
				continue
			}
		}
		
		// Handle escaped operators (legacy system compatibility)
		if strings.HasPrefix(term, "\\") && len(term) > 1 {
			if parseEscapedOperator(term, &filters) {
				continue
			}
		}

		// Handle tag filters (#tag, !#tag)
		if strings.HasPrefix(term, "#") {
			tag := strings.TrimPrefix(term, "#")
			filters.Tags = append(filters.Tags, tag)
			continue
		}
		if strings.HasPrefix(term, "!#") {
			antiTag := strings.TrimPrefix(term, "!#")
			filters.AntiTags = append(filters.AntiTags, antiTag)
			continue
		}

		// Handle negative/exclusion filters (!, -)
		if strings.HasPrefix(term, "!") || strings.HasPrefix(term, "-") {
			if parseNegativeFilter(term, &filters) {
				continue
			}
		}

		// Handle level filters (1-5) and negative level filters (-1, -2, etc.)
		if len(term) == 1 && term[0] >= '1' && term[0] <= '5' {
			level, _ := strconv.Atoi(term)
			filters.Levels = append(filters.Levels, level)
			continue
		}
		if len(term) == 2 && term[0] == '-' && term[1] >= '1' && term[1] <= '5' {
			level, _ := strconv.Atoi(term[1:])
			filters.AntiLevels = append(filters.AntiLevels, level)
			continue
		}

		// Handle level=X syntax
		if strings.Contains(term, "level=") {
			levelStr := strings.TrimPrefix(term, "level=")
			if level, err := strconv.Atoi(levelStr); err == nil && level >= 1 && level <= 5 {
				filters.Levels = append(filters.Levels, level)
				continue
			}
		}

		// Handle collection=exact_id syntax for exact collection matching
		if strings.Contains(term, "collection=") {
			collectionID := strings.TrimPrefix(term, "collection=")
			if collectionID != "" {
				filters.Collections = append(filters.Collections, "exact:"+collectionID)
				continue
			}
		}

		// Handle plain identifiers before defaulting to name search
		if parseIdentifier(term, &filters) {
			continue
		}

		// Default: add to name search
		if filters.Name == "" {
			filters.Name = term
		} else {
			filters.Name += " " + term
		}
	}

	return filters
}

// parseComparisonOperator handles <, >, = operators for sorting and amount filtering
func parseComparisonOperator(term string, filters *SearchFilters) bool {
	operator := term[0]
	substr := term[1:]

	// Handle sort operators (enhanced with legacy system operators)
	switch substr {
	case "level", "star":
		filters.SortBy = SortByLevel
		filters.SortDesc = operator == '>'
		return true
	case "name":
		filters.SortBy = SortByName
		filters.SortDesc = operator == '>'
		return true
	case "col", "collection":
		filters.SortBy = SortByCol
		filters.SortDesc = operator == '>'
		return true
	case "date":
		filters.SortBy = SortByDate
		filters.SortDesc = operator == '>'
		filters.UserQuery = true // date sorting requires user data
		return true
	case "amount":
		filters.SortBy = SortByAmount
		filters.SortDesc = operator == '>'
		filters.UserQuery = true // amount sorting requires user data
		return true
	case "rating":
		filters.SortBy = SortByRating
		filters.SortDesc = operator == '>'
		filters.UserQuery = true // rating sorting requires user data
		filters.EvalQuery = true // rating requires evaluation data
		return true
	case "levels":
		filters.SortBy = SortByExp
		filters.SortDesc = operator == '>'
		filters.UserQuery = true // exp sorting requires user data
		return true
	case "eval":
		filters.SortBy = SortByEval
		filters.SortDesc = operator == '>'
		filters.UserQuery = true // eval sorting requires user data
		filters.EvalQuery = true // evaluation requires special processing
		return true
	}

	// Handle amount filtering (>amount=2, <amount=5, =amount=1)
	if strings.HasPrefix(substr, "amount=") {
		amountStr := strings.TrimPrefix(substr, "amount=")
		if amount, err := strconv.ParseInt(amountStr, 10, 64); err == nil {
			filters.UserQuery = true // amount filtering requires user data
			switch operator {
			case '>':
				filters.AmountFilter.Min = amount + 1
			case '<':
				filters.AmountFilter.Max = amount - 1
			case '=':
				filters.AmountFilter.Exact = amount
			}
			return true
		}
	}
	
	// Handle level comparison (>3, <5, =4) - for direct amounts
	if level, err := strconv.Atoi(substr); err == nil && level >= 1 && level <= 5 {
		filters.UserQuery = true
		switch operator {
		case '>':
			filters.AmountFilter.Min = int64(level)
		case '<':
			filters.AmountFilter.Max = int64(level)
		case '=':
			filters.AmountFilter.Exact = int64(level)
		}
		return true
	}

	return false
}

// parseNegativeFilter handles !, - operators for exclusion filters
func parseNegativeFilter(term string, filters *SearchFilters) bool {
	isExclude := strings.HasPrefix(term, "!")
	substr := strings.TrimPrefix(strings.TrimPrefix(term, "!"), "-")

	switch substr {
	case "multi":
		if isExclude {
			filters.SingleOnly = true
		} else {
			filters.MultiOnly = true
		}
		return true
	case "promo":
		if isExclude {
			filters.ExcludePromo = true
		} else {
			filters.PromoOnly = true
		}
		return true
	case "gif", "animated":
		if isExclude {
			filters.ExcludeAnimated = true
		} else {
			filters.Animated = true
		}
		return true
	case "boygroups":
		filters.BoyGroups = true
		return true
	case "girlgroups":
		filters.GirlGroups = true
		return true
	case "fav", "favorite":
		if isExclude {
			filters.ExcludeFavorites = true
		} else {
			filters.Favorites = true
		}
		return true
	case "locked", "lock":
		if isExclude {
			filters.ExcludeLocked = true
		} else {
			filters.LockedOnly = true
		}
		filters.UserQuery = true // locked filtering requires user data
		return true
	case "new":
		if isExclude {
			filters.ExcludeNew = true
		} else {
			filters.NewOnly = true
		}
		filters.UserQuery = true // new card filtering requires user data
		return true
	case "rated":
		if isExclude {
			filters.ExcludeRated = true
		} else {
			filters.RatedOnly = true
		}
		filters.UserQuery = true // rating filtering requires user data
		filters.EvalQuery = true // rating requires evaluation data
		return true
	case "wish":
		if isExclude {
			filters.ExcludeWish = true
		} else {
			filters.WishOnly = true
		}
		filters.UserQuery = true // wishlist filtering requires user data
		return true
	case "diff":
		if isExclude {
			filters.Diff = 2 // miss mode
		} else {
			filters.Diff = 1 // diff mode
		}
		filters.UserQuery = true // diff mode requires user data
		return true
	case "miss":
		if isExclude {
			filters.Diff = 2 // !miss = show owned
		} else {
			filters.Diff = 1 // -miss = show missing
		}
		filters.UserQuery = true // miss mode requires user data
		return true
	default:
		// Check if it's a level filter (!3, -4)
		if level, err := strconv.Atoi(substr); err == nil && level >= 1 && level <= 5 {
			if isExclude {
				filters.AntiLevels = append(filters.AntiLevels, level)
			} else {
				filters.Levels = append(filters.Levels, level)
			}
			return true
		}

		// Treat as collection filter (!twice, -promo)
		if isExclude {
			filters.AntiCollections = append(filters.AntiCollections, substr)
		} else {
			filters.Collections = append(filters.Collections, substr)
		}
		return true
	}
}

// parseIdentifier handles plain identifier terms without prefixes
func parseIdentifier(term string, filters *SearchFilters) bool {
	switch term {
	case "multi":
		filters.MultiOnly = true
		return true
	case "promo":
		filters.PromoOnly = true
		return true
	case "gif", "animated":
		filters.Animated = true
		return true
	case "boygroups":
		filters.BoyGroups = true
		return true
	case "girlgroups":
		filters.GirlGroups = true
		return true
	case "fav", "favorite":
		filters.Favorites = true
		return true
	case "locked", "lock":
		filters.LockedOnly = true
		filters.UserQuery = true
		return true
	case "new":
		filters.NewOnly = true
		filters.UserQuery = true
		return true
	case "rated":
		filters.RatedOnly = true
		filters.UserQuery = true
		filters.EvalQuery = true
		return true
	case "wish":
		filters.WishOnly = true
		filters.UserQuery = true
		return true
	case "diff":
		filters.Diff = 1
		filters.UserQuery = true
		return true
	case "miss":
		filters.Diff = 1
		filters.UserQuery = true
		return true
	default:
		return false
	}
}

// MatchesCollection provides more precise collection matching to avoid false positives
// like "-exo" matching "exocbx" or "-ninemuses" matching "ninemusesa"
func MatchesCollection(cardColID, searchTerm string) bool {
	// Exact match
	if cardColID == searchTerm {
		return true
	}
	
	// Check if search term is a prefix (for shorter collections)
	if strings.HasPrefix(cardColID, searchTerm) {
		// Only match if the next character is not alphanumeric (to avoid "exo" matching "exocbx")
		if len(cardColID) > len(searchTerm) {
			nextChar := cardColID[len(searchTerm)]
			// Allow match if next character is not alphanumeric (like underscore, hyphen, etc.)
			return !((nextChar >= 'a' && nextChar <= 'z') || (nextChar >= '0' && nextChar <= '9'))
		}
		return true
	}
	
	// Check if search term is a suffix (for collections ending with the term)
	if strings.HasSuffix(cardColID, searchTerm) {
		// Only match if the previous character is not alphanumeric
		if len(cardColID) > len(searchTerm) {
			prevChar := cardColID[len(cardColID)-len(searchTerm)-1]
			return !((prevChar >= 'a' && prevChar <= 'z') || (prevChar >= '0' && prevChar <= '9'))
		}
		return true
	}
	
	// For longer search terms (4+ chars), allow broader matching
	if len(searchTerm) >= 4 {
		return strings.Contains(cardColID, searchTerm)
	}
	
	return false
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

		// Apply enhanced filtering logic
		if !applyCardFilters(card, filters) {
			continue
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

	// Use strings.Builder for efficient string concatenation
	var searchQueryBuilder strings.Builder
	for i, term := range terms {
		if i > 0 {
			searchQueryBuilder.WriteByte(' ')
		}
		searchQueryBuilder.WriteString(strings.ToLower(term))
	}
	searchQuery := searchQueryBuilder.String()

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

	// Give significant bonus for matching all terms to prioritize complete matches
	if matchedTerms == len(terms) {
		weight += WeightNameMatch * 2 // Much higher bonus for complete matches
	}

	// Only give prefix bonus if at least one term matches
	if matchedTerms > 0 && strings.HasPrefix(cardNameNorm, terms[0]) {
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

// applyCardFilters applies all filter criteria to a card
func applyCardFilters(card *models.Card, filters SearchFilters) bool {
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
			return false
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
			return false
		}
	}

	// Check specific tag filters (#tag)
	if len(filters.Tags) > 0 {
		for _, filterTag := range filters.Tags {
			tagFound := false
			for _, cardTag := range card.Tags {
				if strings.EqualFold(cardTag, filterTag) {
					tagFound = true
					break
				}
			}
			if !tagFound {
				return false
			}
		}
	}

	// Check anti-tag filters (!#tag)
	if len(filters.AntiTags) > 0 {
		for _, antiTag := range filters.AntiTags {
			for _, cardTag := range card.Tags {
				if strings.EqualFold(cardTag, antiTag) {
					return false // Exclude this card
				}
			}
		}
	}

	// Check collection filters
	if len(filters.Collections) > 0 {
		collectionMatch := false
		cardColID := strings.ToLower(card.ColID)
		for _, collection := range filters.Collections {
			// Handle exact collection matching with "exact:" prefix
			if strings.HasPrefix(collection, "exact:") {
				exactID := strings.TrimPrefix(collection, "exact:")
				if cardColID == strings.ToLower(exactID) {
					collectionMatch = true
					break
				}
			} else {
				// Improved partial matching to avoid false positives
				if MatchesCollection(cardColID, strings.ToLower(collection)) {
					collectionMatch = true
					break
				}
			}
		}
		if !collectionMatch {
			return false
		}
	}

	// Check anti-collection filters (!collection)
	if len(filters.AntiCollections) > 0 {
		cardColID := strings.ToLower(card.ColID)
		for _, antiCol := range filters.AntiCollections {
			if MatchesCollection(cardColID, strings.ToLower(antiCol)) {
				return false // Exclude this card
			}
		}
	}

	// Check level filters
	if len(filters.Levels) > 0 {
		levelMatch := false
		for _, level := range filters.Levels {
			if card.Level == level {
				levelMatch = true
				break
			}
		}
		if !levelMatch {
			return false
		}
	}

	// Check anti-level filters (!level)
	if len(filters.AntiLevels) > 0 {
		for _, antiLevel := range filters.AntiLevels {
			if card.Level == antiLevel {
				return false // Exclude this level
			}
		}
	}

	// Check animated filters
	if filters.Animated && !card.Animated {
		return false
	}
	if filters.ExcludeAnimated && card.Animated {
		return false
	}

	// Check promo filter logic
	colInfo, colExists := GetCollectionInfo(card.ColID)
	
	if filters.PromoOnly {
		if !colExists || !colInfo.IsPromo {
			return false
		}
	}
	
	if filters.ExcludePromo {
		if colExists && colInfo.IsPromo {
			return false
		}
	}

	// Exclude cards from restricted collections (unless specifically requested)
	if !filters.PromoOnly && !filters.ExcludePromo {
		if colExists && (colInfo.IsPromo || colInfo.IsExcluded) {
			return false
		}
	}

	return true
}

// applyUserCardFilters applies user-specific filter criteria
func applyUserCardFilters(userCard *models.UserCard, filters SearchFilters) bool {
	if userCard == nil {
		return false
	}

	// Check amount filters
	if filters.AmountFilter.Min > 0 && userCard.Amount < filters.AmountFilter.Min {
		return false
	}
	if filters.AmountFilter.Max > 0 && userCard.Amount > filters.AmountFilter.Max {
		return false
	}
	if filters.AmountFilter.Exact > 0 && userCard.Amount != filters.AmountFilter.Exact {
		return false
	}

	// Check multi/single filters
	if filters.MultiOnly && userCard.Amount <= 1 {
		return false
	}
	if filters.SingleOnly && userCard.Amount != 1 {
		return false
	}

	// Check favorite filters
	if filters.Favorites && !userCard.Favorite {
		return false
	}
	if filters.ExcludeFavorites && userCard.Favorite {
		return false
	}

	// Check locked filters
	if filters.LockedOnly && !userCard.Locked {
		return false
	}
	if filters.ExcludeLocked && userCard.Locked {
		return false
	}

	return true
}

// WeightedSearchWithMulti performs a search that can filter for cards with multiple copies and user-specific criteria
func WeightedSearchWithMulti(cards []*models.Card, filters SearchFilters, userCards map[int64]*models.UserCard) []*models.Card {
	if len(cards) == 0 {
		return nil
	}

	// Use existing WeightedSearch first for card-level filtering
	baseResults := WeightedSearch(cards, filters)

	// Apply user-specific filters
	var filteredResults []*models.Card
	for _, card := range baseResults {
		userCard, exists := userCards[card.ID]
		if !exists {
			// If user doesn't own the card, skip user-specific filters
			if !filters.MultiOnly && !filters.SingleOnly && 
			   !filters.Favorites && !filters.ExcludeFavorites &&
			   !filters.LockedOnly && !filters.ExcludeLocked &&
			   filters.AmountFilter.Min == 0 && filters.AmountFilter.Max == 0 && filters.AmountFilter.Exact == 0 {
				filteredResults = append(filteredResults, card)
			}
			continue
		}

		// Apply user-specific filters
		if applyUserCardFilters(userCard, filters) {
			filteredResults = append(filteredResults, card)
		}
	}

	return filteredResults
}

// parseEscapedOperator handles escaped operators from legacy system (e.g., \<3 for heart)
func parseEscapedOperator(term string, filters *SearchFilters) bool {
	// Remove the escape character
	escapedTerm := term[1:]
	
	// Handle escaped heart operator
	if strings.HasPrefix(escapedTerm, "<") && len(escapedTerm) > 1 {
		// This is for handling escaped hearts like \<3
		// In the legacy system, this was used to prevent heart parsing
		// For now, we'll treat it as a regular comparison operator
		return parseComparisonOperator(escapedTerm, filters)
	}
	
	return false
}

// IsQueryEmpty checks if a query has meaningful content (legacy system compatibility)
func (filters *SearchFilters) IsQueryEmpty(useTag bool) bool {
	return filters.UserID == "" && 
		   !filters.LastCard && 
		   len(filters.Levels) == 0 && 
		   len(filters.Collections) == 0 && 
		   len(filters.AntiCollections) == 0 && 
		   len(filters.AntiLevels) == 0 && 
		   filters.Name == "" && 
		   (!useTag || (len(filters.Tags) == 0 && len(filters.AntiTags) == 0))
}

// AddSortCriteria adds a sort criterion to the sort chain (legacy firstBy().thenBy() equivalent)
func (filters *SearchFilters) AddSortCriteria(field string, desc bool) {
	if filters.SortChain == nil {
		filters.SortChain = make([]SortCriteria, 0)
	}
	filters.SortChain = append(filters.SortChain, SortCriteria{
		Field: field,
		Desc:  desc,
	})
}

// GetDefaultSort returns the default sort for legacy compatibility
func GetDefaultSort() []SortCriteria {
	return []SortCriteria{
		{Field: SortByLevel, Desc: true},  // First by level (descending)
		{Field: SortByCol, Desc: false},  // Then by collection (ascending)
		{Field: SortByName, Desc: false}, // Then by name (ascending)
	}
}

// ApplyLegacyFiltering applies all the legacy-style filters to user cards
func ApplyLegacyUserCardFiltering(userCard *models.UserCard, card *models.Card, filters SearchFilters) bool {
	if userCard == nil || card == nil {
		return false
	}
	
	// Apply user-specific filters
	if !applyUserCardFilters(userCard, filters) {
		return false
	}
	
	// Apply new/rated/wish filters
	if filters.NewOnly {
		// This would need lastDaily comparison - placeholder for now
		// if userCard.Obtained <= lastDaily { return false }
	}
	
	if filters.ExcludeNew {
		// This would need lastDaily comparison - placeholder for now
		// if userCard.Obtained > lastDaily { return false }
	}
	
	if filters.RatedOnly && userCard.Rating == 0 {
		return false
	}
	
	if filters.ExcludeRated && userCard.Rating > 0 {
		return false
	}
	
	// Wishlist filtering would need wishlist data
	// if filters.WishOnly && !isInWishlist(card.ID) { return false }
	// if filters.ExcludeWish && isInWishlist(card.ID) { return false }
	
	return true
}

// SortBuilder implements the legacy firstBy().thenBy() pattern for multi-level sorting
type SortBuilder struct {
	criteria []SortCriteria
}

// NewSortBuilder creates a new sort builder
func NewSortBuilder() *SortBuilder {
	return &SortBuilder{
		criteria: make([]SortCriteria, 0),
	}
}

// FirstBy sets the primary sort criterion (equivalent to legacy firstBy())
func (sb *SortBuilder) FirstBy(field string, desc bool) *SortBuilder {
	sb.criteria = []SortCriteria{{Field: field, Desc: desc}}
	return sb
}

// ThenBy adds a secondary sort criterion (equivalent to legacy thenBy())
func (sb *SortBuilder) ThenBy(field string, desc bool) *SortBuilder {
	sb.criteria = append(sb.criteria, SortCriteria{Field: field, Desc: desc})
	return sb
}

// Build returns the final sort criteria chain
func (sb *SortBuilder) Build() []SortCriteria {
	if len(sb.criteria) == 0 {
		// Default legacy sort: level desc, collection asc, name asc
		return GetDefaultSort()
	}
	return sb.criteria
}

// SortUserCards sorts user cards using the multi-level criteria (legacy equivalent)
func (sb *SortBuilder) SortUserCards(userCards []*models.UserCard, cardMap map[int64]*models.Card) {
	if len(userCards) == 0 || len(sb.criteria) == 0 {
		return
	}

	sort.Slice(userCards, func(i, j int) bool {
		cardI, okI := cardMap[userCards[i].CardID]
		cardJ, okJ := cardMap[userCards[j].CardID]

		// Handle missing cards
		if !okI || !okJ {
			return okJ && !okI
		}

		// Apply each sort criterion in order
		for _, criterion := range sb.criteria {
			comparison := sb.compareUserCards(userCards[i], userCards[j], cardI, cardJ, criterion)
			if comparison != 0 {
				return comparison < 0
			}
		}
		return false
	})
}

// SortCards sorts regular cards using the multi-level criteria
func (sb *SortBuilder) SortCards(cards []*models.Card) {
	if len(cards) == 0 || len(sb.criteria) == 0 {
		return
	}

	sort.Slice(cards, func(i, j int) bool {
		// Apply each sort criterion in order
		for _, criterion := range sb.criteria {
			comparison := sb.compareCards(cards[i], cards[j], criterion)
			if comparison != 0 {
				return comparison < 0
			}
		}
		return false
	})
}

// compareUserCards compares two user cards based on a single criterion
func (sb *SortBuilder) compareUserCards(ucA, ucB *models.UserCard, cardA, cardB *models.Card, criterion SortCriteria) int {
	var result int

	switch criterion.Field {
	case SortByLevel:
		result = cardA.Level - cardB.Level
	case SortByName:
		result = strings.Compare(strings.ToLower(cardA.Name), strings.ToLower(cardB.Name))
	case SortByCol:
		result = strings.Compare(strings.ToLower(cardA.ColID), strings.ToLower(cardB.ColID))
	case SortByDate:
		if ucA.Obtained.Before(ucB.Obtained) {
			result = -1
		} else if ucA.Obtained.After(ucB.Obtained) {
			result = 1
		} else {
			result = 0
		}
	case SortByAmount:
		if ucA.Amount < ucB.Amount {
			result = -1
		} else if ucA.Amount > ucB.Amount {
			result = 1
		} else {
			result = 0
		}
	case SortByRating:
		if ucA.Rating < ucB.Rating {
			result = -1
		} else if ucA.Rating > ucB.Rating {
			result = 1
		} else {
			result = 0
		}
	case SortByExp:
		if ucA.Exp < ucB.Exp {
			result = -1
		} else if ucA.Exp > ucB.Exp {
			result = 1
		} else {
			result = 0
		}
	default:
		result = 0
	}

	// Reverse if descending order
	if criterion.Desc {
		result = -result
	}

	return result
}

// compareCards compares two cards based on a single criterion
func (sb *SortBuilder) compareCards(cardA, cardB *models.Card, criterion SortCriteria) int {
	var result int

	switch criterion.Field {
	case SortByLevel:
		result = cardA.Level - cardB.Level
	case SortByName:
		result = strings.Compare(strings.ToLower(cardA.Name), strings.ToLower(cardB.Name))
	case SortByCol:
		result = strings.Compare(strings.ToLower(cardA.ColID), strings.ToLower(cardB.ColID))
	default:
		result = 0
	}

	// Reverse if descending order
	if criterion.Desc {
		result = -result
	}

	return result
}

// Legacy-style sort builders for backward compatibility

// SortByLevelDesc creates a legacy-style level descending sort (equivalent to firstBy((a, b) => b.level - a.level))
func SortByLevelDesc() *SortBuilder {
	return NewSortBuilder().FirstBy(SortByLevel, true)
}

// SortByNameAsc creates a legacy-style name ascending sort
func SortByNameAsc() *SortBuilder {
	return NewSortBuilder().FirstBy(SortByName, false)
}

// DefaultLegacySort creates the default legacy sort chain: level desc -> collection asc -> name asc
func DefaultLegacySort() *SortBuilder {
	return NewSortBuilder().
		FirstBy(SortByLevel, true).  // Level descending (high to low)
		ThenBy(SortByCol, false).    // Collection ascending (A to Z)
		ThenBy(SortByName, false)    // Name ascending (A to Z)
}

// BuildSortFromFilters creates a sort builder from search filters (legacy compatibility)
func BuildSortFromFilters(filters SearchFilters) *SortBuilder {
	builder := NewSortBuilder()
	
	// If sort chain is specified, use it
	if len(filters.SortChain) > 0 {
		for _, criterion := range filters.SortChain {
			if len(builder.criteria) == 0 {
				builder.FirstBy(criterion.Field, criterion.Desc)
			} else {
				builder.ThenBy(criterion.Field, criterion.Desc)
			}
		}
		return builder
	}
	
	// Otherwise use SortBy and SortDesc
	if filters.SortBy != "" {
		builder.FirstBy(filters.SortBy, filters.SortDesc)
	} else {
		// Default legacy sort
		return DefaultLegacySort()
	}
	
	return builder
}
