package prompts

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	errors "github.com/theapemachine/a2a-go/pkg/errors"
)

func Get(
	ctx context.Context,
	raw json.RawMessage,
	handler *MCPHandler,
) (any, *errors.RpcError) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.ErrInvalidParams
	}
	req := &mcp.GetPromptRequest{}
	req.Params.Name = p.Name
	res, err := handler.HandleGetPrompt(ctx, req)
	if err != nil {
		return nil, &errors.RpcError{Code: -32000, Message: err.Error()}
	}
	return res, nil
}
