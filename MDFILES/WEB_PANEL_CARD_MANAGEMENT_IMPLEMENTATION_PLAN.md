# Web Panel Card Management Implementation Plan

## Project Overview

This document outlines the implementation plan for integrating automated card management into the existing GoHYE Discord bot web panel. The goal is to replace the manual Python automation scripts with a robust, user-friendly web interface that maintains all existing functionality while adding comprehensive validation and error handling.

## Current System Analysis

### Existing Python Automation Scripts

#### `/MDFILES/automate.py` (Primary Script - MongoDB)
- **Purpose**: Batch upload cards to DigitalOcean Spaces and MongoDB
- **Flow**: Validate files → Upload to DO → Update MongoDB
- **File Format**: `level_name_additional.ext` (e.g., `1_hello.jpg`, `1_member1_member2.jpg`)
- **Collections**: Supports girlgroups/boygroups with auto-collection creation
- **Database**: Uses MongoDB with next ID generation

#### `/MDFILES/promoauto.py` (Legacy Script - JSON)
- **Purpose**: Similar to automate.py but for JSON-based storage
- **Flow**: Validate files → Upload to DO → Update JSON files
- **Collections**: Supports regular and promo collections

### Current Web Panel Architecture

#### Backend (`/backend/`)
- **Go-based API** with existing card management service
- **PostgreSQL database** with Bun ORM
- **Repository pattern** for data access
- **Existing models**: Card, Collection, UserCard with proper relationships

#### Frontend (`/frontend/`)
- **Next.js application** with existing card management UI
- **TypeScript with Tailwind CSS**
- **Existing components**: Cards table, forms, bulk operations

### Current Bot Database Schema

#### Card Model
```go
type Card struct {
    ID        int64     `bun:"id,pk"`
    Name      string    `bun:"name,notnull"`
    Level     int       `bun:"level,notnull"`           // 1-5 rarity levels
    Animated  bool      `bun:"animated,notnull"`        // GIF support
    ColID     string    `bun:"col_id,notnull"`          // Foreign key to collections
    Tags      []string  `bun:"tags,type:jsonb"`         // Group type tags
    CreatedAt time.Time `bun:"created_at,notnull"`
    UpdatedAt time.Time `bun:"updated_at,notnull"`
}
```

## Implementation Plan

### Phase 1: Backend Service Extensions

#### 1.1 Extend Models (`/backend/models/web_models.go`)

```go
// Add new models for bulk import operations
type CardImportRequest struct {
    CollectionID   string         `json:"collection_id" validate:"required"`
    DisplayName    string         `json:"display_name" validate:"required"`
    GroupType      string         `json:"group_type" validate:"required,oneof=girlgroups boygroups"`
    IsPromo        bool           `json:"is_promo"`
    Files          []*FileUpload  `json:"files" validate:"required,min=1"`
    ValidateOnly   bool           `json:"validate_only"`
}

type FileUpload struct {
    Name        string `json:"name"`
    Size        int64  `json:"size"`
    ContentType string `json:"content_type"`
    Data        []byte `json:"data"`
}

type CardImportResult struct {
    CollectionID     string            `json:"collection_id"`
    CardsCreated     int               `json:"cards_created"`
    FirstCardID      int64             `json:"first_card_id"`
    LastCardID       int64             `json:"last_card_id"`
    FilesUploaded    []string          `json:"files_uploaded"`
    ValidationErrors []ValidationError `json:"validation_errors"`
    Success          bool              `json:"success"`
    ErrorMessage     string            `json:"error_message,omitempty"`
}

type ValidationError struct {
    FileName    string `json:"file_name"`
    ErrorType   string `json:"error_type"`
    Description string `json:"description"`
    Severity    string `json:"severity"`
}

type ParsedFilename struct {
    Level      int    `json:"level"`
    Name       string `json:"name"`
    Extension  string `json:"extension"`
    IsAnimated bool   `json:"is_animated"`
    Original   string `json:"original"`
    Normalized string `json:"normalized"`
}
```

#### 1.2 Create Card Import Service (`/backend/services/card_import.go`)

**Core Features:**
- File validation pipeline matching Python script logic
- Integration with existing SpacesService for DO uploads
- Transaction-based database operations with rollback support
- Support for both single collection and batch collection imports

**Key Methods:**
```go
type CardImportService struct {
    repos         *webmodels.Repositories
    spacesService *services.SpacesService
    cardService   *CardManagementService
}

// Main import pipeline
func (cis *CardImportService) ImportCards(ctx context.Context, req *CardImportRequest) (*CardImportResult, error)

// Validation pipeline
func (cis *CardImportService) ValidateFiles(files []*FileUpload) ([]ValidationError, error)
func (cis *CardImportService) ParseFilename(filename string) (*ParsedFilename, error)
func (cis *CardImportService) ValidateFilename(filename string) error

// Processing pipeline
func (cis *CardImportService) ProcessCollection(ctx context.Context, req *CardImportRequest) (*CardImportResult, error)
func (cis *CardImportService) UploadCardImages(ctx context.Context, files []*FileUpload, collectionID, groupType string) error
func (cis *CardImportService) CreateCardsInDatabase(ctx context.Context, cards []*models.Card) error
```

#### 1.3 Add API Endpoints (`/backend/handlers/handlers.go`)

```go
// Bulk import endpoints
POST /api/cards/import/validate    - Validate files without processing
POST /api/cards/import             - Full import pipeline
POST /api/cards/import/collections - Batch import multiple collections

// Enhanced bulk operations
POST /api/cards/bulk/move          - Move cards between collections
POST /api/cards/bulk/update-levels - Bulk update card levels
POST /api/cards/bulk/toggle-animated - Toggle animated status

// Sync and maintenance
GET  /api/cards/sync/status        - Check sync status between DB and storage
POST /api/cards/sync/repair        - Repair sync issues
```

### Phase 2: File Validation System

#### 2.1 Filename Validation
**Pattern Support:**
- `1_hello.jpg` (single name)
- `1_member1_member2_member3.jpg` (multiple members)
- Support for `.jpg`, `.png`, `.jpeg`, `.gif`
- Automatic animated detection for `.gif` files

```go
func (cis *CardImportService) ParseFilename(filename string) (*ParsedFilename, error) {
    // Regex pattern: ^\d+_\w+(\_\w+)*\.(jpg|jpeg|png|gif)$
    pattern := regexp.MustCompile(`^(\d+)_(.+)\.(jpg|jpeg|png|gif)$`)
    
    matches := pattern.FindStringSubmatch(strings.ToLower(filename))
    if len(matches) != 4 {
        return nil, fmt.Errorf("invalid filename format")
    }
    
    level, _ := strconv.Atoi(matches[1])
    name := strings.ReplaceAll(matches[2], "_", " ")
    extension := matches[3]
    isAnimated := extension == "gif"
    
    return &ParsedFilename{
        Level:      level,
        Name:       name,
        Extension:  extension,
        IsAnimated: isAnimated,
        Original:   filename,
        Normalized: fmt.Sprintf("%d_%s", level, strings.ReplaceAll(name, " ", "_")),
    }, nil
}
```

#### 2.2 File Content Validation
- MIME type verification
- File size limits (configurable)
- Image format validation
- Corruption detection

#### 2.3 Database Validation
- Collection existence checks
- Duplicate card name detection
- Level range validation (1-5)
- Tag consistency validation

### Phase 3: DigitalOcean Spaces Integration

#### 3.1 Extend Existing SpacesService
**File Path Structure:**
```
cards/{group_type}/{collection_id}/{level}_{name}.{ext}
```

**New Methods:**
```go
func (s *SpacesService) UploadCardBatch(ctx context.Context, files []CardFile, collectionID, groupType string) error
func (s *SpacesService) ValidateImageFormat(data []byte, contentType string) error
func (s *SpacesService) GenerateCardImageURL(cardName, collectionID string, level int, groupType string, animated bool) string
```

#### 3.2 Image Processing Pipeline
- Automatic format conversion (PNG → JPG for static images)
- GIF preservation for animated cards
- Image optimization and compression
- CDN URL generation

### Phase 4: Frontend Implementation

#### 4.1 Card Import UI (`/frontend/src/components/import/`)

**Components:**
- `CardImportWizard.tsx` - Multi-step import process
- `FileDropZone.tsx` - Drag & drop file upload
- `FileValidationResults.tsx` - Real-time validation feedback
- `ImportProgress.tsx` - Upload and processing progress
- `ImportSummary.tsx` - Results and summary

**Features:**
- Real-time file validation
- Progress tracking with WebSocket support
- Preview generation for uploaded images
- Error highlighting and suggestions
- Batch processing with pause/resume

#### 4.2 Enhanced Card Management (`/frontend/src/components/cards/`)

**New Components:**
- `CardBulkOperations.tsx` - Enhanced bulk operations
- `CardLevelEditor.tsx` - Inline level editing
- `CollectionMover.tsx` - Move cards between collections
- `SyncStatusMonitor.tsx` - Real-time sync monitoring

**Enhanced Features:**
- Advanced filtering and sorting
- Multi-select with keyboard shortcuts
- Undo/redo operations
- Export functionality

#### 4.3 Real-time Updates
- WebSocket connection for progress updates
- Real-time sync status monitoring
- Live validation feedback
- Error notifications with retry options

### Phase 5: Error Handling & Rollback

#### 5.1 Transaction Management
```go
func (cis *CardImportService) ImportCardsWithTransaction(ctx context.Context, req *CardImportRequest) error {
    tx, err := cis.repos.DB.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Step 1: Validate all files
    if err := cis.validateAllFiles(req.Files); err != nil {
        return err
    }
    
    // Step 2: Upload to DigitalOcean Spaces
    uploadedFiles, err := cis.uploadToSpaces(ctx, req)
    if err != nil {
        cis.cleanupUploads(ctx, uploadedFiles) // Cleanup on failure
        return err
    }
    
    // Step 3: Create database entries
    if err := cis.createCardsInDatabase(ctx, tx, req); err != nil {
        cis.cleanupUploads(ctx, uploadedFiles) // Cleanup on failure
        return err
    }
    
    return tx.Commit()
}
```

#### 5.2 Recovery Mechanisms
- Automatic cleanup of partial uploads
- Database rollback on transaction failures
- Retry logic for network failures
- Manual recovery tools for admins

#### 5.3 Comprehensive Logging
```go
type ImportAuditLog struct {
    ID           int64     `bun:"id,pk,autoincrement"`
    UserID       string    `bun:"user_id,notnull"`
    Operation    string    `bun:"operation,notnull"`
    CollectionID string    `bun:"collection_id"`
    FilesCount   int       `bun:"files_count"`
    CardsCreated int       `bun:"cards_created"`
    Success      bool      `bun:"success"`
    ErrorMessage string    `bun:"error_message"`
    Duration     int64     `bun:"duration_ms"`
    CreatedAt    time.Time `bun:"created_at,notnull"`
}
```

### Phase 6: Testing & Validation

#### 6.1 Unit Tests
- File validation logic
- Filename parsing
- Database operations
- Error handling scenarios

#### 6.2 Integration Tests
- End-to-end import pipeline
- Rollback scenarios
- Concurrent operations
- Large file handling

#### 6.3 Performance Testing
- Bulk upload performance
- Database transaction limits
- Memory usage during large imports
- DigitalOcean Spaces upload limits

## Implementation Timeline

### Week 1: Backend Foundation
- [ ] Extend models and DTOs
- [ ] Create CardImportService skeleton
- [ ] Add basic API endpoints
- [ ] Implement file validation logic

### Week 2: Core Processing
- [ ] Complete import service implementation
- [ ] Integrate with existing SpacesService
- [ ] Add transaction management
- [ ] Implement error handling

### Week 3: Frontend Development
- [ ] Create import wizard UI
- [ ] Implement file upload components
- [ ] Add progress tracking
- [ ] Create validation feedback

### Week 4: Integration & Testing
- [ ] End-to-end testing
- [ ] Performance optimization
- [ ] Error scenario testing
- [ ] Documentation and deployment

## Risk Mitigation

### Data Integrity
- **Risk**: Database corruption during bulk imports
- **Mitigation**: Transaction-based operations with rollback
- **Monitoring**: Comprehensive audit logging

### File Storage
- **Risk**: Orphaned files in DigitalOcean Spaces
- **Mitigation**: Cleanup procedures and sync monitoring
- **Recovery**: Manual cleanup tools

### Performance
- **Risk**: UI blocking during large uploads
- **Mitigation**: Background processing with WebSocket updates
- **Optimization**: Chunked uploads and progress tracking

### User Experience
- **Risk**: Complex interface overwhelming users
- **Mitigation**: Wizard-based approach with clear steps
- **Support**: Detailed error messages and help text

## Backward Compatibility

### Existing Functionality
- All existing card management features remain unchanged
- Current API endpoints continue to work
- Database schema additions only (no breaking changes)
- Frontend components are additive, not replacements

### Migration Strategy
- Gradual rollout with feature flags
- Parallel testing with Python scripts
- Rollback plan to previous system
- Training for admin users

## Success Metrics

### Functionality
- [ ] 100% feature parity with Python scripts
- [ ] Zero data loss during imports
- [ ] Sub-30 second processing for typical collections
- [ ] 99.9% import success rate

### User Experience
- [ ] <5 click workflow for typical imports
- [ ] Real-time progress feedback
- [ ] Clear error messages and recovery steps
- [ ] Mobile-responsive interface

### System Health
- [ ] <1% orphaned files in storage
- [ ] 100% database-storage sync
- [ ] Comprehensive audit trails
- [ ] Automated monitoring and alerts

## Conclusion

This implementation plan provides a comprehensive roadmap for replacing the manual Python automation scripts with a robust, user-friendly web panel system. The plan maintains backward compatibility while significantly improving the user experience and system reliability.

The phased approach ensures that each component is thoroughly tested before moving to the next phase, minimizing risk and ensuring a successful deployment.