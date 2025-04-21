package ai

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func (agent *Agent) execute(
	ctx context.Context, tool *types.MCPClient, args map[string]any,
) (string, error) {
	c, err := client.NewStdioMCPClient(
		"/Users/theapemachine/go/src/github.com/theapemachine/a2a-go/a2a-go",
		[]string{},
		"serve",
		"mcp",
		"--port",
		"3000",
	)

	if err != nil {
		log.Error("Failed to create client: %v", err)
	}
	defer c.Close()

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "example-client",
		Version: "1.0.0",
	}

	if _, err = c.Initialize(ctx, initRequest); err != nil {
		return err.Error(), err
	}

	toolRequest := mcp.CallToolRequest{}
	toolRequest.Params.Name = tool.Toolcall.Params.Name
	toolRequest.Params.Arguments = tool.Toolcall.Params.Arguments

	result, err := c.CallTool(ctx, toolRequest)

	if err != nil {
		return err.Error(), err
	}

	return result.Content[0].(mcp.TextContent).Text, nil
}
