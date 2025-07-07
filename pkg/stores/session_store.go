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
}

func NewInMemorySessionStore() *InMemorySessionStore {
	store := &InMemorySessionStore{
		data:       make(map[string]*sessionData),
		expiration: 24 * time.Hour, // Default 24 hour expiration
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
		s.Delete(id)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	for id, sessionData := range s.data {
		if now.After(sessionData.ExpiresAt) {
			delete(s.data, id)
		}
	}
}

// cleanupExpired runs in a goroutine to periodically clean up expired sessions
func (s *InMemorySessionStore) cleanupExpired() {
	ticker := time.NewTicker(time.Hour) // Run cleanup every hour
	defer ticker.Stop()
	
	for range ticker.C {
		s.Cleanup()
	}
}
