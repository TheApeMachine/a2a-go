package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/viper"
)

func Acquire(id string) (*mcp.Tool, error) {
	log.Info("initializing MCP client")

	switch id {
	case "agent":
		return NewAgentTool(), nil
	case "development", "docker":
		return NewDockerTool(), nil
	case "web-browsing", "browser":
		return NewBrowserTool(), nil
	case "catalog":
		return NewCatalogTool(), nil
	case "management":
		return NewDelegateTool(), nil
	case "azure_get_sprints":
		return NewAzureGetSprintsTool(), nil
	case "azure_create_sprint":
		return NewAzureCreateSprintTool(), nil
	case "azure_sprint_items":
		return NewAzureSprintItemsTool(), nil
	case "azure_sprint_overview":
		return NewAzureSprintOverviewTool(), nil
	case "azure_get_work_items":
		return NewAzureGetWorkItemsTool(), nil
	case "azure_create_work_items":
		return NewAzureCreateWorkItemsTool(), nil
	case "azure_update_work_items":
		return NewAzureUpdateWorkItemsTool(), nil
	case "azure_execute_wiql":
		return NewAzureExecuteWiqlTool(), nil
	case "azure_search_work_items":
		return NewAzureSearchWorkItemsTool(), nil
	case "azure_enrich_work_item":
		return NewAzureEnrichWorkItemTool(), nil
	case "azure_get_github_file_content":
		return NewAzureGetGithubFileContentTool(), nil
	case "azure_work_item_comments":
		return NewAzureWorkItemCommentsTool(), nil
	}

	return nil, fmt.Errorf("tool not found: %s", id)
}

func NewExecutor(
	ctx context.Context, name, args string,
) (string, error) {
	endpointKey := "endpoints." + name + "tool"

	url := viper.GetViper().GetString(endpointKey)
	if url == "" {
		log.Error("endpoint URL not found in config", "key", endpointKey, "toolName", name)
		return "", fmt.Errorf("configuration error: endpoint URL for %s (key: %s) not found", name, endpointKey)
	}

	// All MCP tools connect to an /sse endpoint on their respective MCP server URL
	sseTransport, err := transport.NewSSE(url + "/sse")

	if err != nil {
		log.Error("failed to create SSE transport", "error", err, "url", url)
		return "", fmt.Errorf("failed to create SSE transport: %w", err)
	}

	if err := sseTransport.Start(ctx); err != nil {
		log.Error("failed to start SSE transport", "error", err, "url", url)
		return "", fmt.Errorf("failed to start SSE transport: %w", err)
	}

	c := client.NewClient(sseTransport)
	defer c.Close()

	c.OnNotification(func(notification mcp.JSONRPCNotification) {
		log.Info("received notification", "method", notification.Method)
	})

	log.Info("initializing MCP client")
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "MCP-Go Simple Client Example",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	serverInfo, err := c.Initialize(ctx, initRequest)
	if err != nil {
		log.Error("Failed to initialize", "error", err)
		return "", fmt.Errorf("Failed to initialize: %w", err)
	}

	log.Info("connected to server", "serverName", serverInfo.ServerInfo.Name, "serverVersion", serverInfo.ServerInfo.Version, "serverCapabilities", serverInfo.Capabilities)

	arguments := map[string]any{}
	if err := json.Unmarshal([]byte(args), &arguments); err != nil {
		c.Close()
		return "", fmt.Errorf("failed to unmarshal tool arguments '%s': %w", args, err)
	}

	log.Info("calling tool", "toolName", name, "args", arguments)
	callToolRequest := mcp.CallToolRequest{}
	callToolRequest.Params.Name = name
	callToolRequest.Params.Arguments = arguments

	callToolResult, err := c.CallTool(ctx, callToolRequest)
	if err != nil {
		c.Close()
		log.Error("failed to call tool", "error", err, "tool", name)
		return "", fmt.Errorf("failed to call tool %s: %w", name, err)
	}

	log.Info("tool executed successfully", "toolName", name, "result", callToolResult)

	var resultString string
	if len(callToolResult.Content) > 0 {
		firstContent := callToolResult.Content[0]
		if textContent, ok := firstContent.(mcp.TextContent); ok {
			resultString = textContent.Text
		} else {
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

	log.Info("client shutting down after tool call")
	return resultString, nil
}
