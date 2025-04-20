package sampling

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	errors "github.com/theapemachine/a2a-go/pkg/errors"
)

func CreateMessage(
	ctx context.Context,
	raw json.RawMessage,
	handler *MCPHandler,
) (any, *errors.RpcError) {
	var req mcp.CreateMessageRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, errors.ErrInvalidParams
	}
	res, err := handler.HandleCreateMessage(ctx, &req)
	if err != nil {
		return nil, &errors.RpcError{Code: -32000, Message: err.Error()}
	}
	return res, nil
}
