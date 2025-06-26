package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/uptrace/bun"
)

// BaseRepository provides common repository functionality
type BaseRepository struct {
	db             *bun.DB
	defaultTimeout time.Duration
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(db *bun.DB) *BaseRepository {
	return &BaseRepository{
		db:             db,
		defaultTimeout: config.DefaultQueryTimeout,
	}
}

// RepositoryError represents a repository-level error
type RepositoryError struct {
	Operation string
	Entity    string
	Err       error
}

func (re *RepositoryError) Error() string {
	return fmt.Sprintf("repository error during %s for %s: %v", re.Operation, re.Entity, re.Err)
}

func (re *RepositoryError) Unwrap() error {
	return re.Err
}

// NotFoundError represents an entity not found error
type NotFoundError struct {
	Entity string
	ID     interface{}
}

func (nfe *NotFoundError) Error() string {
	return fmt.Sprintf("%s with ID %v not found", nfe.Entity, nfe.ID)
}

// ConflictError represents a data conflict error
type ConflictError struct {
	Entity string
	Field  string
	Value  interface{}
}

func (ce *ConflictError) Error() string {
	return fmt.Sprintf("%s with %s %v already exists", ce.Entity, ce.Field, ce.Value)
}

// WithTimeout creates a context with the default timeout
func (br *BaseRepository) WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, br.defaultTimeout)
}

// WithCustomTimeout creates a context with a custom timeout
func (br *BaseRepository) WithCustomTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// HandleError standardizes error handling across repositories
func (br *BaseRepository) HandleError(operation, entity string, err error) error {
	if err == nil {
		return nil
	}

	if err == sql.ErrNoRows {
		return &NotFoundError{Entity: entity, ID: "unknown"}
	}

	return &RepositoryError{
		Operation: operation,
		Entity:    entity,
		Err:       err,
	}
}

// HandleErrorWithID standardizes error handling with specific ID
func (br *BaseRepository) HandleErrorWithID(operation, entity string, id interface{}, err error) error {
	if err == nil {
		return nil
	}

	if err == sql.ErrNoRows {
		return &NotFoundError{Entity: entity, ID: id}
	}

	return &RepositoryError{
		Operation: operation,
		Entity:    entity,
		Err:       err,
	}
}

// ExecWithTimeout executes a query with timeout and error handling
func (br *BaseRepository) ExecWithTimeout(ctx context.Context, operation, entity string, query func(context.Context) (sql.Result, error)) (sql.Result, error) {
	timeoutCtx, cancel := br.WithTimeout(ctx)
	defer cancel()

	result, err := query(timeoutCtx)
	return result, br.HandleError(operation, entity, err)
}

// SelectWithTimeout executes a select query with timeout and error handling
func (br *BaseRepository) SelectWithTimeout(ctx context.Context, operation, entity string, query func(context.Context) error) error {
	timeoutCtx, cancel := br.WithTimeout(ctx)
	defer cancel()

	err := query(timeoutCtx)
	return br.HandleError(operation, entity, err)
}

// SelectOneWithTimeout executes a select one query with timeout and error handling
func (br *BaseRepository) SelectOneWithTimeout(ctx context.Context, operation, entity string, id interface{}, query func(context.Context) error) error {
	timeoutCtx, cancel := br.WithTimeout(ctx)
	defer cancel()

	err := query(timeoutCtx)
	return br.HandleErrorWithID(operation, entity, id, err)
}

// Transaction executes a function within a database transaction
func (br *BaseRepository) Transaction(ctx context.Context, fn func(context.Context, bun.Tx) error) error {
	timeoutCtx, cancel := br.WithTimeout(ctx)
	defer cancel()

	return br.db.RunInTx(timeoutCtx, nil, fn)
}

// BatchInsert performs batch insert with optimal batch sizing
func (br *BaseRepository) BatchInsert(ctx context.Context, entity string, items interface{}) error {
	timeoutCtx, cancel := br.WithCustomTimeout(ctx, config.BatchQueryTimeout)
	defer cancel()

	_, err := br.db.NewInsert().Model(items).Exec(timeoutCtx)
	return br.HandleError("batch_insert", entity, err)
}

// Count returns the count of records matching the query
func (br *BaseRepository) Count(ctx context.Context, entity string, query *bun.SelectQuery) (int, error) {
	timeoutCtx, cancel := br.WithTimeout(ctx)
	defer cancel()

	count, err := query.Count(timeoutCtx)
	return count, br.HandleError("count", entity, err)
}

// Exists checks if a record exists
func (br *BaseRepository) Exists(ctx context.Context, entity string, query *bun.SelectQuery) (bool, error) {
	timeoutCtx, cancel := br.WithTimeout(ctx)
	defer cancel()

	exists, err := query.Exists(timeoutCtx)
	return exists, br.HandleError("exists", entity, err)
}

// ValidateRequired checks if required fields are present
func (br *BaseRepository) ValidateRequired(fields map[string]interface{}) error {
	for field, value := range fields {
		if value == nil {
			return fmt.Errorf("required field %s cannot be nil", field)
		}
		
		// Check for empty strings
		if str, ok := value.(string); ok && str == "" {
			return fmt.Errorf("required field %s cannot be empty", field)
		}
	}
	return nil
}

// SetTimestamps updates CreatedAt and UpdatedAt fields
func (br *BaseRepository) SetTimestamps(model interface{}) {
	now := time.Now()
	
	// Use reflection or interface to set timestamps
	// This is a simplified version - in practice you might want to use interfaces
	type Timestamped interface {
		SetTimestamps(created, updated time.Time)
	}
	
	if ts, ok := model.(Timestamped); ok {
		ts.SetTimestamps(now, now)
	}
}

// SetUpdateTimestamp updates only the UpdatedAt field
func (br *BaseRepository) SetUpdateTimestamp(model interface{}) {
	now := time.Now()
	
	type UpdateTimestamped interface {
		SetUpdateTimestamp(updated time.Time)
	}
	
	if ts, ok := model.(UpdateTimestamped); ok {
		ts.SetUpdateTimestamp(now)
	}
}

// GetDB returns the underlying database connection
func (br *BaseRepository) GetDB() *bun.DB {
	return br.db
}

// IsNotFound checks if an error is a NotFoundError
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// IsConflict checks if an error is a ConflictError
func IsConflict(err error) bool {
	_, ok := err.(*ConflictError)
	return ok
}

// IsRepositoryError checks if an error is a RepositoryError
func IsRepositoryError(err error) bool {
	_, ok := err.(*RepositoryError)
	return ok
}