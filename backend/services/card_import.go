package services

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/services"
	webmodels "github.com/disgoorg/bot-template/backend/models"
	"github.com/uptrace/bun"
)

// CardImportService provides card import operations with comprehensive validation
type CardImportService struct {
	repos         *webmodels.Repositories
	spacesService *services.SpacesService
	cardService   *CardManagementService
	db            *bun.DB
}

// NewCardImportService creates a new card import service
func NewCardImportService(repos *webmodels.Repositories, spacesService *services.SpacesService, cardService *CardManagementService, db *bun.DB) *CardImportService {
	return &CardImportService{
		repos:         repos,
		spacesService: spacesService,
		cardService:   cardService,
		db:            db,
	}
}

// ImportCards performs the complete card import pipeline
func (cis *CardImportService) ImportCards(ctx context.Context, req *webmodels.CardImportRequest) (*webmodels.CardImportResult, error) {
	startTime := time.Now()
	
	slog.Info("Starting card import operation",
		slog.String("collection_id", req.CollectionID),
		slog.String("group_type", req.GroupType),
		slog.Int("file_count", len(req.Files)),
		slog.Bool("validate_only", req.ValidateOnly))

	// Validate the request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid import request: %w", err)
	}

	result := &webmodels.CardImportResult{
		CollectionID:     req.CollectionID,
		ValidationErrors: make([]webmodels.ValidationError, 0),
		ProcessingErrors: make([]webmodels.ProcessingError, 0),
		FilesUploaded:    make([]string, 0),
		FilesSkipped:     make([]string, 0),
		ImportSummary:    &webmodels.ImportSummary{
			TotalFiles:    len(req.Files),
			LevelStats:    make(map[int]int),
			FileTypeStats: make(map[string]int),
			Duplicates:    make([]string, 0),
			LargeFiles:    make([]string, 0),
		},
	}

	// Phase 1: Validate all files
	slog.Info("Phase 1: Validating files")
	validationErrors := cis.ValidateFiles(req.Files)
	result.ValidationErrors = append(result.ValidationErrors, validationErrors...)
	result.ImportSummary.InvalidFiles = len(validationErrors)
	result.ImportSummary.ValidFiles = result.ImportSummary.TotalFiles - result.ImportSummary.InvalidFiles

	// Stop if validation only or critical errors found
	if req.ValidateOnly {
		result.Success = len(validationErrors) == 0
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	if result.HasCriticalErrors() {
		result.Success = false
		result.ErrorMessage = "Critical validation errors found. Import aborted."
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	// Phase 2: Check/Create Collection
	slog.Info("Phase 2: Checking collection")
	if err := cis.ensureCollection(ctx, req, result); err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Collection error: %v", err)
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result, err
	}

	// Phase 3: Process valid files in transaction
	slog.Info("Phase 3: Processing files")
	if err := cis.processFilesWithTransaction(ctx, req, result); err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Processing error: %v", err)
		result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
		return result, err
	}

	// Calculate final results
	result.ProcessingTimeMs = time.Since(startTime).Milliseconds()
	result.Success = !result.HasErrors()
	result.PartialSuccess = result.CardsCreated > 0 && result.HasErrors()

	slog.Info("Card import operation completed",
		slog.String("collection_id", req.CollectionID),
		slog.Int("cards_created", result.CardsCreated),
		slog.Int("cards_skipped", result.CardsSkipped),
		slog.Bool("success", result.Success),
		slog.Int64("duration_ms", result.ProcessingTimeMs))

	return result, nil
}

// ValidateFiles performs comprehensive file validation
func (cis *CardImportService) ValidateFiles(files []*webmodels.FileUpload) []webmodels.ValidationError {
	var errors []webmodels.ValidationError
	namesSeen := make(map[string]bool)
	
	for _, file := range files {
		// Basic file validation
		if err := file.Validate(); err != nil {
			errors = append(errors, webmodels.CreateValidationError(
				file.Name, "file_invalid", err.Error(), "critical"))
			continue
		}

		// Filename format validation
		parsed, err := cis.ParseFilename(file.Name)
		if err != nil {
			errors = append(errors, webmodels.CreateValidationError(
				file.Name, "filename_format", err.Error(), "critical"))
			continue
		}

		// Duplicate name check
		normalizedName := fmt.Sprintf("%d_%s", parsed.Level, parsed.Name)
		if namesSeen[normalizedName] {
			errors = append(errors, webmodels.CreateValidationError(
				file.Name, "duplicate_name", 
				fmt.Sprintf("Duplicate card name '%s' at level %d", parsed.Name, parsed.Level), 
				"high"))
		}
		namesSeen[normalizedName] = true

		// MIME type validation
		if err := cis.validateMimeType(file); err != nil {
			errors = append(errors, webmodels.CreateValidationError(
				file.Name, "mime_type", err.Error(), "medium"))
		}

		// File size warnings
		if file.Size > 5*1024*1024 { // 5MB
			errors = append(errors, webmodels.CreateValidationError(
				file.Name, "large_file", 
				fmt.Sprintf("File size %.2fMB is large", float64(file.Size)/(1024*1024)), 
				"low"))
		}
	}

	return errors
}

// ParseFilename parses and validates filename according to the required format
func (cis *CardImportService) ParseFilename(filename string) (*webmodels.ParsedFilename, error) {
	// Pattern: level_name(_additional_names).ext
	// Examples: 1_hello.jpg, 1_member1_member2_member3.jpg
	pattern := regexp.MustCompile(`^(\d+)_(.+)\.(jpg|jpeg|png|gif)$`)
	
	matches := pattern.FindStringSubmatch(strings.ToLower(filename))
	if len(matches) != 4 {
		return &webmodels.ParsedFilename{
			Original: filename,
			Valid:    false,
			ErrorMsg: "Filename must follow format: level_name.ext (e.g., 1_hello.jpg)",
		}, fmt.Errorf("invalid filename format")
	}

	level, err := strconv.Atoi(matches[1])
	if err != nil || level < 1 || level > 5 {
		return &webmodels.ParsedFilename{
			Original: filename,
			Valid:    false,
			ErrorMsg: "Level must be between 1 and 5",
		}, fmt.Errorf("invalid level: %s", matches[1])
	}

	name := strings.ReplaceAll(matches[2], "_", " ")
	extension := matches[3]
	isAnimated := extension == "gif"

	// Clean up name (remove extra spaces, validate characters)
	name = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(name, " "))
	if name == "" {
		return &webmodels.ParsedFilename{
			Original: filename,
			Valid:    false,
			ErrorMsg: "Card name cannot be empty",
		}, fmt.Errorf("empty card name")
	}

	normalized := fmt.Sprintf("%d_%s.%s", level, strings.ReplaceAll(name, " ", "_"), extension)

	return &webmodels.ParsedFilename{
		Level:      level,
		Name:       name,
		Extension:  extension,
		IsAnimated: isAnimated,
		Original:   filename,
		Normalized: normalized,
		Valid:      true,
	}, nil
}

// validateMimeType validates the MIME type of uploaded files
func (cis *CardImportService) validateMimeType(file *webmodels.FileUpload) error {
	// Check declared content type
	expectedTypes := []string{
		"image/jpeg", "image/jpg", "image/png", "image/gif",
	}
	
	validType := false
	for _, expected := range expectedTypes {
		if file.ContentType == expected {
			validType = true
			break
		}
	}
	
	if !validType {
		return fmt.Errorf("invalid content type %s, expected image type", file.ContentType)
	}

	// Check file extension matches content type
	ext := strings.ToLower(filepath.Ext(file.Name))
	expectedMime := mime.TypeByExtension(ext)
	if expectedMime != "" && expectedMime != file.ContentType {
		// Allow some common mismatches
		if !(ext == ".jpg" && file.ContentType == "image/jpeg") {
			return fmt.Errorf("content type %s doesn't match file extension %s", file.ContentType, ext)
		}
	}

	return nil
}

// ensureCollection checks if collection exists or creates it if needed
func (cis *CardImportService) ensureCollection(ctx context.Context, req *webmodels.CardImportRequest, result *webmodels.CardImportResult) error {
	// Check if collection exists
	collection, err := cis.repos.Collection.GetByID(ctx, req.CollectionID)
	if err != nil {
		if req.CreateCollection {
			// Create new collection
			newCollection := &models.Collection{
				ID:         req.CollectionID,
				Name:       req.DisplayName,
				Origin:     "",
				Aliases:    []string{req.CollectionID},
				Promo:      req.IsPromo,
				Compressed: true,
				Fragments:  false,
				Tags:       []string{req.GroupType},
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			if err := cis.repos.Collection.Create(ctx, newCollection); err != nil {
				return fmt.Errorf("failed to create collection: %w", err)
			}

			result.CollectionCreated = true
			slog.Info("Created new collection",
				slog.String("collection_id", req.CollectionID),
				slog.String("name", req.DisplayName))
		} else {
			return fmt.Errorf("collection '%s' not found", req.CollectionID)
		}
	} else {
		// Validate existing collection matches request
		if req.IsPromo != collection.Promo {
			return fmt.Errorf("collection promo status mismatch: request=%t, existing=%t", req.IsPromo, collection.Promo)
		}
	}

	return nil
}

// processFilesWithTransaction processes files within a database transaction
func (cis *CardImportService) processFilesWithTransaction(ctx context.Context, req *webmodels.CardImportRequest, result *webmodels.CardImportResult) error {
	tx, err := cis.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	var uploadedFiles []string
	var cardsToCreate []*models.Card

	// Process each valid file
	for _, file := range req.Files {
		// Skip files with validation errors
		hasError := false
		for _, valError := range result.ValidationErrors {
			if valError.FileName == file.Name && valError.GetSeverityLevel() >= 3 { // high or critical
				hasError = true
				break
			}
		}
		if hasError {
			result.FilesSkipped = append(result.FilesSkipped, file.Name)
			continue
		}

		parsed, err := cis.ParseFilename(file.Name)
		if err != nil {
			result.ProcessingErrors = append(result.ProcessingErrors,
				webmodels.CreateProcessingError(file.Name, "parsing", "parse_error", err.Error(), false))
			continue
		}

		// Check for existing card
		existingCards, err := cis.repos.Card.GetByName(ctx, parsed.Name)
		if err == nil && len(existingCards) > 0 {
			// Handle existing card based on overwrite mode
			handled := cis.handleExistingCard(ctx, req, parsed, existingCards, result)
			if !handled {
				result.FilesSkipped = append(result.FilesSkipped, file.Name)
				continue
			}
		}

		// Upload file to DigitalOcean Spaces
		if err := cis.uploadFileToSpaces(ctx, file, parsed, req.GroupType, req.CollectionID); err != nil {
			result.ProcessingErrors = append(result.ProcessingErrors,
				webmodels.CreateProcessingError(file.Name, "upload", "spaces_error", err.Error(), true))
			continue
		}

		uploadedFiles = append(uploadedFiles, file.Name)
		result.FilesUploaded = append(result.FilesUploaded, file.Name)

		// Prepare card for database creation
		card := &models.Card{
			Name:      parsed.Name,
			Level:     parsed.Level,
			Animated:  parsed.IsAnimated,
			ColID:     req.CollectionID,
			Tags:      []string{req.GroupType},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		cardsToCreate = append(cardsToCreate, card)

		// Update statistics
		result.ImportSummary.LevelStats[parsed.Level]++
		result.ImportSummary.FileTypeStats[parsed.Extension]++
	}

	// Create cards in database with transaction
	if len(cardsToCreate) > 0 {
		// Get next card ID
		lastID, err := cis.repos.Card.GetLastCardID(ctx)
		if err != nil {
			return fmt.Errorf("failed to get last card ID: %w", err)
		}

		nextID := lastID + 1
		for i, card := range cardsToCreate {
			card.ID = nextID + int64(i)
		}

		// Batch create cards
		if err := cis.repos.Card.BatchCreateWithTransaction(ctx, tx, cardsToCreate); err != nil {
			// Cleanup uploaded files on database failure
			cis.cleanupUploadedFiles(ctx, uploadedFiles, req.GroupType, req.CollectionID)
			return fmt.Errorf("failed to create cards in database: %w", err)
		}

		result.CardsCreated = len(cardsToCreate)
		result.FirstCardID = cardsToCreate[0].ID
		result.LastCardID = cardsToCreate[len(cardsToCreate)-1].ID
	}

	result.ImportSummary.ProcessedFiles = len(uploadedFiles)
	result.ImportSummary.FailedFiles = result.ImportSummary.TotalFiles - result.ImportSummary.ProcessedFiles - len(result.FilesSkipped)

	// Commit transaction
	if err := tx.Commit(); err != nil {
		// Cleanup uploaded files on commit failure
		cis.cleanupUploadedFiles(ctx, uploadedFiles, req.GroupType, req.CollectionID)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// handleExistingCard handles existing cards based on overwrite mode
func (cis *CardImportService) handleExistingCard(ctx context.Context, req *webmodels.CardImportRequest, parsed *webmodels.ParsedFilename, existingCards []*models.Card, result *webmodels.CardImportResult) bool {
	switch req.OverwriteMode {
	case "skip":
		result.CardsSkipped++
		return false
	case "overwrite":
		// Delete existing card(s) - will be recreated
		for _, existing := range existingCards {
			if err := cis.repos.Card.Delete(ctx, existing.ID); err != nil {
				result.ProcessingErrors = append(result.ProcessingErrors,
					webmodels.CreateProcessingError(parsed.Original, "database", "delete_error", 
						fmt.Sprintf("Failed to delete existing card %d: %v", existing.ID, err), true))
				return false
			}
		}
		return true
	case "update":
		// Update existing card properties if needed
		for _, existing := range existingCards {
			updated := false
			if existing.Level != parsed.Level {
				existing.Level = parsed.Level
				updated = true
			}
			if existing.Animated != parsed.IsAnimated {
				existing.Animated = parsed.IsAnimated
				updated = true
			}
			if updated {
				existing.UpdatedAt = time.Now()
				if err := cis.repos.Card.Update(ctx, existing); err != nil {
					result.ProcessingErrors = append(result.ProcessingErrors,
						webmodels.CreateProcessingError(parsed.Original, "database", "update_error",
							fmt.Sprintf("Failed to update existing card %d: %v", existing.ID, err), true))
					return false
				}
				result.CardsUpdated++
			} else {
				result.CardsSkipped++
			}
		}
		return false // Don't create new card
	default:
		return false
	}
}

// uploadFileToSpaces uploads a file to DigitalOcean Spaces
func (cis *CardImportService) uploadFileToSpaces(ctx context.Context, file *webmodels.FileUpload, parsed *webmodels.ParsedFilename, groupType, collectionID string) error {
	// Create a temporary card for spaces service
	tempCard := &models.Card{
		Name:     parsed.Name,
		Level:    parsed.Level,
		Animated: parsed.IsAnimated,
		ColID:    collectionID,
		Tags:     []string{groupType},
	}

	// Use the existing spaces service to upload
	result, err := cis.spacesService.ManageCardImage(ctx, services.ImageOperationUpload, 0, file.Data, tempCard)
	if err != nil {
		return fmt.Errorf("spaces upload failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("spaces upload failed: %s", result.ErrorMessage)
	}

	return nil
}

// cleanupUploadedFiles removes uploaded files if database operation fails
func (cis *CardImportService) cleanupUploadedFiles(ctx context.Context, uploadedFiles []string, groupType, collectionID string) {
	for _, fileName := range uploadedFiles {
		parsed, err := cis.ParseFilename(fileName)
		if err != nil {
			continue
		}

		tempCard := &models.Card{
			Name:     parsed.Name,
			Level:    parsed.Level,
			Animated: parsed.IsAnimated,
			ColID:    collectionID,
			Tags:     []string{groupType},
		}

		// Attempt to delete the uploaded file
		_, err = cis.spacesService.ManageCardImage(ctx, services.DeleteOperation, 0, nil, tempCard)
		if err != nil {
			slog.Error("Failed to cleanup uploaded file",
				slog.String("file_name", fileName),
				slog.String("error", err.Error()))
		}
	}
}

// ValidateImportRequest validates import request without processing
func (cis *CardImportService) ValidateImportRequest(ctx context.Context, req *webmodels.CardImportRequest) (*webmodels.CardImportResult, error) {
	req.ValidateOnly = true
	return cis.ImportCards(ctx, req)
}