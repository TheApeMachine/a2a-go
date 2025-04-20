package sampling

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	errors "github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/service/sse"
)

func CreateMessageStream(
	ctx context.Context,
	raw json.RawMessage,
	handler *MCPHandler,
	broker *sse.SSEBroker,
) (any, *errors.RpcError) {
	var req mcp.CreateMessageRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, errors.ErrInvalidParams
	}

	stream, err := handler.HandleStreamMessage(ctx, &req)
	if err != nil {
		return nil, &errors.RpcError{Code: -32000, Message: err.Error()}
	}

	// retrieve first result synchronously
	var first *mcp.CreateMessageResult
	select {
	case first = <-stream:
	default:
		// if none ready yet produce empty chunk so caller can start reading.
		first = &mcp.CreateMessageResult{}
	}

	// forward remainder asynchronously to SSE.
	go func() {
		for res := range stream {
			_ = broker.Broadcast(res)
		}
	}()

	return first, nil
}
