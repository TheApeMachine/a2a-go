package tasks

import (
	"context"
	"encoding/json"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/service/sse"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func SendSubscribe(
	ctx context.Context,
	raw json.RawMessage,
	tm TaskManager,
	broker *sse.SSEBroker,
) (any, *errors.RpcError) {
	var params types.TaskSendParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, errors.ErrInvalidParams
	}

	stream, rpcErr := tm.StreamTask(ctx, params)
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Consume first event to return immediately per JSON‑RPC semantics.
	var first any
	select {
	case first = <-stream:
	default:
		// no event yet – fabricate a working status so caller gets something
		first = types.TaskStatusUpdateEvent{
			ID:     params.ID,
			Status: types.TaskStatus{State: types.TaskStateWorking},
			Final:  false,
		}
	}

	// forward rest of events to SSE broker
	go func() {
		for evt := range stream {
			_ = broker.Broadcast(evt)
		}
	}()

	return first, nil
}
