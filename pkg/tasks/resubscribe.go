package tasks

import (
	"context"
	"encoding/json"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/service/sse"
)

func ResubscribeTask(
	ctx context.Context,
	raw json.RawMessage,
	tm TaskManager,
	broker *sse.SSEBroker,
) (any, *errors.RpcError) {
	var p struct {
		ID            string `json:"id"`
		HistoryLength int    `json:"historyLength,omitempty"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.ErrInvalidParams
	}
	stream, rpcErr := tm.ResubscribeTask(ctx, p.ID, p.HistoryLength)
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Consume first event to return immediately per JSON‑RPC semantics
	var first any
	select {
	case first = <-stream:
	default:
		// no event yet – return empty result
		first = nil
	}

	// Forward rest of events to SSE broker
	go func() {
		for evt := range stream {
			_ = broker.Broadcast(evt)
		}
	}()

	return first, nil
}
