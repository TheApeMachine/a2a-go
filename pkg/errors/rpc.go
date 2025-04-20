package errors

type RpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Convenience errors (JSON‑RPC reserved codes  -32600 .. -32000)
var (
	ErrParseError     = &RpcError{Code: -32700, Message: "Parse error"}
	ErrInvalidRequest = &RpcError{Code: -32600, Message: "Invalid Request"}
	ErrMethodNotFound = &RpcError{Code: -32601, Message: "Method not found"}
	ErrInvalidParams  = &RpcError{Code: -32602, Message: "Invalid params"}
	ErrInternal       = &RpcError{Code: -32603, Message: "Internal error"}
)
