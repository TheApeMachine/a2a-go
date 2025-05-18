package tools

import (
    "context"
    "encoding/json"
    
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/theapemachine/a2a-go/pkg/catalog"
    "github.com/spf13/viper"
)

// NewCatalogTool returns a tool definition that lists available agents.
func NewCatalogTool() *mcp.Tool {
    tool := mcp.NewTool(
        "catalog",
        mcp.WithDescription("List available agents from the catalog"),
    )
    return &tool
}

func executeCatalog(ctx context.Context) (string, error) {
    url := viper.GetViper().GetString("endpoints.catalog")
    client := catalog.NewCatalogClient(url)
    agents, err := client.GetAgents()
    if err != nil {
        return "", err
    }
    data, err := json.Marshal(agents)
    if err != nil {
        return "", err
    }
    return string(data), nil
}
