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
	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
)

type DelegateTool struct {
	tool *mcp.Tool
}

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

func (bt *DelegateTool) RegisterDelegateTools(srv *server.MCPServer) {
	srv.AddTool(*bt.tool, bt.Handle)
}

func (bt *DelegateTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	var p delegateParams
	agentURL, ok := req.Params.Arguments["agent"].(string)
	if !ok {
		return mcp.NewToolResultError("agent argument is missing or not a string"), nil
	}
	taskMessage, ok := req.Params.Arguments["message"].(string)
	if !ok {
		return mcp.NewToolResultError("message argument is missing or not a string"), nil
	}

	p.Agent = agentURL
	p.Message = taskMessage

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
		return mcp.NewToolResultError("failed to marshal payload: " + err.Error()), nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Agent+"/rpc", bytes.NewBuffer(buf))
	if err != nil {
		return mcp.NewToolResultError("failed to create http request: " + err.Error()), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return mcp.NewToolResultError("http request failed: " + err.Error()), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.NewToolResultError("failed to read response body: " + err.Error()), nil
	}

	var rpcResp jsonrpc.Response
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return mcp.NewToolResultError("failed to unmarshal rpc response: " + err.Error()), nil
	}
	if rpcResp.Error != nil {
		return mcp.NewToolResultError(fmt.Sprintf("remote error: %s", rpcResp.Error.Message)), nil
	}

	data, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return mcp.NewToolResultError("failed to marshal rpc result: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// delegateParams defines the input for the delegate tool.
type delegateParams struct {
	Agent   string `json:"agent"`
	Message string `json:"message"`
}
