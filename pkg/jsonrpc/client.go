package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type RPCClient struct {
	Endpoint string
	HTTP     *http.Client
}

func NewRPCClient(endpoint string) *RPCClient {
	return &RPCClient{
		Endpoint: endpoint,
		HTTP:     http.DefaultClient,
	}
}

func (c *RPCClient) Call(
	ctx context.Context,
	method string,
	params any,
	result any,
) error {
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
