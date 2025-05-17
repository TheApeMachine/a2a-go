package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const MCP_SESSION_ID_HEADER = "Mcp-Session-Id"

func Aquire(id string) (*mcp.Tool, error) {
	log.Info("aquiring tool", "id", id)

	switch id {
	case "development":
		return NewDockerTool(), nil
	case "web-browsing":
		return NewBrowserTool(), nil
	}

	return nil, fmt.Errorf("tool not found: %s", id)
}

func NewExecutor(
	ctx context.Context, name, args string, sessionID string,
) (string, error) {
	sseTransport, err := transport.NewSSE("http://" + name + "tool:3210/sse")

	if err != nil {
		log.Error("failed to create SSE transport", "error", err)
		return "", fmt.Errorf("failed to create SSE transport: %w", err)
	}

	if err := sseTransport.Start(ctx); err != nil {
		log.Error("failed to start SSE transport", "error", err)
		return "", fmt.Errorf("failed to start SSE transport: %w", err)
	}

	c := client.NewClient(sseTransport)

	c.OnNotification(func(notification mcp.JSONRPCNotification) {
		fmt.Printf("Received notification: %s\n", notification.Method)
	})

	fmt.Println("Initializing client...")
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "MCP-Go Simple Client Example",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	serverInfo, err := c.Initialize(ctx, initRequest)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// Display server information
	fmt.Printf("Connected to server: %s (version %s)\n",
		serverInfo.ServerInfo.Name,
		serverInfo.ServerInfo.Version)
	fmt.Printf("Server capabilities: %+v\n", serverInfo.Capabilities)

	// Prepare arguments for CallTool
	arguments := map[string]any{}
	if err := json.Unmarshal([]byte(args), &arguments); err != nil {
		c.Close()
		return "", fmt.Errorf("failed to unmarshal tool arguments '%s': %w", args, err)
	}

	// Perform the tool call
	fmt.Printf("Calling tool '%s' with args: %v\n", name, arguments)
	callToolRequest := mcp.CallToolRequest{}
	callToolRequest.Params.Name = name // This 'name' is the actual tool name like "browser"
	callToolRequest.Params.Arguments = arguments
	// We are not sending any Meta for now, so no need to set callToolRequest.Params.Meta

	callToolResult, err := c.CallTool(ctx, callToolRequest)
	if err != nil {
		c.Close()
		log.Error("failed to call tool", "tool", name, "error", err)
		return "", fmt.Errorf("failed to call tool %s: %w", name, err)
	}

	fmt.Printf("Tool '%s' executed successfully. Result: %+v\n", name, callToolResult)

	// Process the result
	// For now, we'll take the text from the first TextContent item.
	// This might need to be more sophisticated depending on expected tool outputs.
	var resultString string
	if len(callToolResult.Content) > 0 {
		firstContent := callToolResult.Content[0]
		if textContent, ok := firstContent.(mcp.TextContent); ok {
			resultString = textContent.Text
		} else {
			// If not text, marshal the first content item to JSON as a fallback
			jsonResult, err := json.Marshal(firstContent)
			if err != nil {
				log.Warn("failed to marshal tool result content", "error", err)
				resultString = "[error marshalling result]"
			} else {
				resultString = string(jsonResult)
			}
		}
	} else {
		resultString = "[empty tool result]"
	}

	fmt.Println("Client shutting down after tool call...")
	c.Close()

	return resultString, nil
}
