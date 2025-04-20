package tasks

import (
	"context"
	"encoding/json"

	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func Send(
	ctx context.Context,
	raw json.RawMessage,
	tm types.TaskManager,
) (any, *errors.RpcError) {
	var params types.TaskSendParams

	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, errors.ErrInvalidParams
	}

	task, rpcErr := tm.SendTask(ctx, params)

	if rpcErr != nil {
		return nil, rpcErr
	}

	return task, nil
}
