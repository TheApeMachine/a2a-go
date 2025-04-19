package service

// A very small, self‑contained JSON‑RPC 2.0 helper.  It is not a full‑fledged
// framework – the goal is to keep the amount of required code minimal yet be
// sufficient for typical agent ↔ agent interactions.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
)

// --------------------------- Wire Types ------------------------------------

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // accepts string | number | null
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// RPCError is an exported alias so that external packages (e.g. examples) can
// satisfy the HandlerFunc signature without accessing the unexported rpcError
// identifier directly.  It intentionally re‑exports the same underlying type
// without widening the public API surface.
type RPCError = rpcError

// Convenience errors (JSON‑RPC reserved codes  -32600 .. -32000)
var (
	errParseError     = &rpcError{Code: -32700, Message: "Parse error"}
	errInvalidRequest = &rpcError{Code: -32600, Message: "Invalid Request"}
	errMethodNotFound = &rpcError{Code: -32601, Message: "Method not found"}
	errInvalidParams  = &rpcError{Code: -32602, Message: "Invalid params"}
	errInternal       = &rpcError{Code: -32603, Message: "Internal error"}
)

// --------------------------- Server  ---------------------------------------

// HandlerFunc processes the raw params field and returns a result or a *rpcError.
// Returning (nil, nil) is treated as null‑result (i.e. {"result":null}).
type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, *rpcError)

// RPCServer multiplexes JSON‑RPC method names to handler functions.
type RPCServer struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

func NewRPCServer() *RPCServer {
	return &RPCServer{
		handlers: make(map[string]HandlerFunc),
	}
}

func (s *RPCServer) Register(method string, h HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[method] = h
}

func (s *RPCServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST supported", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, nil, errParseError)
		return
	}

	// Support batch requests if the first byte is '['
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		respondError(w, nil, errInvalidRequest)
		return
	}

	if body[0] == '[' {
		var batch []RPCRequest
		if err := json.Unmarshal(body, &batch); err != nil {
			respondError(w, nil, errParseError)
			return
		}
		var responses []RPCResponse
		for _, req := range batch {
			resp := s.handle(r.Context(), &req)
			// Notifications have no ID – skip sending a response.
			if len(req.ID) != 0 {
				responses = append(responses, resp)
			}
		}
		if len(responses) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(responses)
		return
	}

	var req RPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respondError(w, nil, errParseError)
		return
	}

	resp := s.handle(r.Context(), &req)
	// Notification – no ID → no response.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *RPCServer) handle(ctx context.Context, req *RPCRequest) RPCResponse {
	if req.JSONRPC != "2.0" {
		return newErrorResponse(req.ID, errInvalidRequest)
	}

	s.mu.RLock()
	h, ok := s.handlers[req.Method]
	s.mu.RUnlock()
	if !ok {
		return newErrorResponse(req.ID, errMethodNotFound)
	}

	result, rpcErr := h(ctx, req.Params)
	if rpcErr != nil {
		return newErrorResponse(req.ID, rpcErr)
	}

	return RPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func newErrorResponse(id json.RawMessage, e *rpcError) RPCResponse {
	// Ensure mandatory Code/Message.
	if e == nil {
		e = errInternal
	}
	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   e,
	}
}

func respondError(w http.ResponseWriter, id json.RawMessage, e *rpcError) {
	_ = json.NewEncoder(w).Encode(newErrorResponse(id, e))
}

// --------------------------- Client  ---------------------------------------

// RPCClient is a minimal wrapper around http.Client to perform JSON‑RPC calls.
type RPCClient struct {
	Endpoint string
	HTTP     *http.Client
}

func (c *RPCClient) Call(ctx context.Context, method string, params any, result any) error {
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}

	reqID := 1 // for simplicity – caller may wrap RPCClient to customise

	payload := RPCRequest{
		JSONRPC: "2.0",
		ID:      mustMarshalID(reqID),
		Method:  method,
	}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return err
		}
		payload.Params = b
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return errors.New(rpcResp.Error.Message)
	}
	if result != nil {
		// Marshal the "result" field back into user‑provided struct.
		b, err := json.Marshal(rpcResp.Result)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, result); err != nil {
			return err
		}
	}
	return nil
}

func mustMarshalID(v int) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
