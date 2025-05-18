package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/tools/catalog"
)

// CatalogTool holds the MCP tool definition.
type CatalogTool struct {
	tool *mcp.Tool
}

// NewCatalogTool returns a new MCP tool for listing available agents.
// It initializes the tool definition.
func NewCatalogTool() *mcp.Tool {
	tool := mcp.NewTool(
		"catalog",
		mcp.WithDescription("List available agents from the catalog"),
	)
	return &tool
}

// RegisterCatalogTool adds the catalog tool to the MCP server.
func (ct *CatalogTool) RegisterCatalogTool(srv *server.MCPServer) {
	srv.AddTool(*ct.tool, ct.Handle)
}

// Handle executes the catalog tool logic.
func (ct *CatalogTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	url := viper.GetViper().GetString("endpoints.catalog")
	client := catalog.NewCatalogClient(url)
	agents, err := client.GetAgents()
	if err != nil {
		return mcp.NewToolResultError("failed to get agents from catalog: " + err.Error()), nil
	}

	jsonBytes, err := json.Marshal(agents)
	if err != nil {
		return mcp.NewToolResultError("failed to marshal agents to JSON: " + err.Error()), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}
