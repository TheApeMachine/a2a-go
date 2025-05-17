package jsonrpc

// JSONRPCRequest represents a JSON-RPC request object base structure
type Request struct {
	Message
	// Method is the name of the method to be invoked
	Method string `json:"method"`
	// Params are the parameters for the method
	Params any `json:"params,omitempty"`
}
