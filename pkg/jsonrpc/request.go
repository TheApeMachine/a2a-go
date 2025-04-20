package jsonrpc

import "encoding/json"

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // accepts string | number | null
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
