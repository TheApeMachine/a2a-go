package ai

import (
	"context"

	"github.com/charmbracelet/log"
	errors "github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func (agent *Agent) SendTask(
	ctx context.Context,
	params types.Task,
) (types.Task, *errors.RpcError) {
	log.Info("task received", "agent", agent.card.Name, "task", params.ID)

	tools := agent.card.Tools()
	agent.chatClient.Complete(ctx, &params, &tools)

	return params, nil
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
	params types.Task,
) (types.Task, *errors.RpcError) {
	tools := agent.card.Tools()

	agent.chatClient.Stream(
		ctx, &params, &tools, agent.notifier,
	)

	return params, nil
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
