package s3

import (
	"bytes"
	"context"
	"encoding/json"
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
	ctx context.Context, id string, historyLength int,
) (*a2a.Task, *errors.RpcError) {
	buf, err := store.conn.Get(ctx, "tasks", id)

	if err != nil {
		log.Error("failed to get task", "error", err)
		return nil, errors.ErrTaskNotFound
	}

	var task a2a.Task
	if err := json.Unmarshal(buf.Bytes(), &task); err != nil {
		log.Error("failed to unmarshal task", "error", err)
		return nil, errors.ErrInternal.WithMessagef("failed to unmarshal task: %v", err)
	}

	return &task, nil
}

/*
Subscribe sets up a channel to receive task updates from S3.
*/
func (store *Store) Subscribe(
	ctx context.Context, id string, ch chan a2a.Task,
) *errors.RpcError {
	actual, loaded := store.subscriptions.LoadOrStore(id, []chan a2a.Task{ch})

	if loaded {
		subscribers := actual.([]chan a2a.Task)
		subscribers = append(subscribers, ch)
		store.subscriptions.Store(id, subscribers)
	}

	return nil
}

/*
Create stores a new task in S3.
*/
func (store *Store) Create(ctx context.Context, task *a2a.Task) *errors.RpcError {
	data, err := json.Marshal(task)
	if err != nil {
		log.Error("failed to marshal task", "error", err)
		return errors.ErrInternal.WithMessagef("failed to marshal task: %v", err)
	}

	if err := store.conn.Put(ctx, "tasks", task.ID, bytes.NewReader(data)); err != nil {
		log.Error("failed to store task", "error", err, "task", task)
		return errors.ErrInternal.WithMessagef("failed to store task: %v", err)
	}

	return nil
}

/*
Update modifies an existing task in S3.
*/
func (store *Store) Update(ctx context.Context, task *a2a.Task) *errors.RpcError {
	data, err := json.Marshal(task)
	if err != nil {
		log.Error("failed to marshal task", "error", err)
		return errors.ErrInternal.WithMessagef("failed to marshal task: %v", err)
	}

	if err := store.conn.Put(ctx, "tasks", task.ID, bytes.NewReader(data)); err != nil {
		log.Error("failed to update task", "error", err)
		return errors.ErrInternal.WithMessagef("failed to update task: %v", err)
	}

	return nil
}

/*
Delete removes a task from S3 storage.
*/
func (store *Store) Delete(ctx context.Context, id string) *errors.RpcError {
	if err := store.conn.client.RemoveObject(
		ctx,
		"tasks",
		id,
		minio.RemoveObjectOptions{},
	); err != nil {
		log.Error("failed to delete task", "error", err)
		return errors.ErrInternal.WithMessagef("failed to delete task: %v", err)
	}

	store.subscriptions.Delete(id)
	return nil
}

/*
Cancel marks a task as cancelled in S3 storage.
*/
func (store *Store) Cancel(ctx context.Context, id string) *errors.RpcError {
	task, rpcErr := store.Get(ctx, id, 0)

	if rpcErr != nil {
		log.Error("failed to get task", "error", rpcErr)
		return rpcErr
	}

	task.Status.State = a2a.TaskStateCanceled
	return store.Update(ctx, task)
}
