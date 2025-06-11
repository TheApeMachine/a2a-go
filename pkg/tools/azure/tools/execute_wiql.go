package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// AzureExecuteWiqlTool provides functionality to execute WIQL queries.
type AzureExecuteWiqlTool struct {
	handle mcp.Tool
	client workitemtracking.Client
	config AzureDevOpsConfig
}

// NewAzureExecuteWiqlTool creates a new tool instance for executing WIQL queries.
func NewAzureExecuteWiqlTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	client, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		return nil
	}

	tool := &AzureExecuteWiqlTool{
		client: client,
		config: config,
	}

	tool.handle = mcp.NewTool(
		"azure_execute_wiql",
		mcp.WithDescription("Execute a WIQL query on Azure DevOps, returning the results."),
		mcp.WithString(
			"query",
			mcp.Required(),
			mcp.Description("WIQL query string for searching work items."),
		),
		// Add other relevant parameters if needed, e.g., for paging or specific formatting
	)
	return tool
}

func (tool *AzureExecuteWiqlTool) Handle() mcp.Tool {
	return tool.handle
}

// Handler executes the WIQL query.
func (tool *AzureExecuteWiqlTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := GetStringArg(request, "query")
	if err != nil {
		exampleQuery := "SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'DOING'"
		return mcp.NewToolResultError(fmt.Sprintf(`
Missing "query" parameter. Please provide a valid WIQL query string.

Example query: %s

For more examples, use the "get_examples" operation or "get_help" with parameter "operation": "query"
`, exampleQuery)), nil
	}

	// Validate that query contains basic WIQL elements
	if !strings.Contains(strings.ToUpper(query), "SELECT") || !strings.Contains(strings.ToUpper(query), "FROM") {
		return mcp.NewToolResultError(fmt.Sprintf(`
Invalid WIQL query format. Query must contain SELECT and FROM clauses.

Your query: %s

Example of valid query: SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'DOING'

For more examples, use the "get_examples" operation.
`, query)), nil
	}

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
		Project: &tool.config.Project, // Assuming project is part of config
		Team:    &tool.config.Team,    // Assuming team is part of config, if @CurrentIteration is used
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		// Try to provide a more helpful error message with suggestions
		errorMsg := err.Error()
		suggestion := ""

		if strings.Contains(errorMsg, "TF51005") {
			suggestion = "Check that field names are enclosed in square brackets, e.g. [System.State]"
		} else if strings.Contains(errorMsg, "TF51004") {
			suggestion = "Check for syntax errors in your query. Make sure field names and values are correctly formatted."
		}

		errorResponse := fmt.Sprintf("Failed to query work items: %v\n\nYour query: %s", err, query)
		if suggestion != "" {
			errorResponse += "\n\nSuggestion: " + suggestion
		}
		// errorResponse += "\n\nFor example queries, use the 'get_examples' operation."

		return mcp.NewToolResultError(errorResponse), nil
	}

	// Marshal the entire queryResult object to JSON
	jsonResult, err := json.MarshalIndent(queryResult, "", "  ")
	if err != nil {
		return HandleError(err, "Failed to serialize WIQL query result to JSON"), nil
	}

	return mcp.NewToolResultText(string(jsonResult)), nil
}

// getStringArg and handleError would be in common.go
