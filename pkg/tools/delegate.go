package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
)

// NewDelegateTool returns a tool definition for delegating a task to another agent.
func NewDelegateTool() *mcp.Tool {
	tool := mcp.NewTool(
		"delegate_task",
		mcp.WithDescription("Delegate a task to another agent"),
		mcp.WithString("agent", mcp.Description("Agent URL"), mcp.Required()),
		mcp.WithString("message", mcp.Description("Task message"), mcp.Required()),
	)
	return &tool
}

// delegateParams defines the input for the delegate tool.
type delegateParams struct {
	Agent   string `json:"agent"`
	Message string `json:"message"`
}

func executeDelegate(ctx context.Context, raw string) (string, error) {
	var p delegateParams
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return "", err
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "tasks/send",
		"params": map[string]any{
			"id": uuid.NewString(),
			"message": map[string]any{
				"role":  "user",
				"parts": []map[string]any{{"type": "text", "text": p.Message}},
			},
		},
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Agent+"/rpc", bytes.NewBuffer(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var rpcResp jsonrpc.Response
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return "", err
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("remote error: %s", rpcResp.Error.Message)
	}

	data, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
