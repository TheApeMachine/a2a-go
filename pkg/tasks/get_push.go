package tasks

import (
	"context"
	"encoding/json"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
)

func GetPushNotification(
	ctx context.Context,
	raw json.RawMessage,
	tm TaskManager,
) (any, *errors.RpcError) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.ErrInvalidParams
	}
	taskPushConfig, rpcErr := tm.GetPushNotification(ctx, p.ID)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return taskPushConfig, nil
}
