package service

// TaskManager defines the server‑side behaviour for the core Task lifecycle
// JSON‑RPC methods.  It is intentionally minimal for Phase‑1: enough to support
// tasks/send, tasks/get and tasks/cancel while remaining easy to extend later
// with streaming, push‑notifications and history.

import (
	"context"
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
	StreamTask(ctx context.Context, params types.TaskSendParams) (<-chan interface{}, *rpcError)
}

// EchoTaskManager is a trivial reference implementation that fulfils every
// task immediately by echoing back the first text part.  It demonstrates the
// contract and makes the “out of the box” server experience pleasant.
type EchoTaskManager struct {
	store *stores.InMemoryTaskStore
}

func NewEchoTaskManager(store *stores.InMemoryTaskStore) *EchoTaskManager {
	if store == nil {
		store = stores.NewInMemoryTaskStore()
	}
	return &EchoTaskManager{store: store}
}

func (m *EchoTaskManager) SendTask(ctx context.Context, p types.TaskSendParams) (types.Task, *rpcError) {
	// extract first text part
	txt := ""
	if len(p.Message.Parts) > 0 && p.Message.Parts[0].Type == types.PartTypeText {
		txt = p.Message.Parts[0].Text
	}

	entry := m.store.Create(p.ID, txt)

	// Build response task
	now := time.Now().UTC()
	task := types.Task{
		ID: p.ID,
		Status: types.TaskStatus{
			State:     types.TaskStateCompleted,
			Timestamp: &now,
		},
		Artifacts: []types.Artifact{{
			Parts: []types.Part{{Type: types.PartTypeText, Text: txt}},
			Index: 0,
		}},
	}
	entry.State = types.TaskStateCompleted
	return task, nil
}

func (m *EchoTaskManager) GetTask(ctx context.Context, id string, historyLength int) (types.Task, *rpcError) {
	// For the echo manager we don’t keep full tasks in store, so return a not
	// found error unless SendTask already produced it.
	e, ok := m.store.Get(id)
	if !ok {
		return types.Task{}, &rpcError{Code: -32001, Message: "Task not found"}
	}
	task := types.Task{
		ID: e.ID,
		Status: types.TaskStatus{
			State: e.State,
		},
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
func (m *EchoTaskManager) StreamTask(ctx context.Context, p types.TaskSendParams) (<-chan interface{}, *rpcError) {
	ch := make(chan interface{}, 4)

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
	m.store.Create(p.ID, p.Message.Parts[0].Text)
	m.store.UpdateState(p.ID, types.TaskStateWorking)

	return ch, nil
}
