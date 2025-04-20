package jsonrpc

import (
	"encoding/json"

	"github.com/theapemachine/a2a-go/pkg/errors"
)

type RPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *errors.RpcError `json:"error,omitempty"`
}
