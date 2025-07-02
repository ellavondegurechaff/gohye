package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/interfaces"
	"github.com/disgoorg/bot-template/bottemplate/utils"
)

// SearchService provides unified search functionality across commands
type SearchService struct {
	cardRepo       interfaces.CardRepositoryInterface
	userCardRepo   interfaces.UserCardRepositoryInterface
	userRepo       repositories.UserRepository
	wishlistRepo   repositories.WishlistRepository
}

// NewSearchService creates a new search service
func NewSearchService(cardRepo interfaces.CardRepositoryInterface, userCardRepo interfaces.UserCardRepositoryInterface, userRepo repositories.UserRepository, wishlistRepo repositories.WishlistRepository) *SearchService {
	return &SearchService{
		cardRepo:     cardRepo,
		userCardRepo: userCardRepo,
		userRepo:     userRepo,
		wishlistRepo: wishlistRepo,
	}
}

// UserCardSearchResult represents search results for user cards
type UserCardSearchResult struct {
	UserCards []*models.UserCard
	Cards     []*models.Card
	CardMap   map[int64]*models.UserCard
}

// SearchUserCards searches through a user's card collection
func (ss *SearchService) SearchUserCards(ctx context.Context, userID, query string) (*UserCardSearchResult, error) {
	userCards, err := ss.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(userCards) == 0 {
		return &UserCardSearchResult{
			UserCards: []*models.UserCard{},
			Cards:     []*models.Card{},
			CardMap:   make(map[int64]*models.UserCard),
		}, nil
	}

	// If no query, return sorted cards
	if strings.TrimSpace(query) == "" {
		ss.sortUserCardsByLevel(ctx, userCards)
		return &UserCardSearchResult{
			UserCards: userCards,
			Cards:     nil,
			CardMap:   nil,
		}, nil
	}

	// Parse search filters
	filters := utils.ParseSearchQuery(query)

	// Convert UserCards to Cards for searching
	var cards []*models.Card
	cardMap := make(map[int64]*models.UserCard)

	for _, userCard := range userCards {
		card, err := ss.cardRepo.GetByID(ctx, userCard.CardID)
		if err != nil {
			continue
		}
		cards = append(cards, card)
		cardMap[card.ID] = userCard
	}

	// Apply search filters
	var results []*models.Card
	// Use user-aware search if there are any user-specific filters
	if ss.hasUserSpecificFilters(filters) {
		results = utils.WeightedSearchWithMulti(cards, filters, cardMap)
	} else {
		results = utils.WeightedSearch(cards, filters)
	}

	// Convert back to UserCards, preserving search order
	var filteredUserCards []*models.UserCard
	for _, card := range results {
		if userCard, ok := cardMap[card.ID]; ok {
			filteredUserCards = append(filteredUserCards, userCard)
		}
	}

	return &UserCardSearchResult{
		UserCards: filteredUserCards,
		Cards:     cards,
		CardMap:   cardMap,
	}, nil
}

// SearchMissingCards finds cards that a user doesn't own
func (ss *SearchService) SearchMissingCards(ctx context.Context, userID, query string) ([]*models.Card, error) {
	// Get all cards
	allCards, err := ss.cardRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// Get user's cards
	userCards, err := ss.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Create map of owned cards
	ownedCards := make(map[int64]bool)
	for _, uc := range userCards {
		ownedCards[uc.CardID] = true
	}

	// Get missing cards
	var missingCards []*models.Card
	for _, card := range allCards {
		if !ownedCards[card.ID] {
			missingCards = append(missingCards, card)
		}
	}

	// Apply search filters if query exists
	if strings.TrimSpace(query) != "" {
		filters := utils.ParseSearchQuery(query)
		missingCards = utils.WeightedSearch(missingCards, filters)
	} else {
		// Default sorting by level and name when no query is provided
		ss.sortCardsByLevel(missingCards)
	}

	return missingCards, nil
}

// SearchCardsForDiff finds cards that user1 has but user2 doesn't
func (ss *SearchService) SearchCardsForDiff(ctx context.Context, user1ID, user2ID, query string) ([]*models.Card, []string, error) {
	// Get both users' cards
	user1Cards, err := ss.userCardRepo.GetAllByUserID(ctx, user1ID)
	if err != nil {
		return nil, nil, err
	}

	user2Cards, err := ss.userCardRepo.GetAllByUserID(ctx, user2ID)
	if err != nil {
		return nil, nil, err
	}

	// Create maps for easier lookup
	user1CardMap := make(map[int64]*models.UserCard)
	user2CardMap := make(map[int64]int64) // cardID -> amount

	for _, uc := range user1Cards {
		user1CardMap[uc.CardID] = uc
	}

	for _, uc := range user2Cards {
		user2CardMap[uc.CardID] = uc.Amount
	}

	// Find cards where user1 has more than user2
	var diffCards []*models.Card
	var percentages []string

	for cardID, user1Card := range user1CardMap {
		user2Amount := user2CardMap[cardID]
		if user1Card.Amount > user2Amount {
			card, err := ss.cardRepo.GetByID(ctx, cardID)
			if err != nil {
				continue
			}

			diffCards = append(diffCards, card)

			// Calculate percentage difference
			percentage := ""
			if user2Amount == 0 {
				percentage = "∞%"
			} else {
				diff := float64(user1Card.Amount-user2Amount) / float64(user2Amount) * 100
				percentage = strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.1f", diff), "0"), ".") + "%"
			}
			percentages = append(percentages, percentage)
		}
	}

	// Apply search filters if query exists
	if strings.TrimSpace(query) != "" {
		filters := utils.ParseSearchQuery(query)
		filteredCards := utils.WeightedSearch(diffCards, filters)
		
		// Filter percentages to match filtered cards
		var filteredPercentages []string
		cardToPercentage := make(map[int64]string)
		for i, card := range diffCards {
			if i < len(percentages) {
				cardToPercentage[card.ID] = percentages[i]
			}
		}
		
		for _, card := range filteredCards {
			if pct, exists := cardToPercentage[card.ID]; exists {
				filteredPercentages = append(filteredPercentages, pct)
			}
		}
		
		return filteredCards, filteredPercentages, nil
	}

	// Default sorting
	ss.sortCardsByLevel(diffCards)
	return diffCards, percentages, nil
}

// SearchWishlistCards searches through a user's wishlist
func (ss *SearchService) SearchWishlistCards(ctx context.Context, userID, query string) ([]*models.Card, error) {
	// This would require a WishlistRepository - placeholder implementation
	// wishlistCards, err := ss.wishlistRepo.GetByUserID(ctx, userID)
	// For now, return empty result
	return []*models.Card{}, nil
}

// sortUserCardsByLevel sorts user cards by level (descending) then name (ascending)
func (ss *SearchService) sortUserCardsByLevel(ctx context.Context, userCards []*models.UserCard) {
	sort.Slice(userCards, func(i, j int) bool {
		cardI, errI := ss.cardRepo.GetByID(ctx, userCards[i].CardID)
		cardJ, errJ := ss.cardRepo.GetByID(ctx, userCards[j].CardID)

		// Handle errors by putting cards with errors at the end
		if errI != nil || errJ != nil {
			return errJ != nil
		}

		// Primary sort by level (descending)
		if cardI.Level != cardJ.Level {
			return cardI.Level > cardJ.Level
		}

		// Secondary sort by name (ascending)
		return strings.ToLower(cardI.Name) < strings.ToLower(cardJ.Name)
	})
}

// sortCardsByLevel sorts cards by level (descending) then name (ascending)
func (ss *SearchService) sortCardsByLevel(cards []*models.Card) {
	sort.Slice(cards, func(i, j int) bool {
		// Primary sort by level (descending)
		if cards[i].Level != cards[j].Level {
			return cards[i].Level > cards[j].Level
		}

		// Secondary sort by name (ascending)
		return strings.ToLower(cards[i].Name) < strings.ToLower(cards[j].Name)
	})
}

// GetSearchSuggestions returns search suggestions based on partial input
func (ss *SearchService) GetSearchSuggestions(ctx context.Context, partial string) ([]string, error) {
	// Get all cards and extract unique names, collections, tags
	allCards, err := ss.cardRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	suggestions := make(map[string]bool)
	partial = strings.ToLower(partial)

	for _, card := range allCards {
		// Add card names
		if strings.Contains(strings.ToLower(card.Name), partial) {
			suggestions[card.Name] = true
		}

		// Add collection IDs
		if strings.Contains(strings.ToLower(card.ColID), partial) {
			suggestions[card.ColID] = true
		}

		// Add tags
		for _, tag := range card.Tags {
			if strings.Contains(strings.ToLower(tag), partial) {
				suggestions[tag] = true
			}
		}
	}

	// Convert to slice and sort
	result := make([]string, 0, len(suggestions))
	for suggestion := range suggestions {
		result = append(result, suggestion)
	}

	sort.Strings(result)
	return result, nil
}

// WithCardsCallback represents a callback function that processes user cards (legacy pattern)
type WithCardsCallback func(userCards []*models.UserCard, cards []*models.Card, filters utils.SearchFilters) error

// WithGlobalCardsCallback represents a callback function that processes all cards (legacy pattern)  
type WithGlobalCardsCallback func(cards []*models.Card, filters utils.SearchFilters) error

// WithMultiQueryCallback represents a callback function that processes multi-query results (legacy pattern)
type WithMultiQueryCallback func(cardBatches [][]*models.Card, filterBatches []utils.SearchFilters, originalFilters utils.SearchFilters) error

// WithCards implements the legacy withCards pattern - enriches command with user cards
func (ss *SearchService) WithCards(ctx context.Context, userID string, filters utils.SearchFilters, callback WithCardsCallback) error {
	if userID == "" {
		return fmt.Errorf("user ID is required for withCards")
	}

	// Get user's cards
	userCards, err := ss.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to fetch user cards: %w", err)
	}

	if len(userCards) == 0 {
		return fmt.Errorf("you don't have any cards. Get some using the claim command")
	}

	// Create a Set of owned card IDs for validation
	ownedCardIds := make(map[int64]bool)
	for _, uc := range userCards {
		ownedCardIds[uc.CardID] = true
	}

	// Map user cards to full card objects
	cardIDs := make([]int64, len(userCards))
	for i, uc := range userCards {
		cardIDs[i] = uc.CardID
	}

	cards, err := ss.cardRepo.GetByIDs(ctx, cardIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch card details: %w", err)
	}

	// Create card mapping for efficient lookups
	cardMap := make(map[int64]*models.Card)
	for _, card := range cards {
		cardMap[card.ID] = card
	}

	// Map user cards to enriched format (similar to legacy mapUserCards)
	var enrichedUserCards []*models.UserCard
	for _, userCard := range userCards {
		// Ensure we have valid card data and user owns this card
		if !ownedCardIds[userCard.CardID] {
			continue
		}

		// Find matching card definition
		card, exists := cardMap[userCard.CardID]
		if !exists {
			continue
		}

		// Double check ownership
		if card.ID != userCard.CardID {
			continue
		}

		enrichedUserCards = append(enrichedUserCards, userCard)
	}

	// Apply filters safely
	if len(filters.Tags) > 0 {
		filteredCards := ss.applyTagFilters(enrichedUserCards, cardMap, filters.Tags, false)
		enrichedUserCards = filteredCards
	}

	if len(filters.AntiTags) > 0 {
		filteredCards := ss.applyTagFilters(enrichedUserCards, cardMap, filters.AntiTags, true)
		enrichedUserCards = filteredCards
	}

	// Apply lastcard filter
	if filters.LastCard {
		enrichedUserCards = ss.applyLastCardFilter(ctx, enrichedUserCards, cardMap, userID)
	}

	// Apply wishlist filters
	if filters.WishOnly || filters.ExcludeWish {
		enrichedUserCards = ss.applyWishlistFilter(ctx, enrichedUserCards, userID, filters.WishOnly)
	}

	// Apply new card filters
	if filters.NewOnly || filters.ExcludeNew {
		enrichedUserCards = ss.applyNewCardFilter(ctx, enrichedUserCards, userID, filters.NewOnly)
	}

	if len(enrichedUserCards) == 0 {
		return fmt.Errorf("no cards found matching the query")
	}

	// Safe sort with ownership validation
	if len(enrichedUserCards) > 0 {
		ss.sortUserCardsByLevel(ctx, enrichedUserCards)
	}

	return callback(enrichedUserCards, cards, filters)
}

// WithGlobalCards implements the legacy withGlobalCards pattern - enriches command with all cards
func (ss *SearchService) WithGlobalCards(ctx context.Context, userID string, filters utils.SearchFilters, callback WithGlobalCardsCallback) error {
	var allCards []*models.Card
	var err error

	if filters.UserQuery {
		// If user query, get user cards and map them
		userCards, err := ss.userCardRepo.GetAllByUserID(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to fetch user cards: %w", err)
		}

		// Get card details for user cards
		cardIDs := make([]int64, len(userCards))
		for i, uc := range userCards {
			cardIDs[i] = uc.CardID
		}

		allCards, err = ss.cardRepo.GetByIDs(ctx, cardIDs)
		if err != nil {
			return fmt.Errorf("failed to fetch card details: %w", err)
		}
	} else {
		// Get all cards
		allCards, err = ss.cardRepo.GetAll(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch all cards: %w", err)
		}
	}

	// Apply card-level filters
	filteredCards := ss.applyCardFilters(allCards, filters)

	// Apply tag filters
	if len(filters.Tags) > 0 {
		cardMap := make(map[int64]*models.Card)
		for _, card := range filteredCards {
			cardMap[card.ID] = card
		}
		
		// For global cards, we need to convert to user cards format temporarily
		tempUserCards := make([]*models.UserCard, len(filteredCards))
		for i, card := range filteredCards {
			tempUserCards[i] = &models.UserCard{CardID: card.ID}
		}
		
		filteredUserCards := ss.applyTagFilters(tempUserCards, cardMap, filters.Tags, false)
		filteredCards = make([]*models.Card, len(filteredUserCards))
		for i, uc := range filteredUserCards {
			filteredCards[i] = cardMap[uc.CardID]
		}
	}

	if len(filters.AntiTags) > 0 {
		cardMap := make(map[int64]*models.Card)
		for _, card := range filteredCards {
			cardMap[card.ID] = card
		}
		
		tempUserCards := make([]*models.UserCard, len(filteredCards))
		for i, card := range filteredCards {
			tempUserCards[i] = &models.UserCard{CardID: card.ID}
		}
		
		filteredUserCards := ss.applyTagFilters(tempUserCards, cardMap, filters.AntiTags, true)
		filteredCards = make([]*models.Card, len(filteredUserCards))
		for i, uc := range filteredUserCards {
			filteredCards[i] = cardMap[uc.CardID]
		}
	}

	// Apply lastcard filter
	if filters.LastCard {
		filteredCards = ss.applyLastCardFilterGlobal(ctx, filteredCards, userID)
	}

	// Apply wishlist filters
	if filters.WishOnly || filters.ExcludeWish {
		filteredCards = ss.applyWishlistFilterGlobal(ctx, filteredCards, userID, filters.WishOnly)
	}

	// Apply new card filters
	if filters.NewOnly || filters.ExcludeNew {
		filteredCards = ss.applyNewCardFilterGlobal(ctx, filteredCards, userID, filters.NewOnly)
	}

	if len(filteredCards) == 0 {
		return fmt.Errorf("no cards found matching the query")
	}

	// Apply sorting
	ss.sortCardsByLevel(filteredCards)

	return callback(filteredCards, filters)
}

// WithMultiQuery implements the legacy withMultiQuery pattern - handles multiple queries
func (ss *SearchService) WithMultiQuery(ctx context.Context, userID string, queries []string, callback WithMultiQueryCallback) error {
	if len(queries) == 0 {
		return fmt.Errorf("no queries provided")
	}

	// Parse all queries
	parsedFilters := make([]utils.SearchFilters, len(queries))
	for i, query := range queries {
		parsedFilters[i] = utils.ParseSearchQuery(query)
	}

	// Get user cards
	userCards, err := ss.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to fetch user cards: %w", err)
	}

	if len(userCards) == 0 {
		return fmt.Errorf("you don't have any cards")
	}

	// Create user card mapping
	cardIDs := make([]int64, len(userCards))
	for i, uc := range userCards {
		cardIDs[i] = uc.CardID
	}

	cards, err := ss.cardRepo.GetByIDs(ctx, cardIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch card details: %w", err)
	}

	cardMap := make(map[int64]*models.Card)
	for _, card := range cards {
		cardMap[card.ID] = card
	}

	// Process each query
	cardBatches := make([][]*models.Card, len(queries))
	
	for i, filters := range parsedFilters {
		var batchCards []*models.Card
		
		if filters.LastCard {
			// Handle lastcard filter - get user's last card
			batchCards = ss.applyLastCardFilterGlobal(ctx, cards, userID)
		} else {
			// Apply filters to user cards
			var filteredUserCards []*models.UserCard
			for _, userCard := range userCards {
				if card, exists := cardMap[userCard.CardID]; exists {
					// Apply user-specific filters
					if ss.matchesUserCardFilters(userCard, card, filters) {
						filteredUserCards = append(filteredUserCards, userCard)
					}
				}
			}

			// Apply tag filters
			if len(filters.Tags) > 0 {
				filteredUserCards = ss.applyTagFilters(filteredUserCards, cardMap, filters.Tags, false)
			}

			if len(filters.AntiTags) > 0 {
				filteredUserCards = ss.applyTagFilters(filteredUserCards, cardMap, filters.AntiTags, true)
			}

			// Apply new card filters
			if filters.NewOnly || filters.ExcludeNew {
				filteredUserCards = ss.applyNewCardFilter(ctx, filteredUserCards, userID, filters.NewOnly)
			}

			// Convert back to cards
			for _, uc := range filteredUserCards {
				if card, exists := cardMap[uc.CardID]; exists {
					batchCards = append(batchCards, card)
				}
			}

			// Apply sorting
			ss.sortCardsByLevel(batchCards)
		}

		cardBatches[i] = batchCards

		// Check if any query returned no results
		if len(batchCards) == 0 {
			return fmt.Errorf("no cards found in request #%d", i+1)
		}
	}

	// Create a combined filters object for callback
	combinedFilters := utils.SearchFilters{
		Query:     strings.Join(queries, " "),
		UserQuery: true,
	}

	return callback(cardBatches, parsedFilters, combinedFilters)
}

// Helper methods for the legacy pattern implementations

// applyTagFilters filters user cards based on tags (similar to legacy fetchTaggedCards)
func (ss *SearchService) applyTagFilters(userCards []*models.UserCard, cardMap map[int64]*models.Card, tags []string, isAntiTag bool) []*models.UserCard {
	var filtered []*models.UserCard
	
	for _, userCard := range userCards {
		card, exists := cardMap[userCard.CardID]
		if !exists {
			continue
		}
		
		hasMatchingTag := false
		for _, filterTag := range tags {
			for _, cardTag := range card.Tags {
				if strings.EqualFold(cardTag, filterTag) {
					hasMatchingTag = true
					break
				}
			}
			if hasMatchingTag {
				break
			}
		}
		
		// For anti-tags, exclude cards that have matching tags
		// For regular tags, include cards that have matching tags
		if isAntiTag {
			if !hasMatchingTag {
				filtered = append(filtered, userCard)
			}
		} else {
			if hasMatchingTag {
				filtered = append(filtered, userCard)
			}
		}
	}
	
	return filtered
}

// applyCardFilters applies basic card-level filters (non-user-specific)
func (ss *SearchService) applyCardFilters(cards []*models.Card, filters utils.SearchFilters) []*models.Card {
	var filtered []*models.Card
	
	for _, card := range cards {
		// Apply level filters
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
		
		// Apply anti-level filters
		if len(filters.AntiLevels) > 0 {
			levelExcluded := false
			for _, antiLevel := range filters.AntiLevels {
				if card.Level == antiLevel {
					levelExcluded = true
					break
				}
			}
			if levelExcluded {
				continue
			}
		}
		
		// Apply collection filters
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
					// Use improved collection matching from utils
					if utils.MatchesCollection(cardColID, strings.ToLower(collection)) {
						collectionMatch = true
						break
					}
				}
			}
			if !collectionMatch {
				continue
			}
		}
		
		// Apply anti-collection filters
		if len(filters.AntiCollections) > 0 {
			collectionExcluded := false
			cardColID := strings.ToLower(card.ColID)
			for _, antiCol := range filters.AntiCollections {
				if utils.MatchesCollection(cardColID, strings.ToLower(antiCol)) {
					collectionExcluded = true
					break
				}
			}
			if collectionExcluded {
				continue
			}
		}
		
		// Apply animated filters
		if filters.Animated && !card.Animated {
			continue
		}
		if filters.ExcludeAnimated && card.Animated {
			continue
		}
		
		// Apply boy/girl group filters
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
		
		// Apply promo filters
		if colInfo, exists := utils.GetCollectionInfo(card.ColID); exists {
			if filters.PromoOnly && !colInfo.IsPromo {
				continue
			}
			if filters.ExcludePromo && colInfo.IsPromo {
				continue
			}
			// No exclusions - all cards are searchable
		}
		
		filtered = append(filtered, card)
	}
	
	return filtered
}

// matchesUserCardFilters checks if a user card matches user-specific filters
func (ss *SearchService) matchesUserCardFilters(userCard *models.UserCard, card *models.Card, filters utils.SearchFilters) bool {
	// Apply amount filters
	if filters.AmountFilter.Min > 0 && userCard.Amount < filters.AmountFilter.Min {
		return false
	}
	if filters.AmountFilter.Max > 0 && userCard.Amount > filters.AmountFilter.Max {
		return false
	}
	if filters.AmountFilter.Exact > 0 && userCard.Amount != filters.AmountFilter.Exact {
		return false
	}
	
	// Apply multi/single filters
	if filters.MultiOnly && userCard.Amount <= 1 {
		return false
	}
	if filters.SingleOnly && userCard.Amount != 1 {
		return false
	}
	
	// Apply favorite filters
	if filters.Favorites && !userCard.Favorite {
		return false
	}
	if filters.ExcludeFavorites && userCard.Favorite {
		return false
	}
	
	// Apply locked filters
	if filters.LockedOnly && !userCard.Locked {
		return false
	}
	if filters.ExcludeLocked && userCard.Locked {
		return false
	}
	
	// Apply rating filters
	if filters.RatedOnly && userCard.Rating == 0 {
		return false
	}
	if filters.ExcludeRated && userCard.Rating > 0 {
		return false
	}
	
	// Apply new card filters - placeholder, needs user context for lastDaily comparison
	// This is handled at the service layer where we have access to user data
	
	// Apply wishlist filters - now implemented using actual wishlist data
	if filters.WishOnly {
		// This card must be in the user's wishlist
		// We can't check this at the UserCard level since it requires a separate service call
		// This will be handled at the service layer level
	}
	
	if filters.ExcludeWish {
		// This card must NOT be in the user's wishlist
		// This will be handled at the service layer level
	}
	
	return true
}

// applyLastCardFilter filters user cards to only include the user's last queried card
func (ss *SearchService) applyLastCardFilter(ctx context.Context, userCards []*models.UserCard, cardMap map[int64]*models.Card, userID string) []*models.UserCard {
	// Get user to access their last card
	user, err := ss.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		// If we can't get the user, return empty result
		return []*models.UserCard{}
	}
	
	lastCardID := user.UserStats.LastCard
	if lastCardID == 0 {
		// No last card set, return empty result
		return []*models.UserCard{}
	}
	
	// Find the user card that matches the last card ID
	for _, userCard := range userCards {
		if userCard.CardID == lastCardID {
			return []*models.UserCard{userCard}
		}
	}
	
	// Last card not found in user's collection
	return []*models.UserCard{}
}

// applyLastCardFilterGlobal filters global cards to only include the user's last queried card
func (ss *SearchService) applyLastCardFilterGlobal(ctx context.Context, cards []*models.Card, userID string) []*models.Card {
	// Get user to access their last card
	user, err := ss.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		// If we can't get the user, return empty result
		return []*models.Card{}
	}
	
	lastCardID := user.UserStats.LastCard
	if lastCardID == 0 {
		// No last card set, return empty result
		return []*models.Card{}
	}
	
	// Find the card that matches the last card ID
	for _, card := range cards {
		if card.ID == lastCardID {
			return []*models.Card{card}
		}
	}
	
	// Last card not found in card collection
	return []*models.Card{}
}

// updateUserLastCard updates the user's last card in the database
func (ss *SearchService) updateUserLastCard(ctx context.Context, userID string, cardID int64) error {
	return ss.userRepo.UpdateLastCard(ctx, userID, cardID)
}

// applyWishlistFilter filters user cards based on wishlist status
func (ss *SearchService) applyWishlistFilter(ctx context.Context, userCards []*models.UserCard, userID string, wishOnly bool) []*models.UserCard {
	// Get user's wishlist
	wishlistItems, err := ss.wishlistRepo.GetByUserID(ctx, userID)
	if err != nil {
		// If we can't get the wishlist, return empty result for safety
		return []*models.UserCard{}
	}
	
	// Create a map of wishlist card IDs for O(1) lookup
	wishlistCardIDs := make(map[int64]bool)
	for _, item := range wishlistItems {
		wishlistCardIDs[item.CardID] = true
	}
	
	var filtered []*models.UserCard
	for _, userCard := range userCards {
		isInWishlist := wishlistCardIDs[userCard.CardID]
		
		if wishOnly {
			// Include only cards that are in the wishlist
			if isInWishlist {
				filtered = append(filtered, userCard)
			}
		} else {
			// Exclude cards that are in the wishlist
			if !isInWishlist {
				filtered = append(filtered, userCard)
			}
		}
	}
	
	return filtered
}

// applyWishlistFilterGlobal filters global cards based on wishlist status
func (ss *SearchService) applyWishlistFilterGlobal(ctx context.Context, cards []*models.Card, userID string, wishOnly bool) []*models.Card {
	// Get user's wishlist
	wishlistItems, err := ss.wishlistRepo.GetByUserID(ctx, userID)
	if err != nil {
		// If we can't get the wishlist, return empty result for safety
		return []*models.Card{}
	}
	
	// Create a map of wishlist card IDs for O(1) lookup
	wishlistCardIDs := make(map[int64]bool)
	for _, item := range wishlistItems {
		wishlistCardIDs[item.CardID] = true
	}
	
	var filtered []*models.Card
	for _, card := range cards {
		isInWishlist := wishlistCardIDs[card.ID]
		
		if wishOnly {
			// Include only cards that are in the wishlist
			if isInWishlist {
				filtered = append(filtered, card)
			}
		} else {
			// Exclude cards that are in the wishlist
			if !isInWishlist {
				filtered = append(filtered, card)
			}
		}
	}
	
	return filtered
}

// applyNewCardFilter filters user cards based on obtained date vs user's last daily
func (ss *SearchService) applyNewCardFilter(ctx context.Context, userCards []*models.UserCard, userID string, newOnly bool) []*models.UserCard {
	// Get user to access their last daily timestamp
	user, err := ss.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		// If we can't get the user, return empty result for safety
		return []*models.UserCard{}
	}
	
	lastDaily := user.LastDaily
	// If lastDaily is zero time, treat as very old date (before any card could be obtained)
	if lastDaily.IsZero() {
		if newOnly {
			// All cards are "new" if no daily has been claimed
			return userCards
		} else {
			// No cards are "old" if no daily has been claimed
			return []*models.UserCard{}
		}
	}
	
	var filtered []*models.UserCard
	for _, userCard := range userCards {
		isNewCard := userCard.Obtained.After(lastDaily)
		
		if newOnly {
			// Include only cards obtained after last daily
			if isNewCard {
				filtered = append(filtered, userCard)
			}
		} else {
			// Exclude cards obtained after last daily (include old cards)
			if !isNewCard {
				filtered = append(filtered, userCard)
			}
		}
	}
	
	return filtered
}

// applyNewCardFilterGlobal filters global cards based on user's last daily timestamp
func (ss *SearchService) applyNewCardFilterGlobal(ctx context.Context, cards []*models.Card, userID string, newOnly bool) []*models.Card {
	// Get user to access their last daily timestamp
	user, err := ss.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		// If we can't get the user, return empty result for safety
		return []*models.Card{}
	}
	
	// For global cards, we need to check if user owns them and when they were obtained
	userCards, err := ss.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		// If we can't get user cards, return empty result for safety
		return []*models.Card{}
	}
	
	// Create a map of card ID to obtained timestamp
	cardObtainedMap := make(map[int64]time.Time)
	for _, userCard := range userCards {
		cardObtainedMap[userCard.CardID] = userCard.Obtained
	}
	
	lastDaily := user.LastDaily
	// If lastDaily is zero time, treat as very old date
	if lastDaily.IsZero() {
		if newOnly {
			// Return cards that user owns (all are "new" if no daily claimed)
			var ownedCards []*models.Card
			for _, card := range cards {
				if _, owned := cardObtainedMap[card.ID]; owned {
					ownedCards = append(ownedCards, card)
				}
			}
			return ownedCards
		} else {
			// No cards are "old" if no daily has been claimed
			return []*models.Card{}
		}
	}
	
	var filtered []*models.Card
	for _, card := range cards {
		obtainedTime, owned := cardObtainedMap[card.ID]
		if !owned {
			// User doesn't own this card, skip it for new/old filtering
			continue
		}
		
		isNewCard := obtainedTime.After(lastDaily)
		
		if newOnly {
			// Include only cards obtained after last daily
			if isNewCard {
				filtered = append(filtered, card)
			}
		} else {
			// Exclude cards obtained after last daily (include old cards)
			if !isNewCard {
				filtered = append(filtered, card)
			}
		}
	}
	
	return filtered
}

// hasUserSpecificFilters checks if the search filters contain any user-specific criteria
func (ss *SearchService) hasUserSpecificFilters(filters utils.SearchFilters) bool {
	return filters.AmountFilter.Min > 0 || filters.AmountFilter.Max > 0 || filters.AmountFilter.Exact > 0 ||
		   filters.ExpFilter.Min > 0 || filters.ExpFilter.Max > 0 || filters.ExpFilter.Exact > 0 ||
		   filters.Favorites || filters.ExcludeFavorites ||
		   filters.LockedOnly || filters.ExcludeLocked ||
		   filters.MultiOnly || filters.SingleOnly ||
		   filters.NewOnly || filters.ExcludeNew ||
		   filters.RatedOnly || filters.ExcludeRated ||
		   filters.WishOnly || filters.ExcludeWish ||
		   filters.LastCard || filters.Diff > 0
}