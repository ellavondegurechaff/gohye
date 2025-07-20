package utils

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// BackgroundProcessManager manages all background goroutines with proper lifecycle control
type BackgroundProcessManager struct {
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	processes map[string]*ProcessInfo
	mu        sync.RWMutex
}

type ProcessInfo struct {
	name        string
	cancel      context.CancelFunc
	description string
}

// NewBackgroundProcessManager creates a new process manager
func NewBackgroundProcessManager() *BackgroundProcessManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &BackgroundProcessManager{
		ctx:       ctx,
		cancel:    cancel,
		processes: make(map[string]*ProcessInfo),
	}
}

// StartProcess registers and starts a background process
func (bpm *BackgroundProcessManager) StartProcess(name, description string, fn func(ctx context.Context)) {
	bpm.mu.Lock()
	defer bpm.mu.Unlock()

	if _, exists := bpm.processes[name]; exists {
		slog.Warn("Process already exists, stopping existing one", slog.String("name", name))
		bpm.stopProcessLocked(name)
	}

	processCtx, processCancel := context.WithCancel(bpm.ctx)
	bpm.processes[name] = &ProcessInfo{
		name:        name,
		cancel:      processCancel,
		description: description,
	}

	bpm.wg.Add(1)
	go func() {
		defer bpm.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Background process panic",
					slog.String("process", name),
					slog.Any("panic", r))
			}
		}()

		slog.Info("Starting background process",
			slog.String("process", name),
			slog.String("description", description))

		fn(processCtx)

		slog.Info("Background process ended",
			slog.String("process", name))
	}()
}

// StopProcess stops a specific background process
func (bpm *BackgroundProcessManager) StopProcess(name string) {
	bpm.mu.Lock()
	defer bpm.mu.Unlock()
	bpm.stopProcessLocked(name)
}

func (bpm *BackgroundProcessManager) stopProcessLocked(name string) {
	if process, exists := bpm.processes[name]; exists {
		process.cancel()
		delete(bpm.processes, name)
		slog.Info("Stopped background process", slog.String("process", name))
	}
}

// Shutdown gracefully stops all background processes
func (bpm *BackgroundProcessManager) Shutdown(timeout time.Duration) error {
	slog.Info("Shutting down background processes",
		slog.Int("process_count", len(bpm.processes)))

	// Cancel all processes
	bpm.cancel()

	// Wait for all processes to finish with timeout
	done := make(chan struct{})
	go func() {
		bpm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("All background processes stopped gracefully")
		return nil
	case <-time.After(timeout):
		slog.Warn("Timeout waiting for background processes to stop",
			slog.Duration("timeout", timeout))
		return context.DeadlineExceeded
	}
}

// GetProcessCount returns the number of active processes
func (bpm *BackgroundProcessManager) GetProcessCount() int {
	bpm.mu.RLock()
	defer bpm.mu.RUnlock()
	return len(bpm.processes)
}

// ListProcesses returns information about all active processes
func (bpm *BackgroundProcessManager) ListProcesses() []ProcessInfo {
	bpm.mu.RLock()
	defer bpm.mu.RUnlock()

	processes := make([]ProcessInfo, 0, len(bpm.processes))
	for _, process := range bpm.processes {
		processes = append(processes, *process)
	}
	return processes
}

// Context returns the global context for the manager
func (bpm *BackgroundProcessManager) Context() context.Context {
	return bpm.ctx
}
