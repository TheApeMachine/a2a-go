package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/log"
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
		mcp.WithDescription("Delegate a task to another agent. Use the 'catalog' tool to discover available agents and their URLs first."),
		mcp.WithString("agent", mcp.Description("Full URL of the target agent to delegate the task to (e.g., http://manager:3210)."), mcp.Required()),
		mcp.WithString("message", mcp.Description("The task message content to send to the target agent."), mcp.Required()),
	)
	return &tool
}

func (bt *DelegateTool) RegisterDelegateTools(srv *server.MCPServer) {
	srv.AddTool(*bt.tool, bt.Handle)
}

func (bt *DelegateTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("DelegateTool: Received call", "arguments", req.Params.Arguments)

	var p delegateParams
	agentURLInterface, agentURLOk := req.Params.Arguments["agent"]
	taskMessageInterface, taskMessageOk := req.Params.Arguments["message"]

	if !agentURLOk {
		log.Warn("DelegateTool: 'agent' argument missing")
		return mcp.NewToolResultError("The 'agent' argument (target agent URL) is missing. Please use the 'catalog' tool to discover available agents and their URLs."), nil
	}
	agentURL, agentURLIsString := agentURLInterface.(string)
	if !agentURLIsString || agentURL == "" {
		log.Warn("DelegateTool: 'agent' argument is not a valid string or is empty", "value", agentURLInterface)
		return mcp.NewToolResultError("The 'agent' argument must be a non-empty string representing the target agent's URL. Please use the 'catalog' tool to discover available agents and their URLs."), nil
	}

	_, err := url.ParseRequestURI(agentURL)
	if err != nil {
		log.Warn("DelegateTool: 'agent' argument is not a valid URL", "url", agentURL, "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("The provided agent URL '%s' is invalid. Please provide a full, valid URL (e.g., http://manager:3210). Use the 'catalog' tool to find correct agent URLs.", agentURL)), nil
	}

	if !taskMessageOk {
		log.Warn("DelegateTool: 'message' argument missing")
		return mcp.NewToolResultError("The 'message' argument (task message content) is missing."), nil
	}
	taskMessage, taskMessageIsString := taskMessageInterface.(string)
	if !taskMessageIsString {
		log.Warn("DelegateTool: 'message' argument is not a string", "value", taskMessageInterface)
		return mcp.NewToolResultError("The 'message' argument must be a string."), nil
	}

	p.Agent = agentURL
	p.Message = taskMessage

	log.Info("DelegateTool: Parsed parameters", "agentURL", p.Agent, "taskMessageLength", len(p.Message))

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
		log.Error("DelegateTool: Failed to marshal payload", "error", err)
		return mcp.NewToolResultError("internal error: failed to marshal payload: " + err.Error()), nil
	}

	rpcURL := p.Agent
	if !strings.HasSuffix(rpcURL, "/rpc") {
		if strings.HasSuffix(rpcURL, "/") {
			rpcURL += "rpc"
		} else {
			rpcURL += "/rpc"
		}
	}
	log.Info("DelegateTool: Sending HTTP POST request", "url", rpcURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewBuffer(buf))
	if err != nil {
		log.Error("DelegateTool: Failed to create http request", "url", rpcURL, "error", err)
		return mcp.NewToolResultError("internal error: failed to create http request: " + err.Error()), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Error("DelegateTool: HTTP request failed", "url", rpcURL, "error", err)
		return mcp.NewToolResultError(fmt.Sprintf("HTTP request failed for agent URL '%s': %s. Ensure the agent URL is correct and reachable. Use the 'catalog' tool if unsure.", p.Agent, err.Error())), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("DelegateTool: Failed to read response body", "url", rpcURL, "error", err)
		return mcp.NewToolResultError("internal error: failed to read response body: " + err.Error()), nil
	}

	log.Info("DelegateTool: Received response", "url", rpcURL, "status", resp.StatusCode, "bodyLength", len(body))

	var rpcResp jsonrpc.Response
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		log.Warn("DelegateTool: Failed to unmarshal JSON-RPC response", "url", rpcURL, "body", string(body), "error", err)
		if resp.StatusCode != http.StatusOK {
			return mcp.NewToolResultError(fmt.Sprintf("Request to agent %s failed with status %d. Response: %s", p.Agent, resp.StatusCode, string(body))), nil
		}
		return mcp.NewToolResultError("internal error: failed to unmarshal JSON-RPC response: " + err.Error()), nil
	}

	if rpcResp.Error != nil {
		log.Warn("DelegateTool: Remote agent returned JSON-RPC error", "url", rpcURL, "errorCode", rpcResp.Error.Code, "errorMessage", rpcResp.Error.Message)
		return mcp.NewToolResultError(fmt.Sprintf("Error from target agent (%s): %s (Code: %d)", p.Agent, rpcResp.Error.Message, rpcResp.Error.Code)), nil
	}

	if rpcResp.Result == nil {
		log.Info("DelegateTool: Remote agent call successful with nil result.", "url", rpcURL)
		return mcp.NewToolResultText(fmt.Sprintf("Task successfully delegated to agent %s. The agent did not return specific data for this delegation call.", p.Agent)), nil
	}

	data, err := json.Marshal(rpcResp.Result)
	if err != nil {
		log.Error("DelegateTool: Failed to marshal rpcResp.Result", "url", rpcURL, "result", rpcResp.Result, "error", err)
		return mcp.NewToolResultError("internal error: failed to marshal result from target agent: " + err.Error()), nil
	}

	log.Info("DelegateTool: Successfully delegated task and received result.", "url", rpcURL, "resultLength", len(data))
	return mcp.NewToolResultText(string(data)), nil
}

// delegateParams defines the input for the delegate tool.
type delegateParams struct {
	Agent   string `json:"agent"`
	Message string `json:"message"`
}
