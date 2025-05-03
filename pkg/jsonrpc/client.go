package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/theapemachine/a2a-go/pkg/auth"
)

type RPCClient struct {
	URL         string
	Client      *http.Client
	AuthService *auth.Service
}

func NewRPCClient(url string) *RPCClient {
	return &RPCClient{
		URL:    url,
		Client: &http.Client{},
	}
}

func (c *RPCClient) Call(
	ctx context.Context,
	method string,
	params any,
	result any,
) error {
	if c.Client == nil {
		c.Client = http.DefaultClient
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))

	if err != nil {
		return err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Add authentication if service is available
	if c.AuthService != nil {
		if err := c.AuthService.AuthenticateRequest(httpReq); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	resp, err := c.Client.Do(httpReq)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Handle authentication errors
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized: invalid or expired token")
	}
	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("forbidden: insufficient permissions")
	}

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
