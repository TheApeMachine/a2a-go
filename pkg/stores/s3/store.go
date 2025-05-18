package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/minio/minio-go/v7"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/errors"
)

/*
Store provides an S3 implementation of the TaskStore interface.
It uses an S3 connection to store and retrieve task data.
*/
type Store struct {
	conn *Conn
	// subscriptions tracks active subscriptions by task ID
	subscriptions sync.Map
}

/*
NewStore creates a new S3-based task store with the given connection.
*/
func NewStore(conn *Conn) *Store {
	return &Store{conn: conn}
}

/*
Get retrieves a task by its ID from S3 storage.
*/
func (store *Store) Get(
	ctx context.Context, prefix string, historyLength int,
) ([]a2a.Task, *errors.RpcError) {
	buf, err := store.conn.Get(ctx, "tasks", prefix)

	if err != nil {
		log.Error("failed to get task", "error", err)
		return nil, errors.ErrTaskNotFound
	}

	var tasks []a2a.Task

	if err := json.Unmarshal(buf.Bytes(), &tasks); err != nil {
		log.Error("failed to unmarshal task", "error", err)
		return nil, errors.ErrInternal.WithMessagef("failed to unmarshal task: %v", err)
	}

	return tasks, nil
}

/*
Subscribe sets up a channel to receive task updates from S3.
*/
func (store *Store) Subscribe(
	ctx context.Context, prefix string, ch chan a2a.Task,
) *errors.RpcError {
	actual, loaded := store.subscriptions.LoadOrStore(prefix, []chan a2a.Task{ch})

	if loaded {
		subscribers := actual.([]chan a2a.Task)
		subscribers = append(subscribers, ch)
		store.subscriptions.Store(prefix, subscribers)
	}

	return nil
}

/*
Create stores a new task in S3.
*/
func (store *Store) Create(ctx context.Context, task *a2a.Task, optionals ...string) *errors.RpcError {
	data, err := json.Marshal(task)

	if err != nil {
		log.Error("failed to marshal task", "error", err)
		return errors.ErrInternal.WithMessagef("failed to marshal task: %v", err)
	}

	if err := store.conn.Put(ctx, "tasks", task.Prefix(optionals...), bytes.NewReader(data)); err != nil {
		log.Error("failed to store task", "error", err, "task", task)
		return errors.ErrInternal.WithMessagef("failed to store task: %v", err)
	}

	return nil
}

/*
Update modifies an existing task in S3.
*/
func (store *Store) Update(ctx context.Context, task *a2a.Task, optionals ...string) *errors.RpcError {
	data, err := json.Marshal(task)
	if err != nil {
		log.Error("failed to marshal task", "error", err)
		return errors.ErrInternal.WithMessagef("failed to marshal task: %v", err)
	}

	if err := store.conn.Put(ctx, "tasks", task.Prefix(optionals...), bytes.NewReader(data)); err != nil {
		log.Error("failed to update task", "error", err)
		return errors.ErrInternal.WithMessagef("failed to update task: %v", err)
	}

	return nil
}

/*
Delete does not actually delete any data from the store, rather marks the object
as being deleted, after which is should be ignored by the surrounding system.
In the interest of traceability, and event consistency, hard-deletion are not
a good idea.
*/
func (store *Store) Delete(ctx context.Context, prefix string) *errors.RpcError {
	obj, err := store.conn.client.GetObject(
		ctx, "tasks", prefix, minio.GetObjectOptions{},
	)

	if err != nil {
		log.Error("failed to delete task", "error", err)
		return errors.ErrInternal.WithMessagef("failed to delete task: %v", err)
	}

	_ = obj
	return nil
}

/*
Cancel marks a task as cancelled in S3 storage.
*/
func (store *Store) Cancel(ctx context.Context, prefix string) *errors.RpcError {
	task, rpcErr := store.Get(ctx, prefix, 0)

	if rpcErr != nil {
		log.Error("failed to get task", "error", rpcErr)
		return rpcErr
	}

	// Extract the agent name from the prefix if possible (assuming prefix format: agentName/taskID)
	parts := strings.Split(prefix, "/")
	var optionals []string
	if len(parts) > 1 {
		optionals = append(optionals, parts[0]) // First part should be agent name
	}

	for _, t := range task {
		t.Status.State = a2a.TaskStateCanceled
		if updateErr := store.Update(ctx, &t, optionals...); updateErr != nil {
			log.Error("failed to update task status to canceled", "error", updateErr)
			return updateErr
		}
	}

	return nil
}
