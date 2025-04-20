package tasks

import (
	"context"
	"encoding/json"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func SetPushNotification(
	ctx context.Context,
	raw json.RawMessage,
	tm TaskManager,
) (any, *errors.RpcError) {
	var config types.TaskPushNotificationConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, errors.ErrInvalidParams
	}
	taskPushConfig, rpcErr := tm.SetPushNotification(ctx, config)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return taskPushConfig, nil
}
