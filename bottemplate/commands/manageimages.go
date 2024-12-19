package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/bot-template/internal/gateways/database/models"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"golang.org/x/sync/errgroup"
)

// Constants for performance tuning
const (
	MaxImageSize           = 800 * 1024 * 1024 // 800MB
	ChunkSize              = 32 * 1024 * 1024  // 32MB chunks
	InitialBufSize         = 4 * 1024 * 1024   // 4MB
	MaxConcurrentDownloads = 4
)

// Constants for level validation
var (
	minLength = 1
	maxLength = 100
	minLevel  = 1
	maxLevel  = 4
)

// Optimized ImageProcessor with connection pooling
type ImageProcessor struct {
	bufferPool        *sync.Pool
	client            *http.Client
	downloadSemaphore chan struct{} // Rate limiter for concurrent downloads
}

func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, InitialBufSize))
			},
		},
		client: &http.Client{
			Timeout: 10 * time.Minute,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  true,
				DisableKeepAlives:   false,
				MaxIdleConnsPerHost: 10,
				WriteBufferSize:     256 << 10, // 256KB
				ReadBufferSize:      256 << 10, // 256KB
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		},
		downloadSemaphore: make(chan struct{}, MaxConcurrentDownloads),
	}
}

// ManageImages command definition
var ManageImages = discord.SlashCommandCreate{
	Name:        "manage-images",
	Description: "🖼️ Manage card images",
	Options: []discord.ApplicationCommandOption{
		&discord.ApplicationCommandOptionSubCommand{
			Name:        "update",
			Description: "Update an existing card image",
			Options: []discord.ApplicationCommandOption{
				&discord.ApplicationCommandOptionInt{
					Name:        "card_id",
					Description: "Card ID",
					Required:    true,
					MinValue:    intPtr(minLevel),
				},
				&discord.ApplicationCommandOptionAttachment{
					Name:        "image",
					Description: "New image file",
					Required:    true,
				},
			},
		},
		&discord.ApplicationCommandOptionSubCommand{
			Name:        "verify",
			Description: "Verify image existence",
			Options: []discord.ApplicationCommandOption{
				&discord.ApplicationCommandOptionInt{
					Name:        "card_id",
					Description: "Card ID to verify",
					Required:    true,
					MinValue:    intPtr(minLevel),
				},
			},
		},
		&discord.ApplicationCommandOptionSubCommand{
			Name:        "delete",
			Description: "Delete a card and its images permanently",
			Options: []discord.ApplicationCommandOption{
				&discord.ApplicationCommandOptionInt{
					Name:        "card_id",
					Description: "Card ID to delete",
					Required:    true,
					MinValue:    intPtr(minLevel),
				},
				&discord.ApplicationCommandOptionBool{
					Name:        "confirm",
					Description: "Confirm deletion",
					Required:    true,
				},
			},
		},
	},
}

func ManageImagesHandler(b *bottemplate.Bot) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		start := time.Now()

		// Setup logger with request ID for tracing
		requestID := fmt.Sprintf("cmd-%s-%d", e.User().ID, time.Now().Unix())
		logger := slog.With(
			slog.String("request_id", requestID),
			slog.String("command", "manageimages"),
			slog.String("user_id", e.User().ID.String()),
		)

		logger.Info("Command started")

		// Increase timeout to 2 minutes
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Defer message with longer thinking time
		if err := e.DeferCreateMessage(true); err != nil {
			logger.Error("Failed to defer message", slog.String("error", err.Error()))
			return fmt.Errorf("failed to defer message: %w", err)
		}

		// Add timeout channel
		done := make(chan struct{})
		var result *services.ImageManagementResult
		var cmdErr error

		go func() {
			defer close(done)
			result, cmdErr = executeImageCommand(ctx, e, b, logger)
		}()

		// Wait for either completion or timeout
		select {
		case <-ctx.Done():
			logger.Error("Command timed out",
				slog.Duration("elapsed", time.Since(start)),
			)
			return createErrorEmbed(e, "Timeout", "The operation took too long to complete")
		case <-done:
			if cmdErr != nil {
				return createImageManagementResponse(e, result)
			}
			return createImageManagementResponse(e, result)
		}
	}
}

// Helper function for int64 min
func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// Helper function for int min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Optimized download function with parallel chunks
func (p *ImageProcessor) downloadLargeImage(ctx context.Context, url string, expectedSize int) ([]byte, error) {
	// Acquire semaphore
	select {
	case p.downloadSemaphore <- struct{}{}:
		defer func() { <-p.downloadSemaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Get content length first
	head, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(head)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	contentLength := resp.ContentLength
	if contentLength <= 0 {
		contentLength = int64(expectedSize)
	}

	// Pre-allocate the final buffer
	finalBuffer := make([]byte, contentLength)

	// Calculate chunk sizes for parallel downloads
	chunkSize := int64(ChunkSize)
	chunkCount := (contentLength + chunkSize - 1) / chunkSize
	if chunkCount > 8 {
		chunkCount = 8 // Max 8 parallel chunks
	}

	g, gctx := errgroup.WithContext(ctx)

	// Download chunks in parallel
	for i := int64(0); i < chunkCount; i++ {
		start := i * chunkSize
		end := min64(start+chunkSize, contentLength)

		// Create new variables for goroutine closure
		chunkStart, chunkEnd := start, end

		g.Go(func() error {
			return p.downloadChunk(gctx, url, finalBuffer, chunkStart, chunkEnd)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return finalBuffer, nil
}

// Helper function to download a single chunk
func (p *ImageProcessor) downloadChunk(ctx context.Context, url string, buffer []byte, start, end int64) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	_, err = io.ReadFull(resp.Body, buffer[start:end])
	return err
}

func executeImageCommand(ctx context.Context, e *handler.CommandEvent, b *bottemplate.Bot, logger *slog.Logger) (*services.ImageManagementResult, error) {
	start := time.Now()
	processor := NewImageProcessor()

	data := e.SlashCommandInteractionData()
	subCmd := *data.SubCommandName
	cardID := data.Int("card_id")

	// Fetch card details first
	card, err := b.CardRepository.GetByID(ctx, int64(cardID))
	if err != nil {
		return nil, fmt.Errorf("card not found: %w", err)
	}

	switch subCmd {
	case "update":
		attachment := data.Attachment("image")
		if attachment.Size > MaxImageSize {
			return &services.ImageManagementResult{
				Operation:    services.ImageOperation(subCmd),
				Success:      false,
				ErrorMessage: fmt.Sprintf("Image size exceeds %dMB limit", MaxImageSize/1024/1024),
			}, nil
		}

		imageData, err := processor.downloadLargeImage(ctx, attachment.URL, attachment.Size)
		if err != nil {
			return nil, fmt.Errorf("failed to download image: %w", err)
		}

		return b.SpacesService.ManageCardImage(ctx, services.ImageOperation(subCmd), int64(cardID), imageData, card)

	case "verify":
		return b.SpacesService.ManageCardImage(ctx, services.ImageOperation(subCmd), int64(cardID), nil, card)

	case "delete":
		confirm := data.Bool("confirm")
		if !confirm {
			return &services.ImageManagementResult{
				Operation:    services.DeleteOperation,
				Success:      false,
				ErrorMessage: "You must confirm the deletion by setting confirm to true",
			}, nil
		}

		// Use errgroup for concurrent operations
		g, gctx := errgroup.WithContext(ctx)

		var deletionReport *models.DeletionReport

		// Delete from database
		g.Go(func() error {
			var err error
			deletionReport, err = b.CardRepository.SafeDelete(gctx, int64(cardID))
			return err
		})

		// Delete from storage
		g.Go(func() error {
			paths := []string{
				fmt.Sprintf("%s/%s/%s/%d_%s.jpg",
					b.SpacesService.CardRoot,
					getGroupType(card.Tags),
					card.ColID,
					card.Level,
					card.Name,
				),
				fmt.Sprintf("%s/%s.jpg",
					card.ColID,
					card.Name,
				),
			}

			// Delete all paths concurrently
			for _, path := range paths {
				path := path // Create new variable for goroutine
				g.Go(func() error {
					return b.SpacesService.DeleteObject(gctx, path)
				})
			}
			return nil
		})

		if err := g.Wait(); err != nil {
			logger.Error("Deletion failed",
				slog.String("error", err.Error()),
				slog.Duration("elapsed", time.Since(start)),
			)
			return &services.ImageManagementResult{
				Operation:    services.DeleteOperation,
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to delete: %v", err),
			}, nil
		}

		return &services.ImageManagementResult{
			Operation:    services.DeleteOperation,
			Success:      true,
			CardName:     card.Name,
			CollectionID: card.ColID,
			Level:        card.Level,
			Stats: map[string]interface{}{
				"users_affected": deletionReport.UserCardsDeleted,
				"card_deleted":   deletionReport.CardDeleted,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown subcommand: %s", subCmd)
	}
}

// Helper function for exponential backoff retries
func retryWithBackoff(ctx context.Context, maxRetries int, operation func() (*services.ImageManagementResult, error)) (*services.ImageManagementResult, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		result, err := operation()
		if err == nil {
			return result, nil
		}

		lastErr = err
		if !isRetryableError(err) {
			break
		}

		// Exponential backoff
		if i < maxRetries-1 {
			time.Sleep(time.Duration(1<<uint(i)) * time.Second)
		}
	}
	return nil, lastErr
}

// Helper function to determine if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	retryableErrors := []string{
		"timeout",
		"connection reset",
		"connection refused",
		"no such host",
		"context canceled",
		"operation error S3",
		"network",
		"EOF",
	}

	for _, retryErr := range retryableErrors {
		if strings.Contains(errStr, retryErr) {
			return true
		}
	}

	return false
}

// Helper function to create error results
func createErrorResult(operation services.ImageOperation, card *models.Card, err error) *services.ImageManagementResult {
	result := &services.ImageManagementResult{
		Operation:    operation,
		Success:      false,
		ErrorMessage: err.Error(),
	}

	if card != nil {
		result.CardName = card.Name
		result.CollectionID = card.ColID
		result.Level = card.Level
	}

	return result
}

// Helper functions for image validation
func isValidImageFormat(filename string) bool {
	validExtensions := []string{".jpg", ".jpeg", ".png", ".gif"}
	for _, ext := range validExtensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}

func isValidImageContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Check magic numbers for different image formats
	if len(data) < 4 {
		return false
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return true
	}

	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return true
	}

	// GIF: 47 49 46 38
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return true
	}

	return false
}

func createImageManagementResponse(e *handler.CommandEvent, result *services.ImageManagementResult) error {
	if result == nil {
		return fmt.Errorf("nil result in createImageManagementResponse")
	}

	var description strings.Builder
	description.WriteString("```md\n")
	description.WriteString("# Operation Details\n")
	description.WriteString(fmt.Sprintf("* Operation: %s\n", result.Operation))

	if result.CardName != "" {
		description.WriteString(fmt.Sprintf("* Card Name: %s\n", result.CardName))
	}
	if result.CollectionID != "" {
		description.WriteString(fmt.Sprintf("* Collection: %s\n", result.CollectionID))
	}
	if result.Level > 0 {
		description.WriteString(fmt.Sprintf("* Level: %d\n", result.Level))
	}

	statusText := "❌ Failed"
	if result.Success {
		statusText = "✅ Success"
	}
	description.WriteString(fmt.Sprintf("* Status: %s\n", statusText))

	if result.ErrorMessage != "" {
		description.WriteString(fmt.Sprintf("* Error: %s\n", result.ErrorMessage))
	}
	description.WriteString("```")

	if result.Success && result.URL != "" {
		description.WriteString(fmt.Sprintf("\n**Image URL:**\n%s", result.URL))
	}

	color := utils.ErrorColor
	if result.Success {
		color = utils.SuccessColor
	}

	now := time.Now()
	embed := discord.Embed{
		Title:       fmt.Sprintf("🖼️ Image Management - %s", result.Operation),
		Description: description.String(),
		Color:       color,
		Timestamp:   &now,
		Footer: &discord.EmbedFooter{
			Text: "Image Management System",
		},
	}

	// Only add image preview if we have a valid URL
	if result.Success && result.URL != "" {
		embed.Image = &discord.EmbedResource{
			URL: result.URL,
		}
	}

	_, err := e.CreateFollowupMessage(discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
	return err
}

func ManageImagesAutocomplete(b *bottemplate.Bot) handler.AutocompleteHandler {
	return func(e *handler.AutocompleteEvent) error {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Panic in autocomplete handler",
					slog.Any("panic", r),
					slog.String("stack_trace", string(debug.Stack())),
				)
			}
		}()

		focused := e.Data.Focused()
		if focused.Name != "collection" {
			return nil
		}

		// Correctly extract the search term from focused.Value
		searchTerm := ""
		if focused.Value != nil {
			var s string
			if err := json.Unmarshal(focused.Value, &s); err != nil {
				slog.Error("Failed to unmarshal focused.Value",
					slog.String("error", err.Error()))
				return e.AutocompleteResult([]discord.AutocompleteChoice{})
			}
			searchTerm = strings.TrimSpace(s)
		}

		// Get collections with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		collections, err := b.CollectionRepository.SearchCollections(ctx, searchTerm)
		if err != nil {
			slog.Error("Failed to search collections",
				slog.String("error", err.Error()),
				slog.String("search_term", searchTerm))
			return e.AutocompleteResult([]discord.AutocompleteChoice{})
		}

		// Build choices (max 25 as per Discord's limit)
		choices := make([]discord.AutocompleteChoice, 0, min(len(collections)+1, 25))

		// Add existing collections
		for _, col := range collections {
			if col == nil {
				continue
			}

			// Display only the name
			displayName := col.Name

			choices = append(choices, discord.AutocompleteChoiceString{
				Name:  displayName,
				Value: col.ID, // Use col.ID as the value for internal processing
			})
		}

		// Only add "Create New" option if:
		// 1. We have a search term
		// 2. No exact matches exist
		// 3. We haven't hit the limit
		if searchTerm != "" && len(choices) < 25 {
			exactMatchExists := false
			searchTermLower := strings.ToLower(searchTerm)

			for _, col := range collections {
				if strings.ToLower(col.Name) == searchTermLower {
					exactMatchExists = true
					break
				}
			}

			if !exactMatchExists {
				choices = append(choices, discord.AutocompleteChoiceString{
					Name:  "➕ Create new collection: " + searchTerm,
					Value: "new:" + searchTerm,
				})
			}
		}

		return e.AutocompleteResult(choices)
	}
}
