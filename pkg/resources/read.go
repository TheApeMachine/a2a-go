package resources

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	errors "github.com/theapemachine/a2a-go/pkg/errors"
)

func Read(
	ctx context.Context,
	raw json.RawMessage,
	handler *MCPHandler,
) (any, *errors.RpcError) {
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.ErrInvalidParams
	}
	req := &mcp.ReadResourceRequest{}
	req.Params.URI = p.URI
	res, err := handler.HandleReadResource(ctx, req)
	if err != nil {
		return nil, &errors.RpcError{Code: -32000, Message: err.Error()}
	}
	return res, nil
}
