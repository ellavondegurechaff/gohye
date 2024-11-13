package claim

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Manager struct {
	activeClaimLock sync.Map // Changed to sync.Map for better concurrent access
	claimCooldowns  sync.Map
	activeUsers     sync.Map // Add this to track users with active claim sessions
	maxClaims       int32    // Changed to int32 for atomic operations
	cooldownPeriod  time.Duration
	lockDuration    time.Duration // Added as configurable parameter
}

func NewManager(cooldownPeriod time.Duration) *Manager {
	return &Manager{
		maxClaims:      3,
		cooldownPeriod: cooldownPeriod,
		lockDuration:   30 * time.Second,
	}
}

func (m *Manager) CanClaim(userID string) (bool, time.Duration) {
	if cooldown, exists := m.claimCooldowns.Load(userID); exists {
		nextClaim := cooldown.(time.Time)
		if time.Now().Before(nextClaim) {
			return false, time.Until(nextClaim)
		}
	}
	return true, 0
}

func (m *Manager) HasActiveSession(userID string) bool {
	_, exists := m.activeUsers.Load(userID)
	return exists
}

func (m *Manager) LockClaim(userID string) bool {
	// First check if user already has an active session
	if m.HasActiveSession(userID) {
		return false
	}

	// Try to create a new session
	if _, loaded := m.activeUsers.LoadOrStore(userID, time.Now()); loaded {
		return false
	}

	now := time.Now()
	expiry := now.Add(m.lockDuration)
	m.activeClaimLock.Store(userID, expiry)
	return true
}

func (m *Manager) ReleaseClaim(userID string) {
	m.activeClaimLock.Delete(userID)
	m.activeUsers.Delete(userID)
}

func (m *Manager) SetClaimCooldown(userID string) {
	m.claimCooldowns.Store(userID, time.Now().Add(m.cooldownPeriod))
}

func (m *Manager) GetClaimStats(userID string) (used int, remaining int, nextReset time.Time) {
	if cooldown, exists := m.claimCooldowns.Load(userID); exists {
		nextReset = cooldown.(time.Time)
	}

	used = m.getUsedClaims(userID)
	remaining = int(atomic.LoadInt32(&m.maxClaims)) - used
	if remaining < 0 {
		remaining = 0
	}

	return
}

func (m *Manager) getUsedClaims(userID string) int {
	count := 0
	now := time.Now()
	m.claimCooldowns.Range(func(key, value interface{}) bool {
		if key.(string) == userID {
			cooldown := value.(time.Time)
			if now.Before(cooldown) {
				count++
			}
		}
		return true
	})
	return count
}

func (m *Manager) cleanupExpiredLocks() {
	now := time.Now()
	m.activeClaimLock.Range(func(key, value interface{}) bool {
		if now.After(value.(time.Time)) {
			m.activeClaimLock.Delete(key)
		}
		return true
	})
}

func (m *Manager) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.cleanupExpiredLocks()
			}
		}
	}()
}
