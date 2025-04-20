package tasks

import (
	"context"
	"encoding/json"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
)

func Get(
	ctx context.Context,
	raw json.RawMessage,
	tm TaskManager,
) (any, *errors.RpcError) {
	var qp struct {
		ID            string `json:"id"`
		HistoryLength int    `json:"historyLength,omitempty"`
	}
	if err := json.Unmarshal(raw, &qp); err != nil {
		return nil, errors.ErrInvalidParams
	}
	task, rpcErr := tm.GetTask(ctx, qp.ID, qp.HistoryLength)
	if rpcErr != nil {
		return nil, rpcErr
	}
	return task, nil
}
