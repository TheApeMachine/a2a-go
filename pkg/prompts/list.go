package prompts

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	errors "github.com/theapemachine/a2a-go/pkg/errors"
)

func List(
	ctx context.Context,
	raw json.RawMessage,
	handler *MCPHandler,
) (any, *errors.RpcError) {
	res, err := handler.HandleListPrompts(ctx, &mcp.ListPromptsRequest{})
	if err != nil {
		return nil, &errors.RpcError{Code: -32000, Message: err.Error()}
	}
	return res, nil
}
