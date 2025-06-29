package models

import (
	"time"
)

// APIResponse represents a standard API response structure
type APIResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// APIError represents an API error response
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	APIResponse
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// PaginationInfo contains pagination metadata
type PaginationInfo struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// UploadProgress represents file upload progress
type UploadProgress struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	BytesRead   int64     `json:"bytes_read"`
	TotalBytes  int64     `json:"total_bytes"`
	Percentage  float64   `json:"percentage"`
	Status      string    `json:"status"` // uploading, processing, completed, failed
	Message     string    `json:"message,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ImportProgress represents collection import progress
type ImportProgress struct {
	ID             string    `json:"id"`
	CollectionName string    `json:"collection_name"`
	TotalFiles    int       `json:"total_files"`
	ProcessedFiles int      `json:"processed_files"`
	SuccessCount  int       `json:"success_count"`
	ErrorCount    int       `json:"error_count"`
	Status        string    `json:"status"` // processing, completed, failed, cancelled
	CurrentFile   string    `json:"current_file,omitempty"`
	Errors        []ImportError `json:"errors,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// ImportError represents an error during import
type ImportError struct {
	Filename string `json:"filename"`
	Error    string `json:"error"`
	Code     string `json:"code"`
}

// FieldValidationError represents a field validation error
type FieldValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// NewSuccessResponse creates a successful API response
func NewSuccessResponse(data interface{}, message string) *APIResponse {
	return &APIResponse{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewErrorResponse creates an error API response
func NewErrorResponse(code, message string, details map[string]string) *APIResponse {
	return &APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now(),
	}
}

// NewPaginatedResponse creates a paginated API response
func NewPaginatedResponse(data interface{}, pagination *PaginationInfo, message string) *PaginatedResponse {
	return &PaginatedResponse{
		APIResponse: APIResponse{
			Success:   true,
			Message:   message,
			Data:      data,
			Timestamp: time.Now(),
		},
		Pagination: pagination,
	}
}

// NewPaginationInfo creates pagination info
func NewPaginationInfo(page, limit int, total int64) *PaginationInfo {
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	return &PaginationInfo{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// HealthCheck represents a health check response
type HealthCheck struct {
	Status     string                 `json:"status"`
	Timestamp  time.Time              `json:"timestamp"`
	Version    string                 `json:"version"`
	Components map[string]ComponentHealth `json:"components"`
}

// ComponentHealth represents health status of a component
type ComponentHealth struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// NewHealthCheck creates a new health check response
func NewHealthCheck(version string) *HealthCheck {
	return &HealthCheck{
		Status:     "healthy",
		Timestamp:  time.Now(),
		Version:    version,
		Components: make(map[string]ComponentHealth),
	}
}

// AddComponent adds a component health status
func (h *HealthCheck) AddComponent(name, status, message string, details map[string]interface{}) {
	h.Components[name] = ComponentHealth{
		Status:  status,
		Message: message,
		Details: details,
	}
	
	// If any component is unhealthy, mark overall status as unhealthy
	if status != "healthy" && h.Status == "healthy" {
		h.Status = "unhealthy"
	}
}