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

// SearchService provides unified search functionality across commands
type SearchService struct {
	cardRepo     interfaces.CardRepositoryInterface
	userCardRepo interfaces.UserCardRepositoryInterface
}

// NewSearchService creates a new search service
func NewSearchService(cardRepo interfaces.CardRepositoryInterface, userCardRepo interfaces.UserCardRepositoryInterface) *SearchService {
	return &SearchService{
		cardRepo:     cardRepo,
		userCardRepo: userCardRepo,
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
	if filters.MultiOnly {
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