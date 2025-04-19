package service

// TaskManager defines the server‑side behaviour for the core Task lifecycle
// JSON‑RPC methods.  It is intentionally minimal for Phase‑1: enough to support
// tasks/send, tasks/get and tasks/cancel while remaining easy to extend later
// with streaming, push‑notifications and history.

import (
	"context"
	"fmt"
	"time"

	"github.com/theapemachine/a2a-go/pkg/stores"
	"github.com/theapemachine/a2a-go/pkg/types"
)

// TaskManager is plugged into an A2AServer.  Each method should do its own
// validation and return a *rpcError value if the request is invalid or cannot
// be fulfilled.
type TaskManager interface {
	SendTask(ctx context.Context, params types.TaskSendParams) (types.Task, *rpcError)
	GetTask(ctx context.Context, id string, historyLength int) (types.Task, *rpcError)
	CancelTask(ctx context.Context, id string) (types.Task, *rpcError)

	// StreamTask starts processing the task and returns a read‑only channel from
	// which the caller will receive TaskStatusUpdateEvent or TaskArtifactUpdate
	// objects until the task finishes (the final flag will be set on the last
	// status event).  The channel should be closed by the TaskManager when the
	// stream is finished.
	StreamTask(ctx context.Context, params types.TaskSendParams) (<-chan any, *rpcError)
	
	// ResubscribeTask reconnects to an existing task's event stream
	ResubscribeTask(ctx context.Context, id string, historyLength int) (<-chan any, *rpcError)
	
	// SetPushNotification configures a push notification URL for a task
	SetPushNotification(ctx context.Context, config types.TaskPushNotificationConfig) (types.TaskPushNotificationConfig, *rpcError)
	
	// GetPushNotification retrieves the current push notification configuration for a task
	GetPushNotification(ctx context.Context, id string) (types.TaskPushNotificationConfig, *rpcError)
}

// EchoTaskManager is a trivial reference implementation that fulfils every
// task immediately by echoing back the first text part.  It demonstrates the
// contract and makes the "out of the box" server experience pleasant.
type EchoTaskManager struct {
	store *stores.InMemoryTaskStore
}

func NewEchoTaskManager(store *stores.InMemoryTaskStore) *EchoTaskManager {
	if store == nil {
		store = stores.NewInMemoryTaskStore()
	}
	return &EchoTaskManager{store: store}
}

// GetStore returns the underlying task store for direct access.
// This is useful for examples and test code.
func (m *EchoTaskManager) GetStore() *stores.InMemoryTaskStore {
	return m.store
}

func (m *EchoTaskManager) SendTask(ctx context.Context, p types.TaskSendParams) (types.Task, *rpcError) {
	// Validate parts according to A2A spec
	if len(p.Message.Parts) > 0 {
		for i, part := range p.Message.Parts {
			if err := part.Validate(); err != nil {
				return types.Task{}, &rpcError{
					Code:    -32602, // Invalid params error code
					Message: fmt.Sprintf("Invalid part at index %d: %v", i, err),
				}
			}
		}
	}
	
	// extract first text part
	txt := ""
	if len(p.Message.Parts) > 0 && p.Message.Parts[0].Type == types.PartTypeText {
		txt = p.Message.Parts[0].Text
	}

	entry := m.store.Create(p.ID, txt)
	
	// Set session ID if provided
	if p.SessionID != "" {
		entry.SessionID = p.SessionID
	}
	
	// Store the message in history
	m.store.AddMessageToHistory(p.ID, p.Message)
	
	// Set push notification if provided
	if p.PushNotification != nil {
		m.store.SetPushNotification(p.ID, *p.PushNotification)
	}

	// Build response task
	now := time.Now().UTC()
	task := types.Task{
		ID:        p.ID,
		SessionID: entry.SessionID,
		Status: types.TaskStatus{
			State:     types.TaskStateCompleted,
			Timestamp: &now,
		},
		Artifacts: []types.Artifact{{
			Parts: []types.Part{{Type: types.PartTypeText, Text: txt}},
			Index: 0,
		}},
	}
	
	// Add history if requested
	if p.HistoryLength > 0 {
		task.History = m.store.GetHistory(p.ID, p.HistoryLength)
	}
	
	entry.State = types.TaskStateCompleted
	return task, nil
}

func (m *EchoTaskManager) GetTask(ctx context.Context, id string, historyLength int) (types.Task, *rpcError) {
	// For the echo manager we don't keep full tasks in store, so return a not
	// found error unless SendTask already produced it.
	e, ok := m.store.Get(id)
	if !ok {
		return types.Task{}, &rpcError{Code: -32001, Message: "Task not found"}
	}
	
	// Build the basic task response
	task := types.Task{
		ID: e.ID,
		SessionID: e.SessionID, // Set the SessionID field
		Status: types.TaskStatus{
			State: e.State,
			Timestamp: &e.UpdatedAt, // Include the last update timestamp
		},
	}

	// Add artifacts if the task is completed
	if e.State == types.TaskStateCompleted && e.Description != "" {
		task.Artifacts = []types.Artifact{{
			Parts: []types.Part{{Type: types.PartTypeText, Text: e.Description}},
			Index: 0,
		}}
	}

	// Include history if historyLength > 0
	if historyLength > 0 {
		task.History = m.store.GetHistory(id, historyLength)
	}
	
	return task, nil
}

func (m *EchoTaskManager) CancelTask(ctx context.Context, id string) (types.Task, *rpcError) {
	ok := m.store.UpdateState(id, types.TaskStateCanceled)
	if !ok {
		return types.Task{}, &rpcError{Code: -32001, Message: "Task not found"}
	}
	task := types.Task{ID: id, Status: types.TaskStatus{State: types.TaskStateCanceled}}
	return task, nil
}

// StreamTask implements a trivial streaming simulation: it first emits a
// "working" status, waits briefly, sends the final artifact, then a completed
// status with final=true.
func (m *EchoTaskManager) StreamTask(ctx context.Context, p types.TaskSendParams) (<-chan any, *rpcError) {
	// Validate parts according to A2A spec
	if len(p.Message.Parts) > 0 {
		for i, part := range p.Message.Parts {
			if err := part.Validate(); err != nil {
				return nil, &rpcError{
					Code:    -32602, // Invalid params error code
					Message: fmt.Sprintf("Invalid part at index %d: %v", i, err),
				}
			}
		}
	}

	ch := make(chan any, 4)

	// first status
	ch <- types.TaskStatusUpdateEvent{
		ID: p.ID,
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		Final: false,
	}

	go func() {
		defer close(ch)
		// simulate work
		time.Sleep(200 * time.Millisecond)

		// artifact
		ch <- types.TaskArtifactUpdateEvent{
			ID: p.ID,
			Artifact: types.Artifact{
				Parts: []types.Part{{Type: types.PartTypeText, Text: "streamed echo: " + p.Message.Parts[0].Text}},
				Index: 0,
			},
		}

		// final status
		ch <- types.TaskStatusUpdateEvent{
			ID:     p.ID,
			Status: types.TaskStatus{State: types.TaskStateCompleted},
			Final:  true,
		}
	}()

	// update store immediately so GetTask sees it.
	entry := m.store.Create(p.ID, p.Message.Parts[0].Text)
	
	// Set session ID if provided
	if p.SessionID != "" {
		entry.SessionID = p.SessionID
	}
	
	// Store message in history
	m.store.AddMessageToHistory(p.ID, p.Message)
	
	m.store.UpdateState(p.ID, types.TaskStateWorking)

	return ch, nil
}

// ResubscribeTask reconnects to an existing task's event stream.
// It creates a new channel and sends both the artifact and final completion status.
func (m *EchoTaskManager) ResubscribeTask(ctx context.Context, id string, historyLength int) (<-chan any, *rpcError) {
	// Check if the task exists
	e, ok := m.store.Get(id)
	if !ok {
		return nil, &rpcError{Code: -32001, Message: "Task not found"}
	}

	// Create a channel to send updates
	ch := make(chan any, 3) // Increased buffer size to accommodate history

	// For the echo manager, send the artifact (if available) and final status
	go func() {
		defer close(ch)
		
		// Send artifact if we have text content
		if e.Description != "" {
			ch <- types.TaskArtifactUpdateEvent{
				ID: id,
				Artifact: types.Artifact{
					Parts: []types.Part{{Type: types.PartTypeText, Text: e.Description}},
					Index: 0,
				},
			}
		}
		
		// If history is requested, send a history update event
		if historyLength > 0 {
			history := m.store.GetHistory(id, historyLength)
			if len(history) > 0 {
				// We don't have a specific history event type in the A2A spec,
				// so we'll include history in the metadata of the status update
				ch <- types.TaskStatusUpdateEvent{
					ID: id,
					Status: types.TaskStatus{
						State: types.TaskStateWorking,
					},
					Metadata: map[string]any{
						"history": history,
					},
					Final: false,
				}
			}
		}
		
		// Send final status with timestamp
		now := time.Now().UTC()
		ch <- types.TaskStatusUpdateEvent{
			ID: id,
			Status: types.TaskStatus{
				State: e.State,
				Timestamp: &now,
			},
			Final: true,
		}
	}()

	return ch, nil
}

// SetPushNotification configures a push notification URL for a task.
func (m *EchoTaskManager) SetPushNotification(ctx context.Context, config types.TaskPushNotificationConfig) (types.TaskPushNotificationConfig, *rpcError) {
	// Check if the task exists
	_, ok := m.store.Get(config.ID)
	if !ok {
		return types.TaskPushNotificationConfig{}, &rpcError{Code: -32001, Message: "Task not found"}
	}
	
	// Validate the push notification config
	if config.PushNotificationConfig.URL == "" {
		return types.TaskPushNotificationConfig{}, &rpcError{Code: -32602, Message: "URL is required for push notifications"}
	}
	
	// Store the push notification configuration
	success := m.store.SetPushNotification(config.ID, config.PushNotificationConfig)
	if !success {
		return types.TaskPushNotificationConfig{}, &rpcError{Code: -32002, Message: "Failed to set push notification"}
	}
	
	return config, nil
}

// GetPushNotification retrieves the current push notification configuration for a task.
func (m *EchoTaskManager) GetPushNotification(ctx context.Context, id string) (types.TaskPushNotificationConfig, *rpcError) {
	// Check if the task exists
	e, ok := m.store.Get(id)
	if !ok {
		return types.TaskPushNotificationConfig{}, &rpcError{Code: -32001, Message: "Task not found"}
	}
	
	// Retrieve the stored push notification configuration
	// If none exists, return an empty one
	config := types.PushNotificationConfig{
		URL: "", // Default empty URL
	}
	
	if e.PushNotification != nil {
		config = *e.PushNotification
	}
	
	return types.TaskPushNotificationConfig{
		ID: id,
		PushNotificationConfig: config,
	}, nil
}