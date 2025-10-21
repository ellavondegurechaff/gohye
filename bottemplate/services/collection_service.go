package services

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/interfaces"
)

type CollectionService struct {
	collectionRepo repositories.CollectionRepository
	cardRepo       interfaces.CardRepositoryInterface
	userCardRepo   interfaces.UserCardRepositoryInterface
}

func NewCollectionService(
	collectionRepo repositories.CollectionRepository,
	cardRepo interfaces.CardRepositoryInterface,
	userCardRepo interfaces.UserCardRepositoryInterface,
) *CollectionService {
	return &CollectionService{
		collectionRepo: collectionRepo,
		cardRepo:       cardRepo,
		userCardRepo:   userCardRepo,
	}
}

func (s *CollectionService) IsFragmentCollection(ctx context.Context, collectionID string) (bool, error) {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return false, fmt.Errorf("failed to get collection: %w", err)
	}
	return collection.Fragments, nil
}

func (s *CollectionService) CalculateProgress(ctx context.Context, userID string, collectionID string) (*models.CollectionProgress, error) {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	// Use optimized method to get only cards for this collection
	colCards, err := s.cardRepo.GetByCollectionID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection cards: %w", err)
	}

	// Filter cards based on collection type (fragments vs regular)
	var filteredCards []*models.Card
	for _, card := range colCards {
		if collection.Fragments {
			if card.Level == 1 {
				filteredCards = append(filteredCards, card)
			}
		} else {
			if card.Level < 5 {
				filteredCards = append(filteredCards, card)
			}
		}
	}

	userCards, err := s.userCardRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user cards: %w", err)
	}

	userCardMap := make(map[int64]bool)
	for _, userCard := range userCards {
		if userCard.Amount > 0 {
			userCardMap[userCard.CardID] = true
		}
	}

	ownedCount := 0
	for _, card := range filteredCards {
		if userCardMap[card.ID] {
			ownedCount++
		}
	}

	totalCards := len(filteredCards)
	percentage := 0.0
	if totalCards > 0 {
		percentage = (float64(ownedCount) / float64(totalCards)) * 100
	}

	return &models.CollectionProgress{
		UserID:       userID,
		CollectionID: collectionID,
		TotalCards:   totalCards,
		OwnedCards:   ownedCount,
		Percentage:   percentage,
		IsCompleted:  percentage >= 100.0,
		IsFragment:   collection.Fragments,
		LastUpdated:  time.Now(),
	}, nil
}

func (s *CollectionService) CheckCompletion(ctx context.Context, userID string, collectionID string) (bool, error) {
	progress, err := s.CalculateProgress(ctx, userID, collectionID)
	if err != nil {
		return false, err
	}
	return progress.IsCompleted, nil
}

func (s *CollectionService) CalculateResetRequirements(ctx context.Context, userID string, collectionID string) (*models.ResetRequirements, error) {
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	allCards, err := s.cardRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all cards: %w", err)
	}

	var colCards []*models.Card
	for _, card := range allCards {
		if card.ColID == collectionID {
			if collection.Fragments {
				if card.Level == 1 {
					colCards = append(colCards, card)
				}
			} else {
				if card.Level < 5 {
					colCards = append(colCards, card)
				}
			}
		}
	}

	amount := len(colCards)
	fourStars := 0
	threeStars := 0
	twoStars := 0
	oneStars := 0

	for _, card := range colCards {
		switch card.Level {
		case 4:
			fourStars++
		case 3:
			threeStars++
		case 2:
			twoStars++
		case 1:
			oneStars++
		}
	}

	if amount < 200 {
		return &models.ResetRequirements{
			FourStars:  fourStars,
			ThreeStars: threeStars,
			TwoStars:   twoStars,
			OneStars:   oneStars,
			Total:      amount,
		}, nil
	}

	division := float64(amount) / 200.0
	return &models.ResetRequirements{
		FourStars:  int(math.Round(float64(fourStars) / division)),
		ThreeStars: int(math.Round(float64(threeStars) / division)),
		TwoStars:   int(math.Round(float64(twoStars) / division)),
		OneStars:   int(math.Round(float64(oneStars) / division)),
		Total:      200,
	}, nil
}

// CalculateProgressBatch calculates progress for multiple collections efficiently
func (s *CollectionService) CalculateProgressBatch(ctx context.Context, userID string, collections []*models.Collection) (map[string]*models.CollectionProgress, error) {
    // Load user cards once for all collections
    userCards, err := s.userCardRepo.GetAllByUserID(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get user cards: %w", err)
    }

    // Create user card lookup map
    userCardMap := make(map[int64]bool, len(userCards))
    for _, userCard := range userCards {
        if userCard.Amount > 0 {
            userCardMap[userCard.CardID] = true
        }
    }

    // Load all cards once and group by collection id to avoid per-collection queries
    allCards, err := s.cardRepo.GetAll(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get all cards: %w", err)
    }
    cardsByCollection := make(map[string][]*models.Card)
    for _, c := range allCards {
        cardsByCollection[c.ColID] = append(cardsByCollection[c.ColID], c)
    }

    // Compute progress for requested collections
    result := make(map[string]*models.CollectionProgress, len(collections))
    for _, collection := range collections {
        colCards := cardsByCollection[collection.ID]

        // Filter cards based on collection type
        ownedCount := 0
        totalCards := 0
        for _, card := range colCards {
            if collection.Fragments {
                if card.Level != 1 { continue }
            } else {
                if card.Level >= 5 { continue }
            }
            totalCards++
            if userCardMap[card.ID] { ownedCount++ }
        }

        percentage := 0.0
        if totalCards > 0 {
            percentage = (float64(ownedCount) / float64(totalCards)) * 100
        }

        result[collection.ID] = &models.CollectionProgress{
            UserID:       userID,
            CollectionID: collection.ID,
            TotalCards:   totalCards,
            OwnedCards:   ownedCount,
            Percentage:   percentage,
            IsCompleted:  percentage >= 100.0,
            IsFragment:   collection.Fragments,
            LastUpdated:  time.Now(),
        }
    }

    return result, nil
}

func (s *CollectionService) GetCollectionLeaderboard(ctx context.Context, collectionID string, limit int) ([]*models.CollectionProgressResult, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}
	if limit > 25 {
		limit = 25 // Maximum limit to prevent performance issues
	}

	return s.collectionRepo.GetCollectionProgress(ctx, collectionID, limit)
}

// GetRandomSampleCard returns a random card from the specified collection
// Filters cards to exclude legendary cards (level >= 5) following JavaScript reference behavior
func (s *CollectionService) GetRandomSampleCard(ctx context.Context, collectionID string) (*models.Card, error) {
	// Get collection to check if it's a fragment collection
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	// Get all cards for this collection
	colCards, err := s.cardRepo.GetByCollectionID(ctx, collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection cards: %w", err)
	}

	// Filter cards based on collection type and level restrictions
	// Following JavaScript reference: exclude legendary cards (level >= 5)
	var eligibleCards []*models.Card
	for _, card := range colCards {
		if collection.Fragments {
			// Fragment collections: only consider level 1 cards
			if card.Level == 1 {
				eligibleCards = append(eligibleCards, card)
			}
		} else {
			// Regular collections: consider all cards except legendary (level < 5)
			if card.Level < 5 {
				eligibleCards = append(eligibleCards, card)
			}
		}
	}

	// Check if we have any eligible cards
	if len(eligibleCards) == 0 {
		return nil, fmt.Errorf("no eligible cards found in collection")
	}

	// Select random card using existing codebase pattern
	selectedCard := eligibleCards[rand.Intn(len(eligibleCards))]

	return selectedCard, nil
}
