package ai

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/stores"
)

type TaskManager struct {
	agent     *a2a.AgentCard
	taskStore stores.TaskStore
	provider  *provider.OpenAIProvider
}

type TaskManagerOption func(*TaskManager)

func NewTaskManager(
	card *a2a.AgentCard, options ...TaskManagerOption,
) (*TaskManager, error) {
	taskManager := &TaskManager{
		agent: card,
	}

	for _, option := range options {
		option(taskManager)
	}

	if taskManager.taskStore == nil {
		log.Error("missing task store")
		return nil, errors.NewError(errors.ErrMissingTaskStore{})
	}

	if taskManager.provider == nil {
		log.Error("missing provider")
		return nil, errors.NewError(errors.ErrMissingProvider{})
	}

	return taskManager, nil
}

func (manager *TaskManager) handleUpdate(
	params *a2a.Task,
	chunk jsonrpc.Response,
) error {
	if chunk.Error != nil {
		log.Error("failed to handle update", "error", chunk.Error)
		return (*errors.RpcError)(chunk.Error)
	}

	switch result := chunk.Result.(type) {
	case a2a.TaskStatusUpdateResult:
		params.ToStatus(result.Status.State, result.Status.Message)
	case a2a.TaskArtifactUpdateEvent:
		params.AddArtifact(result.Artifact)
	}

	return nil
}

func (manager *TaskManager) selectTask(
	ctx context.Context,
	params a2a.TaskSendParams,
) (*a2a.Task, *errors.RpcError) {
	existing, err := manager.taskStore.Get(ctx, params.ID, 0)

	if err != nil {
		log.Error("failed to get existing task", "error", err)
	}

	if existing != nil {
		return existing, nil
	}

	task := &a2a.Task{
		ID:        params.ID,
		SessionID: params.SessionID,
		History:   []a2a.Message{params.Message},
		Status: a2a.TaskStatus{
			State:   a2a.TaskStateSubmitted,
			Message: a2a.NewTextMessage(manager.agent.Name, "task submitted"),
		},
	}

	if err := manager.taskStore.Create(ctx, task); err != nil {
		log.Error("failed to create task", "error", err)
		return nil, err
	}

	return task, nil
}

func (manager *TaskManager) SendTask(
	ctx context.Context, params a2a.TaskSendParams,
) (*a2a.Task, *errors.RpcError) {
	task, err := manager.selectTask(ctx, params)

	if err != nil {
		log.Error("failed to select task", "error", err)
		return nil, err
	}

	task.ToStatus(a2a.TaskStateWorking,
		a2a.NewTextMessage(
			manager.agent.Name,
			"starting task",
		),
	)

	for chunk := range manager.provider.Generate(
		ctx, provider.NewProviderParams(
			task, provider.WithTools(manager.agent.Tools()...),
		),
	) {
		if err := manager.handleUpdate(task, chunk); err != nil {
			log.Error("failed to handle update", "error", err)
			return task, err.(*errors.RpcError)
		}
	}

	return task, nil
}

/*
StreamTask handles a streaming task request.

Returns:
- A task if it exists.
- *errors.RpcError if the task was not found.
*/
func (manager *TaskManager) StreamTask(
	ctx context.Context,
	params *a2a.Task,
) (chan jsonrpc.Response, *errors.RpcError) {
	params.ToStatus(
		a2a.TaskStateWorking,
		a2a.NewTextMessage(
			manager.agent.Name,
			"starting task",
		),
	)

	if err := manager.taskStore.Create(ctx, params); err != nil {
		return nil, err
	}

	out := make(chan jsonrpc.Response)

	for chunk := range manager.provider.Generate(
		ctx, provider.NewProviderParams(params),
	) {
		if err := manager.handleUpdate(params, chunk); err != nil {
			log.Error("failed to handle update", "error", err)
			return nil, err.(*errors.RpcError)
		}

		out <- chunk
	}

	return out, nil
}

/*
GetTask retrieves the current state of a task.

Returns:
- A task if it exists.
- *errors.RpcError if the task was not found.
*/
func (manager *TaskManager) GetTask(
	ctx context.Context,
	id string,
	historyLength int,
) (*a2a.Task, *errors.RpcError) {
	return manager.taskStore.Get(ctx, id, historyLength)
}

/*
CancelTask attempts to cancel an ongoing task.

Returns:
- nil if the task was successfully cancelled.
- *errors.RpcError if the task was not found or could not be cancelled.
*/
func (manager *TaskManager) CancelTask(
	ctx context.Context, id string,
) *errors.RpcError {
	return manager.taskStore.Cancel(ctx, id)
}

/*
ResubscribeTask allows a client to resubscribe to task events.

Returns:
- A channel of task events.
- *errors.RpcError if the task was not found or could not be resubscribed to.
*/
func (manager *TaskManager) ResubscribeTask(
	ctx context.Context, id string, historyLength int,
) (<-chan a2a.Task, *errors.RpcError) {
	ch := make(chan a2a.Task)

	if err := manager.taskStore.Subscribe(ctx, id, ch); err != nil {
		log.Error("failed to subscribe to task", "error", err)
		return nil, err
	}

	return ch, nil
}

func WithTaskStore(taskStore stores.TaskStore) TaskManagerOption {
	return func(t *TaskManager) {
		t.taskStore = taskStore
	}
}

func WithProvider(provider *provider.OpenAIProvider) TaskManagerOption {
	return func(t *TaskManager) {
		t.provider = provider
	}
}
