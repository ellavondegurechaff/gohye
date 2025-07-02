package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/interfaces"
	"github.com/disgoorg/bot-template/bottemplate/utils"
)

// CardOperationsService provides common card operations functionality
type CardOperationsService struct {
	cardRepo     interfaces.CardRepositoryInterface
	userCardRepo interfaces.UserCardRepositoryInterface
}

// NewCardOperationsService creates a new card operations service
func NewCardOperationsService(cardRepo interfaces.CardRepositoryInterface, userCardRepo interfaces.UserCardRepositoryInterface) *CardOperationsService {
	return &CardOperationsService{
		cardRepo:     cardRepo,
		userCardRepo: userCardRepo,
	}
}

// GetUserCardsWithDetails fetches user cards with card details and applies filtering
func (s *CardOperationsService) GetUserCardsWithDetails(ctx context.Context, userID string, query string) ([]*models.UserCard, []*models.Card, error) {
	userCards, cards, _, err := s.GetUserCardsWithDetailsAndFilters(ctx, userID, query)
	return userCards, cards, err
}

// GetUserCardsWithDetailsAndFilters fetches user cards with card details, applies filtering, and returns the parsed filters
func (s *CardOperationsService) GetUserCardsWithDetailsAndFilters(ctx context.Context, userID string, query string) ([]*models.UserCard, []*models.Card, utils.SearchFilters, error) {
	return s.GetUserCardsWithDetailsAndFiltersWithUser(ctx, userID, query, nil)
}

// GetUserCardsWithDetailsAndFiltersWithUser fetches user cards with card details, applies filtering, and returns the parsed filters, with user data for advanced filtering
func (s *CardOperationsService) GetUserCardsWithDetailsAndFiltersWithUser(ctx context.Context, userID string, query string, user *models.User) ([]*models.UserCard, []*models.Card, utils.SearchFilters, error) {
	// Get user's cards
	userCards, err := s.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, nil, utils.SearchFilters{}, fmt.Errorf("failed to fetch user cards: %w", err)
	}

	if len(userCards) == 0 {
		return userCards, nil, utils.SearchFilters{}, nil
	}

	// Extract card IDs for bulk query
	cardIDs := make([]int64, len(userCards))
	cardMap := make(map[int64]*models.UserCard)
	
	for i, userCard := range userCards {
		cardIDs[i] = userCard.CardID
		cardMap[userCard.CardID] = userCard
	}

	// Get card details with bulk query
	cards, err := s.cardRepo.GetByIDs(ctx, cardIDs)
	if err != nil {
		return nil, nil, utils.SearchFilters{}, fmt.Errorf("failed to fetch card details: %w", err)
	}

	// Parse search filters
	filters := utils.SearchFilters{}
	var displayCards []*models.UserCard
	
	if len(query) > 0 {
		filters = utils.ParseSearchQuery(query)
		// Mark this as an inventory search to include album cards
		filters.IsInventorySearch = true
		
		// Apply favorites filtering FIRST (on UserCards before search)
		filteredUserCards := s.applyFavoritesFilter(userCards, filters)
		
		// Apply new card filtering if specified
		if (filters.NewOnly || filters.ExcludeNew) && user != nil {
			filteredUserCards = s.applyNewCardFilter(filteredUserCards, user, filters.NewOnly)
		}
		
		// Build card mappings for the filtered user cards
		filteredCardMap := make(map[int64]*models.UserCard)
		filteredCardIDs := make([]int64, len(filteredUserCards))
		for i, userCard := range filteredUserCards {
			filteredCardMap[userCard.CardID] = userCard
			filteredCardIDs[i] = userCard.CardID
		}
		
		// Get cards for the filtered user cards
		filteredCards, err := s.cardRepo.GetByIDs(ctx, filteredCardIDs)
		if err != nil {
			return nil, nil, utils.SearchFilters{}, fmt.Errorf("failed to fetch filtered card details: %w", err)
		}
		
		// Run search on the favorites-filtered cards using unified search
		var results []*models.Card
		if filters.MultiOnly {
			results = utils.WeightedSearchWithMulti(filteredCards, filters, filteredCardMap)
		} else {
			// Use SearchCardsInCollection which now uses UnifiedSearchService
			results = s.SearchCardsInCollection(ctx, filteredCards, filters)
		}

		// Map search results back to UserCards
		for _, card := range results {
			if userCard, ok := filteredCardMap[card.ID]; ok {
				displayCards = append(displayCards, userCard)
			}
		}
		
		// Apply user-specific sorting after mapping back to UserCards
		// This is needed for sorts like experience, amount, rating that require UserCard data
		if needsUserCardSorting(filters.SortBy) {
			s.sortUserCardsWithFilters(displayCards, cards, filters)
		}
	} else {
		// If no query, use all cards but sort them
		displayCards = userCards
		s.sortUserCards(displayCards, cards)
	}

	return displayCards, cards, filters, nil
}

// GetMissingCards returns cards the user doesn't own, with optional filtering
func (s *CardOperationsService) GetMissingCards(ctx context.Context, userID string, query string) ([]*models.Card, error) {
	// Get all cards from database
	allCards, err := s.cardRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cards: %w", err)
	}

	// Get user's cards
	userCards, err := s.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user cards: %w", err)
	}

	// Create a map of owned card IDs for O(1) lookup
	ownedCards := make(map[int64]bool)
	for _, uc := range userCards {
		ownedCards[uc.CardID] = true
	}

	// Filter out owned cards to get missing cards
	var missingCards []*models.Card
	for _, card := range allCards {
		if !ownedCards[card.ID] {
			missingCards = append(missingCards, card)
		}
	}

	// Apply search filter if provided
	if query != "" {
		filters := utils.ParseSearchQuery(query)
		missingCards = s.SearchCardsInCollection(ctx, missingCards, filters)
	} else {
		// Default sorting by level and name when no query is provided
		sort.Slice(missingCards, func(i, j int) bool {
			if missingCards[i].Level != missingCards[j].Level {
				return missingCards[i].Level > missingCards[j].Level
			}
			return strings.ToLower(missingCards[i].Name) < strings.ToLower(missingCards[j].Name)
		})
	}

	return missingCards, nil
}

// GetCardDifferences returns card differences between two users
func (s *CardOperationsService) GetCardDifferences(ctx context.Context, userID, targetUserID string, mode string) ([]*models.Card, error) {
	// Get cards for both users
	userCards, err := s.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user cards: %w", err)
	}

	targetCards, err := s.userCardRepo.GetAllByUserID(ctx, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch target user cards: %w", err)
	}

	// Get all cards for mapping
	allCards, err := s.cardRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cards: %w", err)
	}

	// Create card mapping
	cardMap := make(map[int64]*models.Card)
	for _, card := range allCards {
		cardMap[card.ID] = card
	}

	var diffCards []*models.Card

	if mode == "for" {
		// Cards user has that target doesn't
		targetOwned := make(map[int64]bool)
		for _, tc := range targetCards {
			targetOwned[tc.CardID] = true
		}

		for _, uc := range userCards {
			if !targetOwned[uc.CardID] {
				if card, exists := cardMap[uc.CardID]; exists {
					diffCards = append(diffCards, card)
				}
			}
		}
	} else if mode == "from" {
		// Cards target has that user doesn't
		userOwned := make(map[int64]bool)
		for _, uc := range userCards {
			userOwned[uc.CardID] = true
		}

		for _, tc := range targetCards {
			if !userOwned[tc.CardID] {
				if card, exists := cardMap[tc.CardID]; exists {
					diffCards = append(diffCards, card)
				}
			}
		}
	}

	return diffCards, nil
}

// SearchCardsInCollection searches within a specific collection of cards using unified search
func (s *CardOperationsService) SearchCardsInCollection(ctx context.Context, cards []*models.Card, filters utils.SearchFilters) []*models.Card {
	// Check if this is a filter-only operation (levels, collections, tags, etc. without name search)
	hasNameQuery := filters.Name != "" && filters.Name != filters.Query
	hasFilterQuery := len(filters.Levels) > 0 || len(filters.AntiLevels) > 0 || 
					  len(filters.Collections) > 0 || len(filters.AntiCollections) > 0 ||
					  len(filters.Tags) > 0 || len(filters.AntiTags) > 0 ||
					  filters.Animated || filters.ExcludeAnimated ||
					  filters.PromoOnly || filters.ExcludePromo
	
	// For filter-only operations or when name is just the parsed query, use WeightedSearch
	if !hasNameQuery || hasFilterQuery {
		return utils.WeightedSearch(cards, filters)
	}
	
	// Use UnifiedSearchService for text-based name searches
	unifiedSearchService := NewUnifiedSearchService(s)
	return unifiedSearchService.SearchCards(ctx, cards, filters.Name, filters)
}

// BuildCardMappings creates optimized lookup maps for card operations
func (s *CardOperationsService) BuildCardMappings(userCards []*models.UserCard, cards []*models.Card) (map[int64]*models.UserCard, map[int64]*models.Card) {
	userCardMap := make(map[int64]*models.UserCard)
	for _, userCard := range userCards {
		userCardMap[userCard.CardID] = userCard
	}

	cardMap := make(map[int64]*models.Card)
	for _, card := range cards {
		cardMap[card.ID] = card
	}

	return userCardMap, cardMap
}

// needsUserCardSorting checks if the sort criteria requires UserCard data
func needsUserCardSorting(sortBy string) bool {
	return sortBy == "exp" || sortBy == "amount" || sortBy == "rating" || sortBy == "date"
}

// sortUserCardsWithFilters sorts user cards based on the provided filters
func (s *CardOperationsService) sortUserCardsWithFilters(userCards []*models.UserCard, cards []*models.Card, filters utils.SearchFilters) {
	if len(userCards) == 0 {
		return
	}

	// Create a map for O(1) lookups
	cardMap := make(map[int64]*models.Card, len(cards))
	for _, card := range cards {
		cardMap[card.ID] = card
	}

	// Sort based on the filter criteria
	sort.Slice(userCards, func(i, j int) bool {
		cardI, okI := cardMap[userCards[i].CardID]
		cardJ, okJ := cardMap[userCards[j].CardID]

		// Handle missing cards by putting them at the end
		if !okI || !okJ {
			return okJ
		}

		switch filters.SortBy {
		case "exp":
			// Sort by experience (descending if SortDesc=true)
			if userCards[i].Exp != userCards[j].Exp {
				if filters.SortDesc {
					return userCards[i].Exp > userCards[j].Exp
				}
				return userCards[i].Exp < userCards[j].Exp
			}
		case "amount":
			// Sort by amount
			if userCards[i].Amount != userCards[j].Amount {
				if filters.SortDesc {
					return userCards[i].Amount > userCards[j].Amount
				}
				return userCards[i].Amount < userCards[j].Amount
			}
		case "rating":
			// Sort by rating
			if userCards[i].Rating != userCards[j].Rating {
				if filters.SortDesc {
					return userCards[i].Rating > userCards[j].Rating
				}
				return userCards[i].Rating < userCards[j].Rating
			}
		case "date":
			// Sort by obtained date
			if !userCards[i].Obtained.Equal(userCards[j].Obtained) {
				if filters.SortDesc {
					return userCards[i].Obtained.After(userCards[j].Obtained)
				}
				return userCards[i].Obtained.Before(userCards[j].Obtained)
			}
		}

		// Default tie-breaker: sort by card level (descending), then name (ascending)
		if cardI.Level != cardJ.Level {
			return cardI.Level > cardJ.Level
		}
		return strings.ToLower(cardI.Name) < strings.ToLower(cardJ.Name)
	})
}

// sortUserCards sorts user cards by level and name (helper method)
func (s *CardOperationsService) sortUserCards(userCards []*models.UserCard, cards []*models.Card) {
	if len(userCards) == 0 {
		return
	}

	// Create a map for O(1) lookups
	cardMap := make(map[int64]*models.Card, len(cards))
	for _, card := range cards {
		cardMap[card.ID] = card
	}

	// Sort using in-memory card data
	sort.Slice(userCards, func(i, j int) bool {
		cardI, okI := cardMap[userCards[i].CardID]
		cardJ, okJ := cardMap[userCards[j].CardID]

		// Handle missing cards by putting them at the end
		if !okI || !okJ {
			return okJ
		}

		// Primary sort by level (descending)
		if cardI.Level != cardJ.Level {
			return cardI.Level > cardJ.Level
		}

		// Secondary sort by name (ascending)
		return strings.ToLower(cardI.Name) < strings.ToLower(cardJ.Name)
	})
}

// applyFavoritesFilter filters UserCards based on favorites setting
func (s *CardOperationsService) applyFavoritesFilter(userCards []*models.UserCard, filters utils.SearchFilters) []*models.UserCard {
	// Check if favorites query was used and determine the type
	favQuery := ""
	for _, term := range strings.Fields(strings.ToLower(filters.Query)) {
		if term == "fav" || term == "!fav" {
			favQuery = term
			break
		}
	}
	
	// If no favorites query, return all cards
	if favQuery == "" {
		return userCards
	}
	
	var filteredCards []*models.UserCard
	for _, userCard := range userCards {
		switch favQuery {
		case "fav":
			// Show only favorited cards (Favorite = true)
			if userCard.Favorite {
				filteredCards = append(filteredCards, userCard)
			}
		case "!fav":
			// Show only non-favorited cards (Favorite = false)
			if !userCard.Favorite {
				filteredCards = append(filteredCards, userCard)
			}
		}
	}
	
	return filteredCards
}

// applyNewCardFilter filters UserCards based on when they were obtained vs last daily
func (s *CardOperationsService) applyNewCardFilter(userCards []*models.UserCard, user *models.User, newOnly bool) []*models.UserCard {
	if user == nil {
		return userCards
	}
	
	lastDaily := user.LastDaily
	
	// Handle zero time case - if user has never claimed daily
	if lastDaily.IsZero() {
		if newOnly {
			// All cards are "new" if no daily ever claimed
			return userCards
		} else {
			// No cards are "old" if no daily ever claimed
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
			// ExcludeNew: include only cards obtained before or on last daily
			if !isNewCard {
				filtered = append(filtered, userCard)
			}
		}
	}
	
	return filtered
}