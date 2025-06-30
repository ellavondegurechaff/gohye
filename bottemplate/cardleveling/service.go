package cardleveling

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
)

type Service struct {
	config     *Config
	calculator *Calculator
	cardRepo   repositories.CardRepository
	cache      *sync.Map
}

func NewService(config *Config, cardRepo repositories.CardRepository) *Service {
	return &Service{
		config:     config,
		calculator: NewCalculator(config),
		cardRepo:   cardRepo,
		cache:      &sync.Map{},
	}
}

func (s *Service) GainExp(ctx context.Context, userCard *models.UserCard) (*LevelingResult, error) {
	// Verify card ownership
	if userCard == nil {
		return nil, errors.New("card not found")
	}

	// Verify the card belongs to the user
	ownedCard, err := s.cardRepo.GetUserCard(ctx, userCard.UserID, userCard.CardID)
	if err != nil {
		return nil, errors.New("you don't own this card")
	}
	if ownedCard.ID != userCard.ID {
		return nil, errors.New("invalid card ownership")
	}

	if userCard.Level >= 5 {
		return nil, errors.New("level 5 cards cannot gain experience")
	}

	// Get stats
	stats := s.getCardStats(userCard.UserID, userCard.CardID)
	if !s.canGainExp(stats) {
		return nil, errors.New("exp gain on cooldown")
	}

	// Calculate exp gain
	expConfig := s.calculator.CalculateExpGain(userCard.Level, stats)
	expGained := s.calculator.CalculateFinalExp(expConfig)

	// Update exp and check for level up
	newExp := userCard.Exp + expGained
	requiredExp := s.calculator.CalculateExpRequirement(userCard.Level)

	result := &LevelingResult{
		Success:     true,
		NewLevel:    userCard.Level,
		CurrentExp:  newExp,
		RequiredExp: requiredExp,
		ExpGained:   expGained,
	}

	// Check for level up
	if newExp >= requiredExp {
		result.NewLevel++
		userCard.Level = result.NewLevel // Update the actual card's level
		newExp = 0
		// Recalculate required exp for the new level
		result.RequiredExp = s.calculator.CalculateExpRequirement(result.NewLevel)
	}

	// Update database
	userCard.Exp = newExp
	if err := s.cardRepo.UpdateUserCard(ctx, userCard); err != nil {
		return nil, err
	}

	// Update stats
	s.updateStats(userCard.UserID, userCard.CardID, stats)

	return result, nil
}

func (s *Service) CombineCards(ctx context.Context, mainCard, fodderCard *models.UserCard) (*LevelingResult, error) {
	// Check if fodder card has exp
	if fodderCard.Exp <= 0 {
		return nil, errors.New("fodder card must have experience points to be used for combination")
	}

	// Transfer exp from fodder to main card
	newExp := mainCard.Exp + fodderCard.Exp
	requiredExp := s.calculator.CalculateExpRequirement(mainCard.Level)

	result := &LevelingResult{
		Success:     true,
		NewLevel:    mainCard.Level,
		CurrentExp:  newExp,
		RequiredExp: requiredExp,
		ExpGained:   fodderCard.Exp,
		Bonuses:     []string{fmt.Sprintf("📈 Gained %d EXP from card combination!", fodderCard.Exp)},
	}

	// Check for level up
	if newExp >= requiredExp && mainCard.Level < 5 {
		result.NewLevel++
		mainCard.Level = result.NewLevel // Update the actual card's level
		newExp = 0
		// Recalculate required exp for the new level
		result.RequiredExp = s.calculator.CalculateExpRequirement(result.NewLevel)
		result.Bonuses = append(result.Bonuses, "🎉 Level up! Ready to proceed to next level!")
	}

	// Update main card
	mainCard.Exp = newExp
	if err := s.cardRepo.UpdateUserCard(ctx, mainCard); err != nil {
		return nil, err
	}

	// Delete fodder card - fixed argument count
	if err := s.cardRepo.DeleteUserCard(ctx, fodderCard.ID); err != nil {
		return nil, err
	}

	return result, nil
}

// Helper methods for stats management
func (s *Service) getCardStats(userID string, cardID int64) *CardLevelingStats {
	key := fmt.Sprintf("%s:%d", userID, cardID)
	if stats, ok := s.cache.Load(key); ok {
		return stats.(*CardLevelingStats)
	}
	return &CardLevelingStats{}
}

func (s *Service) updateStats(userID string, cardID int64, stats *CardLevelingStats) {
	key := fmt.Sprintf("%s:%d", userID, cardID)
	stats.LastExpGain = time.Now()
	stats.DailyExpGains++
	stats.WeeklyExpGains++
	stats.TotalExpGains++
	s.cache.Store(key, stats)
}

func (s *Service) canGainExp(stats *CardLevelingStats) bool {
	if time.Since(stats.LastExpGain) < s.config.ExpGainCooldown {
		return false
	}
	if stats.DailyExpGains >= s.config.DailyExpGainLimit {
		return false
	}
	if stats.WeeklyExpGains >= s.config.WeeklyExpGainLimit {
		return false
	}
	return true
}
