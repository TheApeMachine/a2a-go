package roots

import (
	"context"
	"encoding/json"

	errors "github.com/theapemachine/a2a-go/pkg/errors"
)

func Create(
	ctx context.Context,
	raw json.RawMessage,
	handler *MCPHandler,
) (any, *errors.RpcError) {
	root, err := handler.HandleCreateRoot(ctx, raw)
	if err != nil {
		return nil, &errors.RpcError{Code: -32000, Message: err.Error()}
	}
	return root, nil
}
