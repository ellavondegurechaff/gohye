package effects

import (
	"context"
	"fmt"
	"sync"
	"time"

	"log/slog"
)

// EffectRegistry manages all registered effects
type EffectRegistry struct {
	mu                sync.RWMutex
	handlers          map[string]EffectHandler
	activeHandlers    map[string]ActiveEffectHandler
	passiveHandlers   map[string]PassiveEffectHandler
	categories        map[EffectCategory][]string
	executionStats    map[string]*EffectExecutionStats
	deps              *EffectDependencies
}

// EffectExecutionStats tracks statistics for effect execution
type EffectExecutionStats struct {
	TotalExecutions   int64         `json:"total_executions"`
	SuccessfulRuns    int64         `json:"successful_runs"`
	FailedRuns        int64         `json:"failed_runs"`
	AverageTime       time.Duration `json:"average_time"`
	LastExecuted      time.Time     `json:"last_executed"`
	TotalExecutionTime time.Duration `json:"total_execution_time"`
}

// NewEffectRegistry creates a new effect registry
func NewEffectRegistry(deps *EffectDependencies) *EffectRegistry {
	return &EffectRegistry{
		handlers:        make(map[string]EffectHandler),
		activeHandlers:  make(map[string]ActiveEffectHandler),
		passiveHandlers: make(map[string]PassiveEffectHandler),
		categories:      make(map[EffectCategory][]string),
		executionStats:  make(map[string]*EffectExecutionStats),
		deps:            deps,
	}
}

// RegisterEffect registers an effect handler
func (r *EffectRegistry) RegisterEffect(handler EffectHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	metadata := handler.GetMetadata()
	effectID := metadata.ID

	if effectID == "" {
		return fmt.Errorf("effect ID cannot be empty")
	}

	if _, exists := r.handlers[effectID]; exists {
		return fmt.Errorf("effect %s is already registered", effectID)
	}

	// Register in main handlers map
	r.handlers[effectID] = handler

	// Register in specific type maps
	switch handler.(type) {
	case ActiveEffectHandler:
		r.activeHandlers[effectID] = handler.(ActiveEffectHandler)
	case PassiveEffectHandler:
		r.passiveHandlers[effectID] = handler.(PassiveEffectHandler)
	}

	// Register in category map
	if r.categories[metadata.Category] == nil {
		r.categories[metadata.Category] = make([]string, 0)
	}
	r.categories[metadata.Category] = append(r.categories[metadata.Category], effectID)

	// Initialize execution stats
	r.executionStats[effectID] = &EffectExecutionStats{}

	slog.Info("Effect registered successfully",
		slog.String("effect_id", effectID),
		slog.String("type", string(metadata.Type)),
		slog.String("category", string(metadata.Category)))

	return nil
}

// GetEffect retrieves an effect handler by ID
func (r *EffectRegistry) GetEffect(effectID string) (EffectHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[effectID]
	if !exists {
		return nil, fmt.Errorf("effect %s not found", effectID)
	}

	return handler, nil
}

// GetActiveEffect retrieves an active effect handler by ID
func (r *EffectRegistry) GetActiveEffect(effectID string) (ActiveEffectHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.activeHandlers[effectID]
	if !exists {
		return nil, fmt.Errorf("active effect %s not found", effectID)
	}

	return handler, nil
}

// GetPassiveEffect retrieves a passive effect handler by ID
func (r *EffectRegistry) GetPassiveEffect(effectID string) (PassiveEffectHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.passiveHandlers[effectID]
	if !exists {
		return nil, fmt.Errorf("passive effect %s not found", effectID)
	}

	return handler, nil
}

// ListEffects returns all registered effect IDs
func (r *EffectRegistry) ListEffects() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	effects := make([]string, 0, len(r.handlers))
	for effectID := range r.handlers {
		effects = append(effects, effectID)
	}

	return effects
}

// ListEffectsByCategory returns effect IDs in a specific category
func (r *EffectRegistry) ListEffectsByCategory(category EffectCategory) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	effects, exists := r.categories[category]
	if !exists {
		return []string{}
	}

	// Return a copy to prevent external modification
	result := make([]string, len(effects))
	copy(result, effects)
	return result
}

// ListActiveEffects returns all active effect IDs
func (r *EffectRegistry) ListActiveEffects() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	effects := make([]string, 0, len(r.activeHandlers))
	for effectID := range r.activeHandlers {
		effects = append(effects, effectID)
	}

	return effects
}

// ListPassiveEffects returns all passive effect IDs
func (r *EffectRegistry) ListPassiveEffects() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	effects := make([]string, 0, len(r.passiveHandlers))
	for effectID := range r.passiveHandlers {
		effects = append(effects, effectID)
	}

	return effects
}

// ExecuteEffect executes an effect and tracks statistics
func (r *EffectRegistry) ExecuteEffect(ctx context.Context, effectID string, params EffectParams) (*EffectResult, error) {
	startTime := time.Now()

	// Get the effect handler
	handler, err := r.GetEffect(effectID)
	if err != nil {
		return nil, err
	}

	// Update execution stats
	r.updateExecutionStats(effectID, true, 0, false)

	// Execute the effect
	result, err := handler.Execute(ctx, params)
	
	executionTime := time.Since(startTime)
	success := err == nil && result != nil && result.Success

	// Update final execution stats
	r.updateExecutionStats(effectID, false, executionTime, success)

	if result != nil && result.Metrics == nil {
		result.Metrics = &EffectMetrics{
			ExecutionTime: executionTime,
		}
	}

	slog.Info("Effect executed",
		slog.String("effect_id", effectID),
		slog.String("user_id", params.UserID),
		slog.Duration("execution_time", executionTime),
		slog.Bool("success", success))

	return result, err
}

// updateExecutionStats updates the execution statistics for an effect
func (r *EffectRegistry) updateExecutionStats(effectID string, starting bool, executionTime time.Duration, success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats, exists := r.executionStats[effectID]
	if !exists {
		stats = &EffectExecutionStats{}
		r.executionStats[effectID] = stats
	}

	if starting {
		stats.TotalExecutions++
		stats.LastExecuted = time.Now()
	} else {
		if success {
			stats.SuccessfulRuns++
		} else {
			stats.FailedRuns++
		}

		stats.TotalExecutionTime += executionTime
		if stats.SuccessfulRuns > 0 {
			stats.AverageTime = stats.TotalExecutionTime / time.Duration(stats.SuccessfulRuns)
		}
	}
}

// GetExecutionStats returns execution statistics for an effect
func (r *EffectRegistry) GetExecutionStats(effectID string) (*EffectExecutionStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats, exists := r.executionStats[effectID]
	if !exists {
		return nil, fmt.Errorf("no statistics found for effect %s", effectID)
	}

	// Return a copy to prevent external modification
	statsCopy := *stats
	return &statsCopy, nil
}

// GetAllExecutionStats returns execution statistics for all effects
func (r *EffectRegistry) GetAllExecutionStats() map[string]*EffectExecutionStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*EffectExecutionStats)
	for effectID, stats := range r.executionStats {
		statsCopy := *stats
		result[effectID] = &statsCopy
	}

	return result
}

// ValidateEffect validates if an effect can be executed
func (r *EffectRegistry) ValidateEffect(ctx context.Context, effectID string, params EffectParams) []EffectValidationError {
	handler, err := r.GetEffect(effectID)
	if err != nil {
		return []EffectValidationError{
			{
				Field:   "effect_id",
				Message: err.Error(),
				Code:    "EFFECT_NOT_FOUND",
			},
		}
	}

	return handler.Validate(ctx, params)
}

// CanExecuteEffect checks if an effect can be executed
func (r *EffectRegistry) CanExecuteEffect(ctx context.Context, effectID string, userID string) (bool, string) {
	handler, err := r.GetEffect(effectID)
	if err != nil {
		return false, err.Error()
	}

	return handler.CanExecute(ctx, userID)
}

// UnregisterEffect removes an effect from the registry
func (r *EffectRegistry) UnregisterEffect(effectID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	handler, exists := r.handlers[effectID]
	if !exists {
		return fmt.Errorf("effect %s not found", effectID)
	}

	metadata := handler.GetMetadata()

	// Remove from main handlers map
	delete(r.handlers, effectID)

	// Remove from specific type maps
	delete(r.activeHandlers, effectID)
	delete(r.passiveHandlers, effectID)

	// Remove from category map
	if categoryEffects, exists := r.categories[metadata.Category]; exists {
		for i, id := range categoryEffects {
			if id == effectID {
				r.categories[metadata.Category] = append(categoryEffects[:i], categoryEffects[i+1:]...)
				break
			}
		}
	}

	// Keep execution stats for historical purposes
	// delete(r.executionStats, effectID)

	slog.Info("Effect unregistered",
		slog.String("effect_id", effectID))

	return nil
}

// Shutdown gracefully shuts down the registry
func (r *EffectRegistry) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	slog.Info("Shutting down effect registry",
		slog.Int("registered_effects", len(r.handlers)))

	// Clear all maps
	r.handlers = make(map[string]EffectHandler)
	r.activeHandlers = make(map[string]ActiveEffectHandler)
	r.passiveHandlers = make(map[string]PassiveEffectHandler)
	r.categories = make(map[EffectCategory][]string)

	return nil
}