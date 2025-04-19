package a2a

// SessionStore provides a simple key/value storage for arbitrary JSON‑like
// session state scoped by sessionID.  It is intentionally minimal: callers
// decide the structure of the stored data.  The built‑in implementation is an
// in‑memory map safe for concurrent use which is perfectly sufficient for dev
// & unit tests.  Production deployments can swap in a persistent
// implementation (redis, sql, …).

import "sync"

type SessionStore interface {
    Get(sessionID string) (map[string]interface{}, bool)
    Set(sessionID string, data map[string]interface{})
    Delete(sessionID string)
}

// InMemorySessionStore is the default implementation.
type InMemorySessionStore struct {
    mu   sync.RWMutex
    data map[string]map[string]interface{}
}

func NewInMemorySessionStore() *InMemorySessionStore {
    return &InMemorySessionStore{data: make(map[string]map[string]interface{})}
}

func (s *InMemorySessionStore) Get(id string) (map[string]interface{}, bool) {
    s.mu.RLock()
    v, ok := s.data[id]
    s.mu.RUnlock()
    return v, ok
}

func (s *InMemorySessionStore) Set(id string, d map[string]interface{}) {
    s.mu.Lock()
    s.data[id] = d
    s.mu.Unlock()
}

func (s *InMemorySessionStore) Delete(id string) {
    s.mu.Lock()
    delete(s.data, id)
    s.mu.Unlock()
}
