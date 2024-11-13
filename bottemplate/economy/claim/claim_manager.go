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
	claimOwners     sync.Map
	messageOwners   sync.Map // Add this to track which message belongs to which user
	maxClaims       int32    // Changed to int32 for atomic operations
	cooldownPeriod  time.Duration
	lockDuration    time.Duration // Added as configurable parameter
	sessionTimeout  time.Duration
}

func NewManager(cooldownPeriod time.Duration) *Manager {
	return &Manager{
		maxClaims:      3,
		cooldownPeriod: cooldownPeriod,
		lockDuration:   30 * time.Second,
		sessionTimeout: 30 * time.Second,
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
	m.claimOwners.Delete(userID)
	// Clean up message ownership entries for this user
	m.messageOwners.Range(func(key, value interface{}) bool {
		if value.(string) == userID {
			m.messageOwners.Delete(key)
		}
		return true
	})
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

	// Cleanup active claim locks
	m.activeClaimLock.Range(func(key, value interface{}) bool {
		if now.After(value.(time.Time)) {
			m.activeClaimLock.Delete(key)
			m.activeUsers.Delete(key)
			m.claimOwners.Delete(key)
		}
		return true
	})

	// Cleanup active sessions
	m.activeUsers.Range(func(key, value interface{}) bool {
		sessionStart := value.(time.Time)
		if now.Sub(sessionStart) > m.sessionTimeout {
			m.activeUsers.Delete(key)
			m.activeClaimLock.Delete(key)
			m.claimOwners.Delete(key)
		}
		return true
	})

	// Also cleanup message owners for expired sessions
	m.messageOwners.Range(func(key, value interface{}) bool {
		userID := value.(string)
		if _, exists := m.activeUsers.Load(userID); !exists {
			m.messageOwners.Delete(key)
		}
		return true
	})
}

func (m *Manager) StartCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
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

func (m *Manager) IsClaimOwner(userID string) bool {
	if owner, exists := m.claimOwners.Load(userID); exists {
		return owner.(string) == userID
	}
	return false
}

func (m *Manager) RegisterMessageOwner(messageID string, userID string) {
	m.messageOwners.Store(messageID, userID)
}

func (m *Manager) IsMessageOwner(messageID string, userID string) bool {
	if owner, exists := m.messageOwners.Load(messageID); exists {
		return owner.(string) == userID
	}
	return false
}
