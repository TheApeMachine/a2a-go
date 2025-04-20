package tasks

import (
	"context"
	"encoding/json"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func GetPushNotification(
	ctx context.Context,
	raw json.RawMessage,
	tm types.TaskManager,
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
