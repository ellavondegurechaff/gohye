package cards

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/disgoorg/bot-template/bottemplate/utils"
)

type Service interface {
	GetUserCards(userID string, filters utils.FilterInfo) ([]Card, int, error)
}

type service struct {
	repository Repository
}

func NewService(repository Repository) *service {
	return &service{
		repository: repository,
	}
}

// decople filters later
func (s *service) GetUserCards(userID string, filters utils.FilterInfo) ([]Card, int, error) {
	userCards, err := s.repository.GetAllByUserID(context.Background(), userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch cards")
	}

	if len(userCards) == 0 {
		return nil, 0, fmt.Errorf("no cards found")
	}

	cards := make([]Card, 0, len(userCards))
	for _, userCard := range userCards {
		card, err := s.repository.GetByID(context.Background(), userCard.CardID)
		if err != nil {
			continue
		}

		// Apply filters
		// Should do on query
		if filters.Name != "" && !strings.Contains(strings.ToLower(card.Name), strings.ToLower(filters.Name)) {
			continue
		}
		if filters.Level != 0 && card.Level != filters.Level {
			continue
		}
		if filters.Tags != "" && !contains(card.Tags, filters.Tags) {
			continue
		}
		if filters.Collection != "" && !strings.Contains(strings.ToLower(card.ColID), strings.ToLower(filters.Collection)) {
			continue
		}
		if filters.Animated && !card.Animated {
			continue
		}
		if filters.Favorites && !userCard.Favorite {
			continue
		}

		cards = append(cards, Card{
			ID:        card.ID,
			Name:      card.Name,
			Level:     card.Level,
			Animated:  card.Animated,
			Favorite:  userCard.Favorite,
			Amount:    userCard.Amount,
			ColID:     card.ColID,
			Tags:      card.Tags,
			CreatedAt: card.CreatedAt,
			UpdatedAt: card.UpdatedAt,
		})
	}

	if len(cards) == 0 {
		return nil, 0, fmt.Errorf("no cards match your criteria")
	}

	// Sort cards by level (descending) and then by name
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Level != cards[j].Level {
			return cards[i].Level > cards[j].Level // Descending order
		}
		return cards[i].Name < cards[j].Name // Alphabetical order for same level
	})

	pages := int(math.Ceil(float64(len(cards)) / float64(utils.CardsPerPage)))

	return cards, pages, nil

}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
