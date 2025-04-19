package stores

// A very small, concurrency‑safe in‑memory implementation of a TaskStore that
// is good enough for demos and unit tests.  It intentionally keeps the surface
// minimal – just what is required by the built‑in orchestration tool today.  A
// production‑grade implementation would persist to an external database and
// provide richer querying / filtering facilities.

import (
	"sync"
	"time"

	"github.com/theapemachine/a2a-go/pkg/types"
)

// TaskEntry stores the bare minimum we need to track long‑running (sub)‑tasks
// created by the orchestration tool.  We reuse the TaskState type that is
// already defined for the wire format so the values match the public spec.
type TaskEntry struct {
	ID string
	// ParentID is nil for top‑level tasks.  Child tasks store the ID of their
	// immediate parent to enable hierarchical traversal and orchestration.
	ParentID    *string
	Description string
	State       types.TaskState
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// InMemoryTaskStore satisfies the (future) TaskStore interface expected by
// higher‑level orchestration logic.  For now we only expose a handful of
// helper methods – they can easily be expanded in the future.
type InMemoryTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*TaskEntry
}

func NewInMemoryTaskStore() *InMemoryTaskStore {
	return &InMemoryTaskStore{
		tasks: make(map[string]*TaskEntry),
	}
}

// Create registers a new task in the store and returns its ID.  The caller is
// responsible for passing a unique ID (for example ULID or uuid).
func (s *InMemoryTaskStore) Create(id, desc string) *TaskEntry {
	now := time.Now().UTC()
	entry := &TaskEntry{
		ID:          id,
		ParentID:    nil,
		Description: desc,
		State:       types.TaskStateSubmitted,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.mu.Lock()
	s.tasks[id] = entry
	s.mu.Unlock()
	return entry
}

// CreateChild registers a child task that is logically part of the execution
// tree of the supplied parent task.  Aside from the ParentID linkage it is
// identical to Create.
func (s *InMemoryTaskStore) CreateChild(id, desc, parentID string) *TaskEntry {
	entry := s.Create(id, desc)
	entry.ParentID = &parentID
	return entry
}

func (s *InMemoryTaskStore) Get(id string) (*TaskEntry, bool) {
	s.mu.RLock()
	e, ok := s.tasks[id]
	s.mu.RUnlock()
	return e, ok
}

func (s *InMemoryTaskStore) UpdateState(id string, state types.TaskState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.tasks[id]
	if !ok {
		return false
	}
	e.State = state
	e.UpdatedAt = time.Now().UTC()
	return true
}

// List returns a snapshot of all tasks currently in the store.  Intended for
// debug / inspection only – no filtering or pagination.
func (s *InMemoryTaskStore) List() []*TaskEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*TaskEntry, 0, len(s.tasks))
	for _, e := range s.tasks {
		out = append(out, e)
	}
	return out
}
