package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/types"
)

/*
Manager handles state persistence, recovery, and validation.
*/
type Manager struct {
	mu sync.RWMutex
	// Map of task states
	states map[string]*types.Task
	// Path to state directory
	stateDir    string
	subscribers map[string]chan *types.Task
}

/*
NewManager creates a new state manager.
*/
func NewManager(stateDir string) (*Manager, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &Manager{
		states:      make(map[string]*types.Task),
		stateDir:    stateDir,
		subscribers: make(map[string]chan *types.Task),
	}, nil
}

/*
GetTask retrieves a task's state.
*/
func (m *Manager) GetTask(ctx context.Context, id string) (*types.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.states[id]
	if !ok {
		return nil, errors.ErrTaskNotFound.WithMessagef("task %s not found", id)
	}

	return task, nil
}

/*
UpdateTask updates a task's state.
*/
func (m *Manager) UpdateTask(ctx context.Context, task *types.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate state transition
	if err := m.validateStateTransition(task); err != nil {
		return err
	}

	// Update state
	m.states[task.ID] = task

	// Persist state
	if err := m.persistTask(ctx, task); err != nil {
		return err
	}

	// Notify subscribers
	if ch, ok := m.subscribers[task.ID]; ok {
		select {
		case ch <- task:
		default:
			// Drop update if channel is full
		}
	}

	return nil
}

/*
validateStateTransition validates a state transition.
*/
func (m *Manager) validateStateTransition(task *types.Task) error {
	oldTask, ok := m.states[task.ID]
	if !ok {
		return nil // New task, no validation needed
	}

	// Check if state transition is valid
	switch oldTask.Status.State {
	case types.TaskStateSubmitted:
		if task.Status.State != types.TaskStateWorking && task.Status.State != types.TaskStateCanceled {
			return errors.ErrInvalidParams.WithMessagef("invalid state transition from %s to %s", oldTask.Status.State, task.Status.State)
		}
	case types.TaskStateWorking:
		if task.Status.State != types.TaskStateInputReq && task.Status.State != types.TaskStateCompleted && task.Status.State != types.TaskStateFailed && task.Status.State != types.TaskStateCanceled {
			return errors.ErrInvalidParams.WithMessagef("invalid state transition from %s to %s", oldTask.Status.State, task.Status.State)
		}
	case types.TaskStateInputReq:
		if task.Status.State != types.TaskStateWorking && task.Status.State != types.TaskStateFailed && task.Status.State != types.TaskStateCanceled {
			return errors.ErrInvalidParams.WithMessagef("invalid state transition from %s to %s", oldTask.Status.State, task.Status.State)
		}
	case types.TaskStateCompleted, types.TaskStateFailed, types.TaskStateCanceled:
		return errors.ErrInvalidParams.WithMessagef("cannot transition from final state %s", oldTask.Status.State)
	}

	return nil
}

/*
persistTask persists a task's state to disk.
*/
func (m *Manager) persistTask(ctx context.Context, task *types.Task) error {
	path := filepath.Join(m.stateDir, fmt.Sprintf("%s.json", task.ID))
	
	// Create a new file with context
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create state file: %w", err)
	}
	defer f.Close()

	// Check if context is done before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Encode and write the task state
	enc := json.NewEncoder(f)
	if err := enc.Encode(task); err != nil {
		return fmt.Errorf("failed to encode task state: %w", err)
	}

	return nil
}

/*
RecoverTask recovers a task's state from disk.
*/
func (m *Manager) RecoverTask(ctx context.Context, id string) error {
	path := filepath.Join(m.stateDir, fmt.Sprintf("%s.json", id))
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open state file: %w", err)
	}
	defer f.Close()

	var task types.Task
	if err := json.NewDecoder(f).Decode(&task); err != nil {
		return fmt.Errorf("failed to decode task state: %w", err)
	}

	m.mu.Lock()
	m.states[id] = &task
	m.mu.Unlock()

	return nil
}

/*
SubscribeToUpdates subscribes to state updates.
*/
func (m *Manager) SubscribeToUpdates(ctx context.Context, id string) <-chan *types.Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *types.Task, 10)
	m.subscribers[id] = ch

	return ch
}

/*
Cleanup removes old task states.
*/
func (m *Manager) Cleanup(ctx context.Context, maxAge time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, task := range m.states {
		if task.Status.Timestamp != nil && now.Sub(*task.Status.Timestamp) > maxAge {
			delete(m.states, id)
			if err := os.Remove(filepath.Join(m.stateDir, fmt.Sprintf("%s.json", id))); err != nil {
				return fmt.Errorf("failed to remove state file: %w", err)
			}
		}
	}

	return nil
}
