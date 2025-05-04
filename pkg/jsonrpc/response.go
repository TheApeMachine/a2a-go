package jsonrpc

type Response struct {
	Message
	// Result is the result of the method invocation. Required on success.
	// Should be null or omitted if an error occurred.
	Result interface{} `json:"result,omitempty"`
	// Error is an error object if an error occurred during the request.
	// Required on failure. Should be null or omitted if the request was successful.
	Error *Error `json:"error,omitempty"`
}
