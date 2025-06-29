package utils

import (
	"fmt"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/disgoorg/bot-template/backend/models"
)

var (
	// ValidImageExtensions contains valid image file extensions
	ValidImageExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
	
	// MaxImageSize defines maximum image size (10MB)
	MaxImageSize int64 = 10 * 1024 * 1024
	
	// ValidCardNameRegex validates card names
	ValidCardNameRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-_()]+$`)
	
	// ValidCollectionIDRegex validates collection IDs
	ValidCollectionIDRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
)

// ValidateCardCreateRequest validates a card creation request
func ValidateCardCreateRequest(req *models.CardCreateRequest) []models.ValidationError {
	var errors []models.ValidationError
	
	// Validate name
	if req.Name == "" {
		errors = append(errors, models.ValidationError{
			FileName:    "name",
			ErrorType:   "validation",
			Description: "Name is required",
			Severity:    "critical",
		})
	} else if len(req.Name) > 100 {
		errors = append(errors, models.ValidationError{
			FileName:    "name",
			ErrorType:   "validation",
			Description: "Name must be less than 100 characters",
			Severity:    "high",
		})
	} else if !ValidCardNameRegex.MatchString(req.Name) {
		errors = append(errors, models.ValidationError{
			FileName:    "name",
			ErrorType:   "validation",
			Description: "Name contains invalid characters",
			Severity:    "high",
		})
	}
	
	// Validate level
	if req.Level < 1 || req.Level > 5 {
		errors = append(errors, models.ValidationError{
			FileName:    "level",
			ErrorType:   "validation",
			Description: "Level must be between 1 and 5",
			Severity:    "critical",
		})
	}
	
	// Validate collection ID
	if req.ColID == "" {
		errors = append(errors, models.ValidationError{
			FileName:    "col_id",
			ErrorType:   "validation",
			Description: "Collection ID is required",
			Severity:    "critical",
		})
	} else if !ValidCollectionIDRegex.MatchString(req.ColID) {
		errors = append(errors, models.ValidationError{
			FileName:    "col_id",
			ErrorType:   "validation",
			Description: "Collection ID contains invalid characters",
			Severity:    "high",
		})
	}
	
	// Validate tags
	for i, tag := range req.Tags {
		if len(tag) > 50 {
			errors = append(errors, models.ValidationError{
				FileName:    fmt.Sprintf("tags[%d]", i),
				ErrorType:   "validation",
				Description: "Tag must be less than 50 characters",
				Severity:    "medium",
			})
		}
	}
	
	return errors
}

// ValidateCardUpdateRequest validates a card update request
func ValidateCardUpdateRequest(req *models.CardUpdateRequest) []models.ValidationError {
	var errors []models.ValidationError
	
	// Validate name if provided
	if req.Name != nil {
		if *req.Name == "" {
			errors = append(errors, models.ValidationError{
				FileName:    "name",
				ErrorType:   "validation",
				Description: "Name cannot be empty",
				Severity:    "critical",
			})
		} else if len(*req.Name) > 100 {
			errors = append(errors, models.ValidationError{
				FileName:    "name",
				ErrorType:   "validation",
				Description: "Name must be less than 100 characters",
				Severity:    "high",
			})
		} else if !ValidCardNameRegex.MatchString(*req.Name) {
			errors = append(errors, models.ValidationError{
				FileName:    "name",
				ErrorType:   "validation",
				Description: "Name contains invalid characters",
				Severity:    "high",
			})
		}
	}
	
	// Validate level if provided
	if req.Level != nil {
		if *req.Level < 1 || *req.Level > 5 {
			errors = append(errors, models.ValidationError{
				FileName:    "level",
				ErrorType:   "validation",
				Description: "Level must be between 1 and 5",
				Severity:    "critical",
			})
		}
	}
	
	// Validate collection ID if provided
	if req.ColID != nil {
		if *req.ColID == "" {
			errors = append(errors, models.ValidationError{
				FileName:    "col_id",
				ErrorType:   "validation",
				Description: "Collection ID cannot be empty",
				Severity:    "critical",
			})
		} else if !ValidCollectionIDRegex.MatchString(*req.ColID) {
			errors = append(errors, models.ValidationError{
				FileName:    "col_id",
				ErrorType:   "validation",
				Description: "Collection ID contains invalid characters",
				Severity:    "high",
			})
		}
	}
	
	// Validate tags if provided
	if req.Tags != nil {
		for i, tag := range req.Tags {
			if len(tag) > 50 {
				errors = append(errors, models.ValidationError{
					FileName:    fmt.Sprintf("tags[%d]", i),
					ErrorType:   "validation",
					Description: "Tag must be less than 50 characters",
					Severity:    "medium",
				})
			}
		}
	}
	
	return errors
}

// ValidateImageFile validates an uploaded image file
func ValidateImageFile(fileHeader *multipart.FileHeader) []models.ValidationError {
	var errors []models.ValidationError
	
	// Check file size
	if fileHeader.Size > MaxImageSize {
		errors = append(errors, models.ValidationError{
			FileName:    fileHeader.Filename,
			ErrorType:   "file_size",
			Description: fmt.Sprintf("Image size must be less than %dMB", MaxImageSize/(1024*1024)),
			Severity:    "critical",
		})
	}
	
	// Check file extension
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	validExt := false
	for _, validExtension := range ValidImageExtensions {
		if ext == validExtension {
			validExt = true
			break
		}
	}
	
	if !validExt {
		errors = append(errors, models.ValidationError{
			FileName:    fileHeader.Filename,
			ErrorType:   "file_format",
			Description: fmt.Sprintf("Invalid image format. Allowed formats: %s", strings.Join(ValidImageExtensions, ", ")),
			Severity:    "critical",
			Suggestion:  "Please use JPG, PNG, GIF, or WebP format",
		})
	}
	
	return errors
}


// ValidateBulkOperation validates a bulk operation request
func ValidateBulkOperation(req *models.CardBulkOperation) []models.ValidationError {
	var errors []models.ValidationError
	
	// Validate operation
	validOperations := []string{"delete", "update", "move", "export"}
	validOp := false
	for _, op := range validOperations {
		if req.Operation == op {
			validOp = true
			break
		}
	}
	
	if !validOp {
		errors = append(errors, models.ValidationError{
			FileName:    "operation",
			ErrorType:   "validation",
			Description: fmt.Sprintf("Invalid operation. Must be one of: %s", strings.Join(validOperations, ", ")),
			Severity:    "critical",
		})
	}
	
	// Validate card IDs
	if len(req.CardIDs) == 0 {
		errors = append(errors, models.ValidationError{
			FileName:    "card_ids",
			ErrorType:   "validation",
			Description: "At least one card ID is required",
			Severity:    "critical",
		})
	} else if len(req.CardIDs) > 1000 {
		errors = append(errors, models.ValidationError{
			FileName:    "card_ids",
			ErrorType:   "validation",
			Description: "Maximum 1000 cards allowed per bulk operation",
			Severity:    "high",
		})
	}
	
	// Validate updates for update operation
	if req.Operation == "update" && req.Updates == nil {
		errors = append(errors, models.ValidationError{
			FileName:    "updates",
			ErrorType:   "validation",
			Description: "Updates are required for update operation",
			Severity:    "critical",
		})
	}
	
	// Validate target collection for move operation
	if req.Operation == "move" && req.TargetCollection == "" {
		errors = append(errors, models.ValidationError{
			FileName:    "target_collection",
			ErrorType:   "validation",
			Description: "Target collection is required for move operation",
			Severity:    "critical",
		})
	}
	
	return errors
}

// SanitizeFilename sanitizes a filename for safe storage
func SanitizeFilename(filename string) string {
	// Remove path separators and dangerous characters
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, "..", "_")
	filename = strings.ReplaceAll(filename, " ", "_")
	
	// Remove any non-alphanumeric characters except dots, dashes, and underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9.\-_]`)
	filename = reg.ReplaceAllString(filename, "_")
	
	return filename
}

// GenerateCardSlug generates a URL-safe slug from card name
func GenerateCardSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	
	// Replace spaces and special characters with dashes
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	
	// Remove leading and trailing dashes
	slug = strings.Trim(slug, "-")
	
	return slug
}