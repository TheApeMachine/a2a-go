package errors

import (
	"fmt"
)

type RpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Convenience errors (JSON‑RPC reserved codes  -32600 .. -32000)
// Application specific codes should use other ranges.
var (
	ErrParseError     = &RpcError{Code: -32700, Message: "Parse error"}
	ErrInvalidRequest = &RpcError{Code: -32600, Message: "Invalid Request"}
	ErrMethodNotFound = &RpcError{Code: -32601, Message: "Method not found"}
	ErrInvalidParams  = &RpcError{Code: -32602, Message: "Invalid params"}
	ErrInternal       = &RpcError{Code: -32603, Message: "Internal error"}

	// A2A Specific Errors (Example range: -32000 to -32099)
	ErrTaskNotFound                   = &RpcError{Code: -32000, Message: "Task not found"}
	ErrTaskCancelled                  = &RpcError{Code: -32001, Message: "Task was cancelled"}
	ErrTaskCreationFailed             = &RpcError{Code: -32002, Message: "Task creation failed"}
	ErrPushNotificationConfigNotFound = &RpcError{Code: -32010, Message: "Push notification config not found"}
	ErrNotImplemented                 = &RpcError{Code: -32099, Message: "Method not implemented"}
)

// WithMessagef creates a *copy* of an RpcError with a formatted message.
// It does not modify the original error variable.
func (e *RpcError) WithMessagef(format string, args ...any) *RpcError {
	// Return a new error instance to avoid modifying the global variables
	newErr := *e // Create a shallow copy
	newErr.Message = fmt.Sprintf(format, args...)
	return &newErr
}
