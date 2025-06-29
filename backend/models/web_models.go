package models

import (
	"fmt"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
)

// UserSession represents a user session for web authentication
type UserSession struct {
	DiscordID   string    `json:"discord_id"`
	Username    string    `json:"username"`
	Avatar      string    `json:"avatar"`
	Email       string    `json:"email"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	ExpiresAt   time.Time `json:"expires_at"`
	IsAdmin     bool      `json:"is_admin"`
}

// CardDTO represents a card data transfer object for web UI
type CardDTO struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Level        int       `json:"level"`
	Animated     bool      `json:"animated"`
	ColID        string    `json:"col_id"`
	CollectionName string  `json:"collection_name"`
	Tags         []string  `json:"tags"`
	ImageURL     string    `json:"image_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CollectionDTO represents a collection data transfer object
type CollectionDTO struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	CollectionType string    `json:"collection_type"`
	Origin         string    `json:"origin"`
	Aliases        []string  `json:"aliases"`
	Promo          bool      `json:"promo"`
	Compressed     bool      `json:"compressed"`
	Fragments      bool      `json:"fragments"`
	Tags           []string  `json:"tags"`
	CardCount      int       `json:"card_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CardSearchRequest represents search parameters for cards
type CardSearchRequest struct {
	Query      string `json:"query" form:"query"`
	Collection string `json:"collection" form:"collection"`
	Level      int    `json:"level" form:"level"`
	Animated   *bool  `json:"animated" form:"animated"`
	Tags       []string `json:"tags" form:"tags"`
	Page       int    `json:"page" form:"page"`
	Limit      int    `json:"limit" form:"limit"`
	SortBy     string `json:"sort_by" form:"sort_by"`
	SortOrder  string `json:"sort_order" form:"sort_order"`
}

// CardCreateRequest represents a request to create a new card
type CardCreateRequest struct {
	Name       string   `json:"name" validate:"required,min=1,max=100"`
	Level      int      `json:"level" validate:"required,min=1,max=5"`
	Animated   bool     `json:"animated"`
	ColID      string   `json:"col_id" validate:"required"`
	Tags       []string `json:"tags"`
	ImageData  []byte   `json:"image_data,omitempty"`
	ImageName  string   `json:"image_name,omitempty"`
}

// CardUpdateRequest represents a request to update a card
type CardUpdateRequest struct {
	Name      *string  `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Level     *int     `json:"level,omitempty" validate:"omitempty,min=1,max=5"`
	Animated  *bool    `json:"animated,omitempty"`
	ColID     *string  `json:"col_id,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	ImageData []byte   `json:"image_data,omitempty"`
	ImageName string   `json:"image_name,omitempty"`
}

// CardBulkOperation represents a bulk operation request
type CardBulkOperation struct {
	Operation string   `json:"operation" validate:"required,oneof=delete update move export"`
	CardIDs   []int64  `json:"card_ids" validate:"required,min=1"`
	Updates   *CardUpdateRequest `json:"updates,omitempty"`
	TargetCollection string `json:"target_collection,omitempty"`
}

// CollectionImportRequest represents a collection import request
type CollectionImportRequest struct {
	CollectionID string     `json:"collection_id" validate:"required"`
	DisplayName  string     `json:"display_name" validate:"required"`
	GroupType    string     `json:"group_type" validate:"required,oneof=girlgroups boygroups"`
	IsPromo      bool       `json:"is_promo"`
	Files        []*FileUpload `json:"files" validate:"required,min=1"`
}

// FileUpload represents an uploaded file
type FileUpload struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

// CollectionImportResult represents the result of a collection import
type CollectionImportResult struct {
	CollectionID  string   `json:"collection_id"`
	CardsCreated  int      `json:"cards_created"`
	FirstCardID   int64    `json:"first_card_id"`
	LastCardID    int64    `json:"last_card_id"`
	FilesUploaded []string `json:"files_uploaded"`
	Success       bool     `json:"success"`
	ErrorMessage  string   `json:"error_message,omitempty"`
}

// ParsedFilename represents a parsed filename
type ParsedFilename struct {
	Level      int    `json:"level"`
	Name       string `json:"name"`
	Extension  string `json:"extension"`
	IsAnimated bool   `json:"is_animated"`
	Original   string `json:"original"`
	Normalized string `json:"normalized"`
	Valid      bool   `json:"valid"`
	ErrorMsg   string `json:"error_msg,omitempty"`
}

// ValidationError represents a file validation error
type ValidationError struct {
	FileName    string `json:"file_name"`
	ErrorType   string `json:"error_type"`   // format, size, duplicate, invalid_level, etc.
	Description string `json:"description"`
	Severity    string `json:"severity"`     // low, medium, high, critical
	Suggestion  string `json:"suggestion,omitempty"`
}

// CardImportRequest represents an enhanced card import request
type CardImportRequest struct {
	CollectionID   string         `json:"collection_id" validate:"required"`
	DisplayName    string         `json:"display_name" validate:"required"`
	GroupType      string         `json:"group_type" validate:"required,oneof=girlgroups boygroups"`
	IsPromo        bool           `json:"is_promo"`
	Files          []*FileUpload  `json:"files" validate:"required,min=1"`
	ValidateOnly   bool           `json:"validate_only"`
	OverwriteMode  string         `json:"overwrite_mode" validate:"oneof=skip overwrite update"` // skip, overwrite, update
	CreateCollection bool         `json:"create_collection"` // Auto-create collection if not exists
}

// CardImportResult represents enhanced import results
type CardImportResult struct {
	CollectionID       string             `json:"collection_id"`
	CollectionCreated  bool               `json:"collection_created"`
	CardsCreated       int                `json:"cards_created"`
	CardsSkipped       int                `json:"cards_skipped"`
	CardsUpdated       int                `json:"cards_updated"`
	FirstCardID        int64              `json:"first_card_id"`
	LastCardID         int64              `json:"last_card_id"`
	FilesUploaded      []string           `json:"files_uploaded"`
	FilesSkipped       []string           `json:"files_skipped"`
	ValidationErrors   []ValidationError  `json:"validation_errors"`
	ProcessingErrors   []ProcessingError  `json:"processing_errors"`
	Success            bool               `json:"success"`
	PartialSuccess     bool               `json:"partial_success"`
	ErrorMessage       string             `json:"error_message,omitempty"`
	ProcessingTimeMs   int64              `json:"processing_time_ms"`
	ImportSummary      *ImportSummary     `json:"import_summary,omitempty"`
}

// ProcessingError represents an error during processing
type ProcessingError struct {
	FileName    string `json:"file_name"`
	Stage       string `json:"stage"`       // validation, upload, database
	ErrorType   string `json:"error_type"`
	Description string `json:"description"`
	Recoverable bool   `json:"recoverable"`
}

// ImportSummary provides a summary of the import operation
type ImportSummary struct {
	TotalFiles     int                    `json:"total_files"`
	ValidFiles     int                    `json:"valid_files"`
	InvalidFiles   int                    `json:"invalid_files"`
	ProcessedFiles int                    `json:"processed_files"`
	FailedFiles    int                    `json:"failed_files"`
	LevelStats     map[int]int            `json:"level_stats"`    // level -> count
	FileTypeStats  map[string]int         `json:"file_type_stats"` // extension -> count
	Duplicates     []string               `json:"duplicates"`
	LargeFiles     []string               `json:"large_files"`    // Files over size limit
}

// CardBatchOperation represents enhanced bulk operations
type CardBatchOperation struct {
	Operation        string             `json:"operation" validate:"required,oneof=delete update move export level_update toggle_animated"`
	CardIDs          []int64            `json:"card_ids" validate:"required,min=1"`
	Updates          *CardUpdateRequest `json:"updates,omitempty"`
	TargetCollection string             `json:"target_collection,omitempty"`
	NewLevel         *int               `json:"new_level,omitempty" validate:"omitempty,min=1,max=5"`
	DryRun           bool               `json:"dry_run"` // Preview operation without executing
}

// CardBatchResult represents the result of batch operations
type CardBatchResult struct {
	Operation       string               `json:"operation"`
	TotalCards      int                  `json:"total_cards"`
	ProcessedCards  int                  `json:"processed_cards"`
	FailedCards     int                  `json:"failed_cards"`
	Errors          []CardOperationError `json:"errors"`
	Success         bool                 `json:"success"`
	DryRun          bool                 `json:"dry_run"`
	PreviewResults  []CardPreview        `json:"preview_results,omitempty"`
}

// CardOperationError represents an error in card operations
type CardOperationError struct {
	CardID      int64  `json:"card_id"`
	CardName    string `json:"card_name"`
	ErrorType   string `json:"error_type"`
	Description string `json:"description"`
}

// CardPreview represents a preview of what will happen to a card
type CardPreview struct {
	CardID     int64                  `json:"card_id"`
	CardName   string                 `json:"card_name"`
	Changes    map[string]interface{} `json:"changes"`
	Warnings   []string               `json:"warnings"`
}

// ImportAuditLog represents audit logging for import operations
type ImportAuditLog struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	Operation    string    `json:"operation"`
	CollectionID string    `json:"collection_id"`
	FilesCount   int       `json:"files_count"`
	CardsCreated int       `json:"cards_created"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Duration     int64     `json:"duration_ms"`
	ClientIP     string    `json:"client_ip"`
	UserAgent    string    `json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
}


// SyncStatus represents synchronization status between database and storage
type SyncStatus struct {
	CollectionID   string    `json:"collection_id"`
	CollectionName string    `json:"collection_name"`
	DatabaseCards  int       `json:"database_cards"`
	StorageFiles   int       `json:"storage_files"`
	Status         string    `json:"status"` // synced, missing_files, extra_files, inconsistent
	Issues         []SyncIssue `json:"issues"`
	LastChecked    time.Time `json:"last_checked"`
}

// SyncIssue represents a synchronization issue
type SyncIssue struct {
	Type        string `json:"type"` // missing_file, orphan_file, naming_mismatch
	Description string `json:"description"`
	CardID      *int64 `json:"card_id,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	Severity    string `json:"severity"` // low, medium, high, critical
}

// DashboardStats represents dashboard statistics
type DashboardStats struct {
	TotalCards      int64   `json:"total_cards"`
	TotalCollections int64  `json:"total_collections"`
	TotalUsers      int64   `json:"total_users"`
	SyncPercentage  float64 `json:"sync_percentage"`
	IssueCount      int     `json:"issue_count"`
	RecentActivity  []ActivityItem `json:"recent_activity"`
}

// ActivityItem represents a recent activity item
type ActivityItem struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
	UserID      string    `json:"user_id,omitempty"`
}

// ConvertCardToDTO converts a database card model to DTO
func ConvertCardToDTO(card *models.Card, collection *models.Collection, imageURL string) *CardDTO {
	dto := &CardDTO{
		ID:        card.ID,
		Name:      card.Name,
		Level:     card.Level,
		Animated:  card.Animated,
		ColID:     card.ColID,
		Tags:      card.Tags,
		ImageURL:  imageURL,
		CreatedAt: card.CreatedAt,
		UpdatedAt: card.UpdatedAt,
	}
	
	if collection != nil {
		dto.CollectionName = collection.Name
	}
	
	return dto
}

// ConvertCollectionToDTO converts a database collection model to DTO
func ConvertCollectionToDTO(collection *models.Collection, cardCount int) *CollectionDTO {
	return &CollectionDTO{
		ID:             collection.ID,
		Name:           collection.Name,
		Description:    "",
		CollectionType: "other", // Default value, should be determined by logic
		Origin:         collection.Origin,
		Aliases:        collection.Aliases,
		Promo:          collection.Promo,
		Compressed:     collection.Compressed,
		Fragments:      collection.Fragments,
		Tags:           collection.Tags,
		CardCount:      cardCount,
		CreatedAt:      collection.CreatedAt,
		UpdatedAt:      collection.UpdatedAt,
	}
}

// Validate validates the card search request
func (r *CardSearchRequest) Validate() error {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.Limit < 1 || r.Limit > 100 {
		r.Limit = 20
	}
	if r.SortBy == "" {
		r.SortBy = "id"
	}
	if r.SortOrder == "" {
		r.SortOrder = "asc"
	}
	return nil
}

// Validate validates the card import request
func (r *CardImportRequest) Validate() error {
	if r.CollectionID == "" {
		return fmt.Errorf("collection_id is required")
	}
	if r.DisplayName == "" {
		return fmt.Errorf("display_name is required")
	}
	if r.GroupType != "girlgroups" && r.GroupType != "boygroups" {
		return fmt.Errorf("group_type must be 'girlgroups' or 'boygroups'")
	}
	if len(r.Files) == 0 {
		return fmt.Errorf("at least one file is required")
	}
	if r.OverwriteMode != "" && r.OverwriteMode != "skip" && r.OverwriteMode != "overwrite" && r.OverwriteMode != "update" {
		return fmt.Errorf("overwrite_mode must be 'skip', 'overwrite', or 'update'")
	}
	if r.OverwriteMode == "" {
		r.OverwriteMode = "skip" // Default
	}
	return nil
}

// Validate validates the file upload
func (f *FileUpload) Validate() error {
	if f.Name == "" {
		return fmt.Errorf("file name is required")
	}
	if f.Size <= 0 {
		return fmt.Errorf("file size must be greater than 0")
	}
	if len(f.Data) == 0 {
		return fmt.Errorf("file data is required")
	}
	if f.ContentType == "" {
		return fmt.Errorf("content type is required")
	}
	
	// Check file size limit (10MB)
	const maxFileSize = 10 * 1024 * 1024
	if f.Size > maxFileSize {
		return fmt.Errorf("file size exceeds 10MB limit")
	}
	
	return nil
}

// Validate validates the card batch operation
func (r *CardBatchOperation) Validate() error {
	validOps := []string{"delete", "update", "move", "export", "level_update", "toggle_animated"}
	validOp := false
	for _, op := range validOps {
		if r.Operation == op {
			validOp = true
			break
		}
	}
	if !validOp {
		return fmt.Errorf("operation must be one of: %v", validOps)
	}
	
	if len(r.CardIDs) == 0 {
		return fmt.Errorf("at least one card ID is required")
	}
	
	// Validate operation-specific requirements
	switch r.Operation {
	case "move":
		if r.TargetCollection == "" {
			return fmt.Errorf("target_collection is required for move operation")
		}
	case "level_update":
		if r.NewLevel == nil {
			return fmt.Errorf("new_level is required for level_update operation")
		}
		if *r.NewLevel < 1 || *r.NewLevel > 5 {
			return fmt.Errorf("new_level must be between 1 and 5")
		}
	case "update":
		if r.Updates == nil {
			return fmt.Errorf("updates are required for update operation")
		}
	}
	
	return nil
}

// GetSeverityLevel returns numeric severity for sorting
func (ve ValidationError) GetSeverityLevel() int {
	switch ve.Severity {
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return 0
	}
}

// IsCritical returns true if the validation error is critical
func (ve ValidationError) IsCritical() bool {
	return ve.Severity == "critical"
}

// HasErrors returns true if there are any validation errors
func (r *CardImportResult) HasErrors() bool {
	return len(r.ValidationErrors) > 0 || len(r.ProcessingErrors) > 0
}

// HasCriticalErrors returns true if there are critical validation errors
func (r *CardImportResult) HasCriticalErrors() bool {
	for _, err := range r.ValidationErrors {
		if err.IsCritical() {
			return true
		}
	}
	return false
}

// GetSuccessRate returns the success rate as a percentage
func (r *CardImportResult) GetSuccessRate() float64 {
	if r.ImportSummary == nil || r.ImportSummary.TotalFiles == 0 {
		return 0.0
	}
	return float64(r.ImportSummary.ProcessedFiles) / float64(r.ImportSummary.TotalFiles) * 100
}

// IsValid returns true if the parsed filename is valid
func (pf *ParsedFilename) IsValid() bool {
	return pf.Valid && pf.Level >= 1 && pf.Level <= 5 && pf.Name != ""
}

// GetDisplayName returns a user-friendly display name
func (pf *ParsedFilename) GetDisplayName() string {
	if pf.Name == "" {
		return "Unknown"
	}
	return pf.Name
}

// CreateValidationError creates a new validation error
func CreateValidationError(fileName, errorType, description, severity string) ValidationError {
	return ValidationError{
		FileName:    fileName,
		ErrorType:   errorType,
		Description: description,
		Severity:    severity,
	}
}

// CreateProcessingError creates a new processing error
func CreateProcessingError(fileName, stage, errorType, description string, recoverable bool) ProcessingError {
	return ProcessingError{
		FileName:    fileName,
		Stage:       stage,
		ErrorType:   errorType,
		Description: description,
		Recoverable: recoverable,
	}
}