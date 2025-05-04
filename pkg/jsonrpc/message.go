package jsonrpc

// JSONRPCMessageIdentifier represents the base interface for identifying JSON-RPC messages
type MessageIdentifier struct {
	// ID is the request identifier. Can be a string, number, or null.
	// Responses must have the same ID as the request they relate to.
	// Notifications (requests without an expected response) should omit the ID or use null.
	ID interface{} `json:"id,omitempty"`
}

// JSONRPCMessage represents the base interface for all JSON-RPC messages
type Message struct {
	MessageIdentifier
	// JSONRPC specifies the JSON-RPC version. Must be "2.0"
	JSONRPC string `json:"jsonrpc,omitempty"`
}

// JSONRPCError represents a JSON-RPC error object
type Error struct {
	// Code is a number indicating the error type that occurred
	Code int `json:"code"`
	// Message is a string providing a short description of the error
	Message string `json:"message"`
	// Data is optional additional data about the error
	Data interface{} `json:"data,omitempty"`
}
