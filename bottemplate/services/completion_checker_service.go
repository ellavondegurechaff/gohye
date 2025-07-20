package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/interfaces"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// CompletionCheckerService handles automatic detection of collection completions
type CompletionCheckerService struct {
	client            bot.Client
	collectionService *CollectionService
	userRepo          repositories.UserRepository
	cardRepo          interfaces.CardRepositoryInterface
	userCardRepo      interfaces.UserCardRepositoryInterface
	collectionRepo    repositories.CollectionRepository
}

// NewCompletionCheckerService creates a new completion checker service
func NewCompletionCheckerService(
	client bot.Client,
	collectionService *CollectionService,
	userRepo repositories.UserRepository,
	cardRepo interfaces.CardRepositoryInterface,
	userCardRepo interfaces.UserCardRepositoryInterface,
	collectionRepo repositories.CollectionRepository,
) *CompletionCheckerService {
	return &CompletionCheckerService{
		client:            client,
		collectionService: collectionService,
		userRepo:          userRepo,
		cardRepo:          cardRepo,
		userCardRepo:      userCardRepo,
		collectionRepo:    collectionRepo,
	}
}

// CheckCompletionForCards checks collection completion for cards that were just added/modified
// This is called asynchronously after card operations
func (s *CompletionCheckerService) CheckCompletionForCards(ctx context.Context, userID string, cardIDs []int64) {
	if len(cardIDs) == 0 {
		return
	}

	// Get unique collection IDs from the affected cards
	collectionIDs, err := s.getUniqueCollectionIDs(ctx, cardIDs)
	if err != nil {
		slog.Error("Failed to get collection IDs for completion check",
			slog.String("user_id", userID),
			slog.Any("card_ids", cardIDs),
			slog.String("error", err.Error()))
		return
	}

	// Check each collection for completion
	for _, collectionID := range collectionIDs {
		s.checkSingleCollectionCompletion(ctx, userID, collectionID)
	}
}

// checkSingleCollectionCompletion checks if a user has completed a specific collection
func (s *CompletionCheckerService) checkSingleCollectionCompletion(ctx context.Context, userID string, collectionID string) {
	// Get user to check if they already have this collection marked as completed
	user, err := s.userRepo.GetByDiscordID(ctx, userID)
	if err != nil {
		slog.Error("Failed to get user for completion check",
			slog.String("user_id", userID),
			slog.String("collection_id", collectionID),
			slog.String("error", err.Error()))
		return
	}

	// Check if user already has this collection marked as completed
	alreadyCompleted := s.isCollectionAlreadyCompleted(user, collectionID)

	// Calculate current progress
	progress, err := s.collectionService.CalculateProgress(ctx, userID, collectionID)
	if err != nil {
		slog.Error("Failed to calculate collection progress",
			slog.String("user_id", userID),
			slog.String("collection_id", collectionID),
			slog.String("error", err.Error()))
		return
	}

	// Handle completion state changes
	if !alreadyCompleted && progress.IsCompleted {
		// User just completed this collection
		s.handleNewCompletion(ctx, user, collectionID)
	} else if alreadyCompleted && !progress.IsCompleted {
		// User lost completion (card was removed/transferred)
		s.handleLostCompletion(ctx, user, collectionID)
	}
}

// isCollectionAlreadyCompleted checks if user already has this collection marked as completed
func (s *CompletionCheckerService) isCollectionAlreadyCompleted(user *models.User, collectionID string) bool {
	if user.CompletedCols == nil {
		return false
	}

	for _, completedCol := range user.CompletedCols {
		if completedCol.ID == collectionID {
			return true
		}
	}
	return false
}

// handleNewCompletion handles when a user completes a collection for the first time
func (s *CompletionCheckerService) handleNewCompletion(ctx context.Context, user *models.User, collectionID string) {
	// Add to completed collections
	if user.CompletedCols == nil {
		user.CompletedCols = make(models.FlexibleCompletedCols, 0)
	}
	user.CompletedCols = append(user.CompletedCols, models.CompletedColModel{
		ID: collectionID,
	})

	// Save user
	if err := s.userRepo.Update(ctx, user); err != nil {
		slog.Error("Failed to update user completed collections",
			slog.String("user_id", user.DiscordID),
			slog.String("collection_id", collectionID),
			slog.String("error", err.Error()))
		return
	}

	// Send notification
	s.sendCompletionNotification(ctx, user, collectionID, true)

	slog.Info("User completed collection",
		slog.String("user_id", user.DiscordID),
		slog.String("username", user.Username),
		slog.String("collection_id", collectionID))
}

// handleLostCompletion handles when a user loses completion of a collection
func (s *CompletionCheckerService) handleLostCompletion(ctx context.Context, user *models.User, collectionID string) {
	// Remove from completed collections
	if user.CompletedCols != nil {
		newCompletedCols := make(models.FlexibleCompletedCols, 0)
		for _, completedCol := range user.CompletedCols {
			if completedCol.ID != collectionID {
				newCompletedCols = append(newCompletedCols, completedCol)
			}
		}
		user.CompletedCols = newCompletedCols
	}

	// Save user
	if err := s.userRepo.Update(ctx, user); err != nil {
		slog.Error("Failed to update user completed collections after loss",
			slog.String("user_id", user.DiscordID),
			slog.String("collection_id", collectionID),
			slog.String("error", err.Error()))
		return
	}

	// Send notification
	s.sendCompletionNotification(ctx, user, collectionID, false)

	slog.Info("User lost collection completion",
		slog.String("user_id", user.DiscordID),
		slog.String("username", user.Username),
		slog.String("collection_id", collectionID))
}

// sendCompletionNotification sends a DM to the user about collection completion changes
func (s *CompletionCheckerService) sendCompletionNotification(ctx context.Context, user *models.User, collectionID string, isCompleted bool) {
	// Check if user has notifications enabled (following JavaScript pattern)
	// For now, we'll assume notifications are enabled. In the future, this could check user preferences

	// Get collection information
	collection, err := s.collectionRepo.GetByID(ctx, collectionID)
	if err != nil {
		slog.Error("Failed to get collection for notification",
			slog.String("collection_id", collectionID),
			slog.String("error", err.Error()))
		return
	}

	var message string
	var color int

	if isCompleted {
		message = fmt.Sprintf("🎉 **Collection Completed!**\n\nYou have just completed `%s`!\n\nYou can now decide if you want to reset this collection for a clout star and a legendary card if it contains one!\n\nOne copy of each card below 5 stars will be consumed if the collection has 200 or fewer cards. Otherwise 200 specified cards will be taken based on overall card composition.\n\nTo reset type: `/collection reset collection:%s`",
			collection.Name, collectionID)
		color = 0x00FF00 // Green
	} else {
		message = fmt.Sprintf("⚠️ **Collection Completion Lost**\n\nYou no longer have all the cards required for a full completion of `%s`. This collection has been removed from your completed list.",
			collection.Name)
		color = 0xFF9900 // Orange
	}

	// Create embed
	embed := discord.NewEmbedBuilder().
		SetDescription(message).
		SetColor(color).
		SetTimestamp(time.Now()).
		Build()

	// Send DM
	dmChannel, err := s.client.Rest().CreateDMChannel(snowflake.MustParse(user.DiscordID))
	if err != nil {
		slog.Error("Failed to create DM channel for completion notification",
			slog.String("user_id", user.DiscordID),
			slog.String("error", err.Error()))
		return
	}

	_, err = s.client.Rest().CreateMessage(dmChannel.ID(), discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
	if err != nil {
		slog.Debug("Failed to send completion notification DM (user may have DMs disabled)",
			slog.String("user_id", user.DiscordID),
			slog.String("error", err.Error()))
		return
	}

	slog.Info("Sent completion notification",
		slog.String("user_id", user.DiscordID),
		slog.String("collection_id", collectionID),
		slog.Bool("completed", isCompleted))
}

// getUniqueCollectionIDs gets unique collection IDs from a list of card IDs
func (s *CompletionCheckerService) getUniqueCollectionIDs(ctx context.Context, cardIDs []int64) ([]string, error) {
	if len(cardIDs) == 0 {
		return []string{}, nil
	}

	// Get cards to find their collection IDs
	cards, err := s.cardRepo.GetByIDs(ctx, cardIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get cards: %w", err)
	}

	// Extract unique collection IDs
	collectionIDSet := make(map[string]bool)
	for _, card := range cards {
		if card.ColID != "" {
			collectionIDSet[card.ColID] = true
		}
	}

	// Convert to slice
	collectionIDs := make([]string, 0, len(collectionIDSet))
	for collectionID := range collectionIDSet {
		collectionIDs = append(collectionIDs, collectionID)
	}

	return collectionIDs, nil
}
