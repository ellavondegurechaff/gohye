# Collection Management Implementation Plan

## 🎯 **System Architecture Overview**

### **Collection Creation Format Compliance**
Based on existing collection structure analysis, new collections must follow this exact format:

```json
{
  "id": "collection_id",           // lowercase, no spaces (e.g., "twice", "redvelvet")
  "name": "Display Name",          // proper display name (e.g., "TWICE", "Red Velvet")
  "origin": null,                  // always null for new collections
  "aliases": ["collection_id"],    // array with ID as first element
  "promo": false,                  // true for promo collections, false for regular
  "compressed": true,              // always true (standard format)
  "fragments": false,              // always false (default)
  "tags": ["girlgroups"]          // ["girlgroups"] or ["boygroups"]
}
```

### **Database Model Mapping**
```go
type Collection struct {
    ID         string    `bun:"id,pk"`              // "twice"
    Name       string    `bun:"name,notnull"`       // "TWICE"
    Origin     string    `bun:"origin,notnull"`     // "" (empty string, not null)
    Aliases    []string  `bun:"aliases,type:jsonb"` // ["twice"]
    Promo      bool      `bun:"promo,notnull"`      // false/true
    Compressed bool      `bun:"compressed,notnull"` // true
    Fragments  bool      `bun:"fragments,default:false"` // false
    Tags       []string  `bun:"tags,type:jsonb"`    // ["girlgroups"]
    CreatedAt  time.Time `bun:"created_at,default:current_timestamp"`
    UpdatedAt  time.Time `bun:"updated_at,notnull"`
}
```

---

## 🔧 **Implementation Components**

### **1. Enhanced Card Repository**
**File**: `bottemplate/database/repositories/card_repository.go`

Add missing methods:
```go
// Add to CardRepository interface
GetLastCardID(ctx context.Context) (int64, error)
BatchCreateWithTransaction(ctx context.Context, tx bun.Tx, cards []*models.Card) error

// Implementation
func (r *cardRepository) GetLastCardID(ctx context.Context) (int64, error) {
    var maxID int64
    err := r.db.NewSelect().
        Model((*models.Card)(nil)).
        ColumnExpr("COALESCE(MAX(id), 0)").
        Scan(ctx, &maxID)
    return maxID, err
}

func (r *cardRepository) BatchCreateWithTransaction(ctx context.Context, tx bun.Tx, cards []*models.Card) error {
    if len(cards) == 0 {
        return nil
    }
    
    now := time.Now()
    for _, card := range cards {
        card.CreatedAt = now
        card.UpdatedAt = now
    }
    
    _, err := tx.NewInsert().
        Model(&cards).
        Exec(ctx)
    
    return err
}
```

### **2. Collection Repository Enhancement**
**File**: `bottemplate/database/repositories/collection_repository.go`

Ensure collection creation follows proper format:
```go
func (r *collectionRepository) CreateWithStandardFormat(ctx context.Context, collectionID, displayName, groupType string, isPromo bool) error {
    collection := &models.Collection{
        ID:         strings.ToLower(collectionID),  // Force lowercase
        Name:       displayName,                    // Keep original casing
        Origin:     "",                            // Empty string (not null)
        Aliases:    []string{strings.ToLower(collectionID)}, // ID in array
        Promo:      isPromo,
        Compressed: true,                          // Always true
        Fragments:  false,                         // Always false
        Tags:       []string{groupType},           // Single tag array
        CreatedAt:  time.Now(),
        UpdatedAt:  time.Now(),
    }
    
    return r.Create(ctx, collection)
}
```

### **3. Collection Import Service**
**File**: `web/services/collection_import.go`

```go
package services

import (
    "context"
    "fmt"
    "regexp"
    "strings"
    "time"
    
    "github.com/disgoorg/bot-template/bottemplate/database/models"
    "github.com/disgoorg/bot-template/bottemplate/database/repositories"
    "github.com/disgoorg/bot-template/bottemplate/services"
    "github.com/disgoorg/bot-template/bottemplate/economy/utils"
    webmodels "github.com/disgoorg/bot-template/web/models"
)

type CollectionImportService struct {
    cardRepo       repositories.CardRepository
    collectionRepo repositories.CollectionRepository
    spacesService  *services.SpacesService
    txManager      *utils.EconomicTransactionManager
}

type CollectionImportRequest struct {
    CollectionID   string                 `json:"collection_id"`
    DisplayName    string                 `json:"display_name"`
    GroupType      string                 `json:"group_type"` // "girlgroups" or "boygroups"
    IsPromo        bool                   `json:"is_promo"`
    Files          []*webmodels.FileUpload `json:"files"`
}

type ImportResult struct {
    CollectionID  string   `json:"collection_id"`
    CardsCreated  int      `json:"cards_created"`
    FirstCardID   int64    `json:"first_card_id"`
    LastCardID    int64    `json:"last_card_id"`
    FilesUploaded []string `json:"files_uploaded"`
    Success       bool     `json:"success"`
    ErrorMessage  string   `json:"error_message,omitempty"`
}

type ParsedFilename struct {
    Level      int    `json:"level"`
    Name       string `json:"name"`
    Extension  string `json:"extension"`
    IsAnimated bool   `json:"is_animated"`
    Original   string `json:"original"`
    Normalized string `json:"normalized"`
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

func (cis *CollectionImportService) ValidateAndNormalizeFilename(filename string) (*ParsedFilename, error) {
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
    
    return &ParsedFilename{
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
        return fmt.Sprintf("%s/%s/%s", basePath, groupType, collectionID)
    }
    return fmt.Sprintf("%s/%s/%s", basePath, groupType, collectionID)
}

func (cis *CollectionImportService) ProcessCollectionImport(ctx context.Context, req *CollectionImportRequest) (*ImportResult, error) {
    // 1. Validate all files first
    validatedFiles := make([]*ParsedFilename, 0, len(req.Files))
    for _, file := range req.Files {
        parsed, err := cis.ValidateAndNormalizeFilename(file.Name)
        if err != nil {
            return &ImportResult{
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
            return &ImportResult{
                Success:      false,
                ErrorMessage: fmt.Sprintf("Duplicate card name found: %s", parsed.Name),
            }, nil
        }
    }
    
    // 3. Get next card ID
    lastID, err := cis.cardRepo.GetLastCardID(ctx)
    if err != nil {
        return &ImportResult{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to get last card ID: %s", err.Error()),
        }, nil
    }
    nextID := lastID + 1
    
    // 4. Ensure collection exists with proper format
    err = cis.ensureCollectionExists(ctx, req.CollectionID, req.DisplayName, req.GroupType, req.IsPromo)
    if err != nil {
        return &ImportResult{
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
            return &ImportResult{
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
        return &ImportResult{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Database operation failed: %s", err.Error()),
        }, nil
    }
    
    return &ImportResult{
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
    collection := &models.Collection{
        ID:         strings.ToLower(collectionID),
        Name:       displayName,
        Origin:     "",                            // Empty string (not null)
        Aliases:    []string{strings.ToLower(collectionID)},
        Promo:      isPromo,
        Compressed: true,                          // Always true
        Fragments:  false,                         // Always false
        Tags:       []string{groupType},
        CreatedAt:  time.Now(),
        UpdatedAt:  time.Now(),
    }
    
    return cis.collectionRepo.Create(ctx, collection)
}

func (cis *CollectionImportService) uploadFileToSpaces(ctx context.Context, file *webmodels.FileUpload, spacesPath string) error {
    // Implementation depends on your SpacesService structure
    // This is a placeholder for the actual upload logic
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
```

### **4. Web Handler Updates**
**File**: `web/handlers/handlers.go`

Replace album handlers with collection handlers:
```go
func CollectionsImport(webApp *WebApp) fiber.Handler {
    return func(c *fiber.Ctx) error {
        ctx := c.Context()
        
        var req webmodels.CollectionImportRequest
        if err := c.BodyParser(&req); err != nil {
            return utils.SendError(c, 400, "INVALID_REQUEST", "Invalid request body", map[string]string{
                "error": err.Error(),
            })
        }
        
        // Validate required fields
        if req.CollectionID == "" || req.DisplayName == "" || req.GroupType == "" {
            return utils.SendError(c, 400, "MISSING_FIELDS", "Missing required fields", nil)
        }
        
        // Validate group type
        if req.GroupType != "girlgroups" && req.GroupType != "boygroups" {
            return utils.SendError(c, 400, "INVALID_GROUP_TYPE", "Group type must be 'girlgroups' or 'boygroups'", nil)
        }
        
        result, err := webApp.CollectionImportService.ProcessCollectionImport(ctx, &req)
        if err != nil {
            return utils.SendError(c, 500, "IMPORT_FAILED", err.Error(), nil)
        }
        
        if !result.Success {
            return utils.SendError(c, 400, "IMPORT_FAILED", result.ErrorMessage, nil)
        }
        
        return utils.SendSuccess(c, result, "Collection imported successfully")
    }
}

func CollectionsList(webApp *WebApp) fiber.Handler {
    return func(c *fiber.Ctx) error {
        ctx := c.Context()
        
        collections, err := webApp.Repos.Collection.GetAll(ctx)
        if err != nil {
            return utils.SendError(c, 500, "FETCH_FAILED", "Failed to fetch collections", nil)
        }
        
        return c.Render("pages/collections", fiber.Map{
            "Title":       "Collections - GoHYE Admin Panel",
            "Collections": collections,
        })
    }
}
```

### **5. Template Updates**
**File**: `web/templates/pages/collections.html`

Replace album interface with collection-focused workflow:
```html
<div class="page-container">
    <div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start;">
        <div>
            <h1>Collection Management</h1>
            <p class="text-secondary">Import and manage K-pop card collections</p>
        </div>
        <button class="btn btn-primary" onclick="openImportModal()">
            <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24" style="margin-right: var(--space-xs);">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"></path>
            </svg>
            Import Collection
        </button>
    </div>

    <!-- Import Modal -->
    <div id="import-modal" class="modal" style="display: none;">
        <div class="modal-content">
            <h2>Import New Collection</h2>
            
            <form id="collection-import-form" enctype="multipart/form-data">
                <!-- Collection Details -->
                <div class="form-grid">
                    <div>
                        <label class="form-label">Collection ID</label>
                        <input type="text" name="collection_id" class="form-input" placeholder="e.g., twice" required>
                        <small class="text-secondary">Lowercase, no spaces. Used for storage paths.</small>
                    </div>
                    <div>
                        <label class="form-label">Display Name</label>
                        <input type="text" name="display_name" class="form-input" placeholder="e.g., TWICE" required>
                    </div>
                </div>
                
                <div class="form-grid">
                    <div>
                        <label class="form-label">Group Type</label>
                        <select name="group_type" class="form-select" required>
                            <option value="">Select group type</option>
                            <option value="girlgroups">Girl Groups & Female Soloists</option>
                            <option value="boygroups">Boy Groups & Male Soloists</option>
                        </select>
                    </div>
                    <div>
                        <label class="form-label">Collection Type</label>
                        <label class="checkbox-label">
                            <input type="checkbox" name="is_promo">
                            <span>Promo Collection</span>
                        </label>
                    </div>
                </div>
                
                <!-- File Upload -->
                <div class="form-group">
                    <label class="form-label">Card Files</label>
                    <div class="upload-zone" ondrop="handleDrop(event)" ondragover="handleDragOver(event)">
                        <input type="file" id="file-input" multiple accept=".jpg,.png,.jpeg,.gif" onchange="handleFileSelect(event)">
                        <div class="upload-content">
                            <svg width="48" height="48" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"></path>
                            </svg>
                            <p>Drop files here or click to select</p>
                            <small>Required format: {level}_{name}.{ext}</small>
                        </div>
                    </div>
                </div>
                
                <!-- Validation Results -->
                <div id="validation-results" style="display: none;">
                    <h3>File Validation</h3>
                    <div id="validation-list"></div>
                </div>
                
                <div class="modal-actions">
                    <button type="button" class="btn btn-secondary" onclick="closeImportModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Import Collection</button>
                </div>
            </form>
        </div>
    </div>
</div>

<script>
// Collection import functionality
let selectedFiles = [];

function openImportModal() {
    document.getElementById('import-modal').style.display = 'flex';
}

function closeImportModal() {
    document.getElementById('import-modal').style.display = 'none';
    document.getElementById('collection-import-form').reset();
    selectedFiles = [];
    document.getElementById('validation-results').style.display = 'none';
}

function handleFileSelect(event) {
    selectedFiles = Array.from(event.target.files);
    validateFiles();
}

function handleDrop(event) {
    event.preventDefault();
    selectedFiles = Array.from(event.dataTransfer.files);
    validateFiles();
}

function handleDragOver(event) {
    event.preventDefault();
}

function validateFiles() {
    if (selectedFiles.length === 0) {
        document.getElementById('validation-results').style.display = 'none';
        return;
    }
    
    const pattern = /^(\d+)_(.+)\.(jpg|png|jpeg|gif)$/i;
    const results = [];
    const nameCount = {};
    
    selectedFiles.forEach(file => {
        const match = file.name.match(pattern);
        if (match) {
            const level = parseInt(match[1]);
            const name = match[2];
            const extension = match[3].toLowerCase();
            const isAnimated = extension === 'gif';
            
            // Check for duplicates
            if (nameCount[name]) {
                nameCount[name]++;
            } else {
                nameCount[name] = 1;
            }
            
            const normalized = `${level}_${name.toLowerCase().replace(/\s+/g, '_')}.${extension}`;
            
            results.push({
                filename: file.name,
                level: level,
                name: name,
                extension: extension,
                isAnimated: isAnimated,
                normalized: normalized,
                status: level >= 1 && level <= 5 ? 'valid' : 'invalid',
                error: level < 1 || level > 5 ? 'Level must be 1-5' : null,
                isDuplicate: false
            });
        } else {
            results.push({
                filename: file.name,
                status: 'invalid',
                error: 'Invalid format. Use: {level}_{name}.{ext}'
            });
        }
    });
    
    // Mark duplicates
    Object.keys(nameCount).forEach(name => {
        if (nameCount[name] > 1) {
            results.forEach(result => {
                if (result.name === name) {
                    result.isDuplicate = true;
                    result.status = 'invalid';
                    result.error = 'Duplicate name in collection';
                }
            });
        }
    });
    
    displayValidationResults(results);
}

function displayValidationResults(results) {
    const validationResults = document.getElementById('validation-results');
    const validationList = document.getElementById('validation-list');
    
    let html = '<table class="validation-table">';
    html += '<thead><tr><th>File Name</th><th>Status</th><th>Level</th><th>Normalized</th><th>Notes</th></tr></thead>';
    html += '<tbody>';
    
    results.forEach(result => {
        const statusClass = result.status === 'valid' ? 'text-success' : 'text-danger';
        const statusIcon = result.status === 'valid' ? '✅' : '❌';
        
        html += `<tr>`;
        html += `<td>${result.filename}</td>`;
        html += `<td class="${statusClass}">${statusIcon} ${result.status}</td>`;
        html += `<td>${result.level || '-'}</td>`;
        html += `<td>${result.normalized || '-'}</td>`;
        html += `<td>${result.error || (result.isAnimated ? 'Animated' : '')}</td>`;
        html += `</tr>`;
    });
    
    html += '</tbody></table>';
    
    validationList.innerHTML = html;
    validationResults.style.display = 'block';
}

// Form submission
document.getElementById('collection-import-form').addEventListener('submit', async function(e) {
    e.preventDefault();
    
    const formData = new FormData();
    formData.append('collection_id', document.querySelector('[name="collection_id"]').value);
    formData.append('display_name', document.querySelector('[name="display_name"]').value);
    formData.append('group_type', document.querySelector('[name="group_type"]').value);
    formData.append('is_promo', document.querySelector('[name="is_promo"]').checked);
    
    selectedFiles.forEach(file => {
        formData.append('files', file);
    });
    
    try {
        const response = await fetch('/admin/collections/import', {
            method: 'POST',
            body: formData
        });
        
        const result = await response.json();
        
        if (result.success) {
            alert(`Collection imported successfully! Created ${result.data.cards_created} cards (ID ${result.data.first_card_id}-${result.data.last_card_id})`);
            closeImportModal();
            location.reload();
        } else {
            alert(`Import failed: ${result.message}`);
        }
    } catch (error) {
        alert(`Import failed: ${error.message}`);
    }
});
</script>
```

---

## 🚨 **Critical Implementation Points**

### **1. Collection Format Compliance**
- **ID**: Always lowercase, no spaces
- **Name**: Proper display name with original casing
- **Origin**: Empty string `""`, not null
- **Aliases**: Array with ID as first element
- **Compressed**: Always `true`
- **Fragments**: Always `false`
- **Tags**: Single element array with group type

### **2. ID Management**
- Use `SELECT COALESCE(MAX(id), 0) FROM cards` to get last ID
- Sequential assignment for batch imports
- Atomic transactions with complete rollback on failure

### **3. Filename Validation**
- Pattern: `^(\d+)_(.+)\.(jpg|png|jpeg|gif)$`
- Support single words: `1_hello.jpg`
- Support multiple words: `2_cheer_up_twice.jpg`
- Auto-normalize: lowercase + underscores

### **4. Storage Path Generation**
- Regular: `cards/{group_type}/{collection_id}/`
- Promo: `promo/{group_type}/{collection_id}/`
- Examples: `cards/girlgroups/twice/`, `promo/boygroups/bts/`

### **5. Error Recovery**
- Upload files first (easier to cleanup than database)
- Complete transaction rollback on any failure
- Cleanup uploaded files if database operations fail
- Detailed error messages for UI feedback

---

## 🎯 **Success Criteria**

1. ✅ **Format Compliance**: All collections follow exact JSON structure
2. ✅ **ID Integrity**: Sequential card IDs without conflicts
3. ✅ **Atomic Operations**: Complete rollback on any failure
4. ✅ **Flexible Naming**: Support all card name patterns

5. ✅ **Promo Support**: Correct storage paths for promo collections
6. ✅ **UI Validation**: Real-time feedback and error handling
7. ✅ **Performance**: Process 200+ files efficiently
8. ✅ **Error Recovery**: Complete cleanup on failures

This implementation plan ensures robust collection management while maintaining exact compliance with existing collection format standards.