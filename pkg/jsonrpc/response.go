package jsonrpc

import (
	"github.com/theapemachine/a2a-go/pkg/errors"
)

type RPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      interface{}      `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *errors.RpcError `json:"error,omitempty"`
}
