package claim

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
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
	claimCards      sync.Map // stores messageID -> []models.Card
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

	// Try to create a new session atomically
	now := time.Now()
	if _, loaded := m.activeUsers.LoadOrStore(userID, now); loaded {
		return false
	}

	// Store claim lock with expiry time - this must succeed since we just created the session
	expiry := now.Add(m.lockDuration)
	m.activeClaimLock.Store(userID, expiry)
	
	// Initialize claim owner entry to maintain consistency
	m.claimOwners.Store(userID, userID)
	
	return true
}

func (m *Manager) ReleaseClaim(userID string) {
	// Atomically clean up all related state to prevent inconsistencies
	m.activeClaimLock.Delete(userID)
	m.activeUsers.Delete(userID)
	m.claimOwners.Delete(userID)
	
	// Clean up message owners - need to iterate to find messages owned by this user
	var messagesToDelete []string
	m.messageOwners.Range(func(key, value interface{}) bool {
		messageID := key.(string)
		ownerID := value.(string)
		if ownerID == userID {
			messagesToDelete = append(messagesToDelete, messageID)
		}
		return true
	})
	
	for _, messageID := range messagesToDelete {
		m.messageOwners.Delete(messageID)
		m.claimCards.Delete(messageID)
	}

	// Set cooldown
	m.claimCooldowns.Store(userID, time.Now().Add(m.cooldownPeriod))
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
	var expiredUsers []string

	// First, identify all expired users from active claim locks
	m.activeClaimLock.Range(func(key, value interface{}) bool {
		userID := key.(string)
		expiry := value.(time.Time)
		if now.After(expiry) {
			expiredUsers = append(expiredUsers, userID)
		}
		return true
	})

	// Check for session timeouts
	m.activeUsers.Range(func(key, value interface{}) bool {
		userID := key.(string)
		sessionStart := value.(time.Time)
		if now.Sub(sessionStart) > m.sessionTimeout {
			expiredUsers = append(expiredUsers, userID)
		}
		return true
	})

	// Remove duplicates and clean up all expired users atomically
	expiredMap := make(map[string]bool)
	for _, userID := range expiredUsers {
		expiredMap[userID] = true
	}

	for userID := range expiredMap {
		// Use ReleaseClaim to ensure consistent cleanup
		m.ReleaseClaim(userID)
	}
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
				func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("Panic in claim cleanup routine", slog.Any("panic", r))
						}
					}()
					m.cleanupExpiredLocks()
				}()
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

func (m *Manager) StoreClaimCards(messageID string, cards []*models.Card) {
	m.claimCards.Store(messageID, cards)
}

func (m *Manager) GetClaimCards(messageID string) []*models.Card {
	if cards, ok := m.claimCards.Load(messageID); ok {
		return cards.([]*models.Card)
	}
	return nil
}
