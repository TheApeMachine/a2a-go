package ai

import (
	"context"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func (agent *Agent) SendTask(
	ctx context.Context,
	params types.TaskSendParams,
) (types.Task, *errors.RpcError) {
	task := agent.paramsToTask(params)

	agent.chatClient.Complete(ctx, task.History, nil)

	return task, nil
}

func (agent *Agent) GetTask(
	ctx context.Context,
	id string,
	historyLength int,
) (types.Task, *errors.RpcError) {
	return types.Task{}, nil
}

func (agent *Agent) CancelTask(
	ctx context.Context,
	id string,
) (types.Task, *errors.RpcError) {
	return types.Task{}, nil
}

func (agent *Agent) StreamTask(
	ctx context.Context,
	params types.TaskSendParams,
) (<-chan any, *errors.RpcError) {
	return nil, nil
}

func (agent *Agent) ResubscribeTask(
	ctx context.Context,
	id string,
	historyLength int,
) (<-chan any, *errors.RpcError) {
	return nil, nil
}

func (agent *Agent) SetPushNotification(
	ctx context.Context,
	config types.TaskPushNotificationConfig,
) (types.TaskPushNotificationConfig, *errors.RpcError) {
	return types.TaskPushNotificationConfig{}, nil
}

func (agent *Agent) GetPushNotification(
	ctx context.Context,
	id string,
) (types.TaskPushNotificationConfig, *errors.RpcError) {
	return types.TaskPushNotificationConfig{}, nil
}

func (agent *Agent) paramsToTask(params types.TaskSendParams) types.Task {
	return types.Task{
		ID:        params.ID,
		SessionID: params.SessionID,
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
		History: []types.Message{
			{
				Role:  "user",
				Parts: params.Message.Parts,
			},
		},
		Artifacts: []types.Artifact{},
		Metadata:  map[string]any{},
	}
}
