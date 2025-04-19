package roots

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/google/uuid"
)

// Manager offers CRUD + list.  For now it is backed by an in‑memory map.

type Manager struct {
    mu    sync.RWMutex
    roots map[string]*Root
}

func NewManager() *Manager {
    m := &Manager{roots: map[string]*Root{}}
    // seed a generic “file:///” root.
    now := time.Now()
    r := &Root{ID: uuid.NewString(), URI: "file:///", Name: "Local Files", CreatedAt: now, UpdatedAt: now}
    m.roots[r.ID] = r
    return m
}

func (m *Manager) List(ctx context.Context) ([]Root, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    arr := make([]Root, 0, len(m.roots))
    for _, r := range m.roots {
        arr = append(arr, *r)
    }
    return arr, nil
}

func (m *Manager) Create(ctx context.Context, root Root) (*Root, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    for _, existing := range m.roots {
        if existing.URI == root.URI {
            return nil, fmt.Errorf("root already exists: %s", root.URI)
        }
    }
    if root.ID == "" {
        root.ID = uuid.NewString()
    }
    now := time.Now()
    root.CreatedAt, root.UpdatedAt = now, now
    m.roots[root.ID] = &root
    return &root, nil
}
