package stores

// SessionStore provides a simple key/value storage for arbitrary JSON‑like
// session state scoped by sessionID.  It is intentionally minimal: callers
// decide the structure of the stored data.  The built‑in implementation is an
// in‑memory map safe for concurrent use which is perfectly sufficient for dev
// & unit tests.  Production deployments can swap in a persistent
// implementation (redis, sql, …).

import (
	"sync"
	"time"
)

type SessionStore interface {
	Get(sessionID string) (map[string]any, bool)
	Set(sessionID string, data map[string]any)
	Delete(sessionID string)
	Cleanup() // Add cleanup method to interface
	Close()   // Add close method to interface
}

// sessionData wraps the actual data with expiration time
type sessionData struct {
	Data      map[string]any
	ExpiresAt time.Time
}

// InMemorySessionStore is the default implementation.
type InMemorySessionStore struct {
	mu         sync.RWMutex
	data       map[string]*sessionData
	expiration time.Duration
	stopChan   chan struct{}
}

func NewInMemorySessionStore() *InMemorySessionStore {
	store := &InMemorySessionStore{
		data:       make(map[string]*sessionData),
		expiration: 24 * time.Hour, // Default 24 hour expiration
		stopChan:   make(chan struct{}),
	}
	
	// Start cleanup goroutine
	go store.cleanupExpired()
	
	return store
}

func (s *InMemorySessionStore) Get(id string) (map[string]any, bool) {
	s.mu.RLock()
	sessionData, ok := s.data[id]
	s.mu.RUnlock()
	
	if !ok {
		return nil, false
	}
	
	// Check if session has expired
	if time.Now().After(sessionData.ExpiresAt) {
		s.mu.Lock()
		// Re-check after upgrading to write lock to avoid race condition
		currentSession, stillExists := s.data[id]
		if stillExists && currentSession == sessionData {
			// Only delete if it's still the same expired session
			delete(s.data, id)
		}
		s.mu.Unlock()
		return nil, false
	}
	
	return sessionData.Data, true
}

func (s *InMemorySessionStore) Set(id string, d map[string]any) {
	s.mu.Lock()
	s.data[id] = &sessionData{
		Data:      d,
		ExpiresAt: time.Now().Add(s.expiration),
	}
	s.mu.Unlock()
}

func (s *InMemorySessionStore) Delete(id string) {
	s.mu.Lock()
	delete(s.data, id)
	s.mu.Unlock()
}

func (s *InMemorySessionStore) Cleanup() {
	// First pass: collect expired session IDs with read lock
	s.mu.RLock()
	now := time.Now()
	expiredIDs := make([]string, 0)
	for id, sessionData := range s.data {
		if now.After(sessionData.ExpiresAt) {
			expiredIDs = append(expiredIDs, id)
		}
	}
	s.mu.RUnlock()
	
	// Second pass: delete expired sessions with write lock
	if len(expiredIDs) > 0 {
		s.mu.Lock()
		for _, id := range expiredIDs {
			delete(s.data, id)
		}
		s.mu.Unlock()
	}
}

// Close stops the cleanup goroutine and releases resources
func (s *InMemorySessionStore) Close() {
	close(s.stopChan)
}

// cleanupExpired runs in a goroutine to periodically clean up expired sessions
func (s *InMemorySessionStore) cleanupExpired() {
	ticker := time.NewTicker(time.Hour) // Run cleanup every hour
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			s.Cleanup()
		case <-s.stopChan:
			return
		}
	}
}
