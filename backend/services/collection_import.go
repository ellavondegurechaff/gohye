package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/disgoorg/bot-template/bottemplate/database/repositories"
	"github.com/disgoorg/bot-template/bottemplate/economy/utils"
	"github.com/disgoorg/bot-template/bottemplate/services"
	webmodels "github.com/disgoorg/bot-template/backend/models"
	"github.com/uptrace/bun"
)

type CollectionImportService struct {
	cardRepo       repositories.CardRepository
	collectionRepo repositories.CollectionRepository
	spacesService  *services.SpacesService
	txManager      *utils.EconomicTransactionManager
}

const FilenamePattern = `^(\d+)_(.+)\.(jpg|png|jpeg|gif)$`

func NewCollectionImportService(
	cardRepo repositories.CardRepository,
	collectionRepo repositories.CollectionRepository,
	spacesService *services.SpacesService,
	txManager *utils.EconomicTransactionManager,
) *CollectionImportService {
	return &CollectionImportService{
		cardRepo:       cardRepo,
		collectionRepo: collectionRepo,
		spacesService:  spacesService,
		txManager:      txManager,
	}
}

func (cis *CollectionImportService) ValidateAndNormalizeFilename(filename string) (*webmodels.ParsedFilename, error) {
	re := regexp.MustCompile(FilenamePattern)
	matches := re.FindStringSubmatch(filename)
	
	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid filename format. Expected: {level}_{name}.{ext}")
	}
	
	level := 0
	if l, err := strconv.Atoi(matches[1]); err == nil {
		level = l
	} else {
		return nil, fmt.Errorf("invalid level in filename")
	}
	
	if level < 1 || level > 5 {
		return nil, fmt.Errorf("level must be between 1 and 5")
	}
	
	name := strings.ToLower(strings.ReplaceAll(matches[2], " ", "_"))
	extension := strings.ToLower(matches[3])
	isAnimated := extension == "gif"
	
	normalized := fmt.Sprintf("%d_%s.%s", level, name, extension)
	
	return &webmodels.ParsedFilename{
		Level:      level,
		Name:       name,
		Extension:  extension,
		IsAnimated: isAnimated,
		Original:   filename,
		Normalized: normalized,
	}, nil
}

func (cis *CollectionImportService) GenerateStoragePath(groupType string, collectionID string, isPromo bool) string {
	basePath := "cards"
	if isPromo {
		basePath = "promo"
	}
	return fmt.Sprintf("%s/%s/%s", basePath, groupType, collectionID)
}

func (cis *CollectionImportService) ProcessCollectionImport(ctx context.Context, req *webmodels.CollectionImportRequest) (*webmodels.CollectionImportResult, error) {
	// 1. Validate all files first
	validatedFiles := make([]*webmodels.ParsedFilename, 0, len(req.Files))
	for _, file := range req.Files {
		parsed, err := cis.ValidateAndNormalizeFilename(file.Name)
		if err != nil {
			return &webmodels.CollectionImportResult{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Invalid file %s: %s", file.Name, err.Error()),
			}, nil
		}
		validatedFiles = append(validatedFiles, parsed)
	}
	
	// 2. Check for duplicate names within the collection
	nameCount := make(map[string]int)
	for _, parsed := range validatedFiles {
		nameCount[parsed.Name]++
		if nameCount[parsed.Name] > 1 {
			return &webmodels.CollectionImportResult{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Duplicate card name found: %s", parsed.Name),
			}, nil
		}
	}
	
	// 3. Get next card ID
	lastID, err := cis.cardRepo.GetLastCardID(ctx)
	if err != nil {
		return &webmodels.CollectionImportResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to get last card ID: %s", err.Error()),
		}, nil
	}
	nextID := lastID + 1
	
	// 4. Ensure collection exists with proper format
	err = cis.ensureCollectionExists(ctx, req.CollectionID, req.DisplayName, req.GroupType, req.IsPromo)
	if err != nil {
		return &webmodels.CollectionImportResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create collection: %s", err.Error()),
		}, nil
	}
	
	// 5. Upload files to Spaces (with cleanup on failure)
	uploadedFiles := make([]string, 0, len(req.Files))
	storagePath := cis.GenerateStoragePath(req.GroupType, req.CollectionID, req.IsPromo)
	
	for i, file := range req.Files {
		parsed := validatedFiles[i]
		spacesPath := fmt.Sprintf("%s/%s", storagePath, parsed.Normalized)
		
		err := cis.uploadFileToSpaces(ctx, file, spacesPath)
		if err != nil {
			// Cleanup all uploaded files
			cis.cleanupUploadedFiles(ctx, uploadedFiles)
			return &webmodels.CollectionImportResult{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Upload failed for %s: %s", file.Name, err.Error()),
			}, nil
		}
		uploadedFiles = append(uploadedFiles, spacesPath)
	}
	
	// 6. Create cards in database transaction
	cards := make([]*models.Card, 0, len(validatedFiles))
	for i, parsed := range validatedFiles {
		card := &models.Card{
			ID:        nextID + int64(i),
			Name:      parsed.Name,
			Level:     parsed.Level,
			Animated:  parsed.IsAnimated,
			ColID:     req.CollectionID,
			Tags:      []string{req.GroupType},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		cards = append(cards, card)
	}
	
	// Use transaction manager for atomic database operations
	err = cis.txManager.WithTransaction(ctx, utils.StandardTransactionOptions(), func(ctx context.Context, tx bun.Tx) error {
		return cis.cardRepo.BatchCreateWithTransaction(ctx, tx, cards)
	})
	
	if err != nil {
		// Cleanup uploaded files and rollback
		cis.cleanupUploadedFiles(ctx, uploadedFiles)
		return &webmodels.CollectionImportResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Database operation failed: %s", err.Error()),
		}, nil
	}
	
	return &webmodels.CollectionImportResult{
		CollectionID:  req.CollectionID,
		CardsCreated:  len(cards),
		FirstCardID:   nextID,
		LastCardID:    nextID + int64(len(cards)) - 1,
		FilesUploaded: uploadedFiles,
		Success:       true,
	}, nil
}

func (cis *CollectionImportService) ensureCollectionExists(ctx context.Context, collectionID, displayName, groupType string, isPromo bool) error {
	// Check if collection already exists
	existing, err := cis.collectionRepo.GetByID(ctx, collectionID)
	if err == nil && existing != nil {
		return nil // Collection exists
	}
	
	// Create new collection with proper format
	return cis.collectionRepo.CreateWithStandardFormat(ctx, collectionID, displayName, groupType, isPromo)
}

func (cis *CollectionImportService) uploadFileToSpaces(ctx context.Context, file *webmodels.FileUpload, spacesPath string) error {
	return cis.spacesService.UploadFile(ctx, file.Data, spacesPath, file.ContentType)
}

func (cis *CollectionImportService) cleanupUploadedFiles(ctx context.Context, filePaths []string) {
	for _, path := range filePaths {
		err := cis.spacesService.DeleteFile(ctx, path)
		if err != nil {
			// Log error but continue cleanup
			fmt.Printf("Failed to cleanup uploaded file %s: %s\n", path, err.Error())
		}
	}
}