package ai

import (
	"context"
	"time"

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
	provider  provider.Interface
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
		log.Debug("failed to handle update (raw chunk error)",
			"code", chunk.Error.Code,
			"message", chunk.Error.Message,
		)
		log.Debug("chunk error data", "data", chunk.Error.Data)
		return &errors.RpcError{
			Code:    chunk.Error.Code,
			Message: chunk.Error.Message,
			Data:    chunk.Error.Data,
		}
	}

	switch result := chunk.Result.(type) {
	case a2a.TaskStatusUpdateResult:
		params.ToStatus(result.Status.State, result.Status.Message)
	case a2a.TaskArtifactUpdateEvent:
		params.AddArtifact(result.Artifact)
	}

	return nil
}

func (manager *TaskManager) createNewTask(ctx context.Context, params a2a.TaskSendParams) (*a2a.Task, *errors.RpcError) {
	log.Info("creating new task", "task_id", params.ID, "session_id", params.SessionID)
	newTask := a2a.NewTask(manager.agent.Name)
	newTask.ID = params.ID
	if params.SessionID != "" {
		newTask.SessionID = params.SessionID
	}
	newTask.History = append(newTask.History, params.Message)
	newTask.ToStatus(a2a.TaskStateSubmitted,
		a2a.NewTextMessage(manager.agent.Name, "task created and submitted"),
	)
	if createErr := manager.taskStore.Create(ctx, newTask, manager.agent.Name); createErr != nil {
		log.Error("failed to create new task in store", "task_id", params.ID, "error", createErr)
		return nil, createErr
	}
	log.Info("newly created task stored", "task_id", newTask.ID, "status", newTask.Status.State)
	return newTask, nil
}

func (manager *TaskManager) selectTask(
	ctx context.Context,
	params a2a.TaskSendParams,
) (a2a.Task, *errors.RpcError) {
	existingTasks, getErr := manager.taskStore.Get(
		ctx, manager.agent.Name+"/"+params.ID, 0,
	)

	if getErr != nil {
		errMsg := getErr.Error()
		if errMsg == errors.ErrTaskNotFound.Error() {
			task, err := manager.createNewTask(ctx, params)
			if err != nil {
				return a2a.Task{}, err
			}
			return *task, nil
		}
		log.Error("error getting task from store (not ErrTaskNotFound)", "task_id", params.ID, "error", getErr)
		return a2a.Task{}, getErr
	}

	if len(existingTasks) == 0 {
		task, err := manager.createNewTask(ctx, params)
		if err != nil {
			return a2a.Task{}, err
		}
		return *task, nil
	}

	mostRecentTimestamp := time.Unix(0, 0).UTC()
	var mostRecentTask a2a.Task

	for _, task := range existingTasks {
		if task.Status.Timestamp.After(mostRecentTimestamp) || task.Status.Timestamp.Equal(mostRecentTimestamp) {
			mostRecentTimestamp = task.Status.Timestamp
			mostRecentTask = task
		}
	}

	mostRecentTask.History = append(mostRecentTask.History, params.Message)

	if updateErr := manager.taskStore.Update(ctx, &mostRecentTask, manager.agent.Name); updateErr != nil {
		log.Error("failed to update existing task in store after appending message", "task_id", mostRecentTask.ID, "error", updateErr)
		return a2a.Task{}, updateErr
	}

	return mostRecentTask, nil
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

	prvdrParams := provider.NewProviderParams(
		&task, provider.WithTools(manager.agent.Tools()...),
	)

	prvdrParams.Stream = false

	for chunk := range manager.provider.Generate(
		ctx, prvdrParams,
	) {
		if err := manager.handleUpdate(&task, chunk); err != nil {
			log.Error("failed to handle update", "error", err)
			return &task, err.(*errors.RpcError)
		}
	}

	return &task, nil
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
		a2a.NewTextMessage(manager.agent.Name, "starting task"),
	)

	if createErr := manager.taskStore.Create(ctx, params, manager.agent.Name); createErr != nil {
		log.Error("failed to create task in store for streaming", "task_id", params.ID, "error", createErr)
		return nil, createErr
	}

	out := make(chan jsonrpc.Response)

	go func() {
		defer close(out) // Ensure out is closed when this goroutine exits

		providerChan := manager.provider.Generate(ctx, provider.NewProviderParams(params))
		for {
			select {
			case <-ctx.Done(): // If the overall context for StreamTask is done/cancelled
				log.Info("StreamTask context done, exiting stream processing.", "task_id", params.ID)
				return
			case chunk, ok := <-providerChan:
				if !ok { // providerChan was closed, normal completion of provider stream
					return // Goroutine finishes, out will be closed by defer
				}

				if err := manager.handleUpdate(params, chunk); err != nil {
					log.Error("failed to handle update during stream, stopping stream", "task_id", params.ID, "error", err)
					// Error logged, goroutine will exit, and 'out' will be closed by defer.
					// The client will see any chunks sent before this error, then the channel closes.
					return
				}

				// Send the processed chunk to the output channel
				select {
				case out <- chunk:
					// Chunk sent successfully
				case <-ctx.Done():
					log.Info("StreamTask context done while sending chunk to output, exiting stream processing.", "task_id", params.ID)
					return
				}
			}
		}
	}()

	return out, nil // Return immediately
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
	tasks, err := manager.taskStore.Get(ctx, id, historyLength)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, errors.ErrTaskNotFound
	}

	// If multiple task versions are returned, use the most recent one
	if len(tasks) > 1 {
		mostRecentTimestamp := time.Unix(0, 0).UTC()
		mostRecentIdx := 0

		for i, task := range tasks {
			if task.Status.Timestamp.After(mostRecentTimestamp) || task.Status.Timestamp.Equal(mostRecentTimestamp) {
				mostRecentTimestamp = task.Status.Timestamp
				mostRecentIdx = i
			}
		}

		return &tasks[mostRecentIdx], nil
	}

	return &tasks[0], nil
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

func WithProvider(p provider.Interface) TaskManagerOption {
	return func(t *TaskManager) {
		t.provider = p
	}
}
