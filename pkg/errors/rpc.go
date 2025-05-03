package errors

import (
	"fmt"
	"time"
)

/*
RpcError represents a JSON-RPC error response.
*/
type RpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

/*
Error implements the error interface for RpcError.
*/
func (e *RpcError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
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

// RetryConfig holds configuration for retry behavior.
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Second,
		MaxDelay:      time.Minute,
		BackoffFactor: 2.0,
	}
}

// RetryWithBackoff executes a function with exponential backoff retry logic.
func RetryWithBackoff(config *RetryConfig, fn func() error) error {
	var err error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		if attempt == config.MaxAttempts-1 {
			break
		}

		time.Sleep(delay)
		delay = time.Duration(float64(delay) * config.BackoffFactor)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("after %d attempts, last error: %w", config.MaxAttempts, err)
}
