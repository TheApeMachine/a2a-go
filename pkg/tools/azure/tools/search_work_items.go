package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// SearchWorkItemOutput defines the structure for a single work item in search results.
type SearchWorkItemOutput struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	State string `json:"state"`
	URL   string `json:"url"`
}

// AzureSearchWorkItemsTool provides functionality to search for work items.
type AzureSearchWorkItemsTool struct {
	handle mcp.Tool
	client workitemtracking.Client
	config AzureDevOpsConfig
}

// NewAzureSearchWorkItemsTool creates a new tool instance for searching work items.
func NewAzureSearchWorkItemsTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	client, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating workitemtracking client for AzureSearchWorkItemsTool: %v\n", err)
		return nil
	}

	tool := &AzureSearchWorkItemsTool{
		client: client,
		config: config,
	}

	tool.handle = mcp.NewTool(
		"azure_search_work_items",
		mcp.WithDescription("Search for work items in Azure DevOps by keywords, with optional type and state filters."),
		mcp.WithString("search_term", mcp.Required(), mcp.Description("The keyword or phrase to search for in work item titles, descriptions, and tags.")),
		mcp.WithString("work_item_types", mcp.Description("Optional. Comma-separated list of work item types to filter by (e.g., 'User Story,Bug').")),
		mcp.WithString("states", mcp.Description("Optional. Comma-separated list of states to filter by (e.g., 'Active,Resolved', 'New').")),
		mcp.WithString("format", mcp.Description("Response format: 'text' (default) or 'json'."), mcp.Enum("text", "json")),
		mcp.WithString("limit", mcp.Description("Optional. Maximum number of items to return (default: 50).")),
	)
	return tool
}

func (tool *AzureSearchWorkItemsTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *AzureSearchWorkItemsTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	searchTerm, err := GetStringArg(request, "search_term")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: search_term"), nil
	}
	workItemTypesStr, _ := GetStringArg(request, "work_item_types")
	statesStr, _ := GetStringArg(request, "states")
	format, _ := GetStringArg(request, "format")
	limitStr, _ := GetStringArg(request, "limit")

	limit := 50 // Default limit
	if limitStr != "" {
		if l, convErr := strconv.Atoi(limitStr); convErr == nil && l > 0 {
			limit = l
		}
	}

	var conditions []string
	// Search term conditions
	if searchTerm != "" {
		// Escape single quotes in search term for WIQL
		escapedSearchTerm := strings.ReplaceAll(searchTerm, "'", "''")
		conditions = append(conditions, fmt.Sprintf("([System.Title] CONTAINS '%s' OR [System.Description] CONTAINS '%s' OR [System.Tags] CONTAINS '%s')", escapedSearchTerm, escapedSearchTerm, escapedSearchTerm))
	}

	// Work item type conditions
	if workItemTypesStr != "" {
		types := strings.Split(workItemTypesStr, ",")
		var typeConditions []string
		for _, t := range types {
			cleanType := strings.TrimSpace(t)
			if cleanType != "" {
				// Escape single quotes in type for WIQL
				escapedType := strings.ReplaceAll(cleanType, "'", "''")
				typeConditions = append(typeConditions, fmt.Sprintf("'%s'", escapedType))
			}
		}
		if len(typeConditions) > 0 {
			conditions = append(conditions, fmt.Sprintf("[System.WorkItemType] IN (%s)", strings.Join(typeConditions, ",")))
		}
	}

	// State conditions
	if statesStr != "" {
		states := strings.Split(statesStr, ",")
		var stateConditions []string
		for _, s := range states {
			cleanState := strings.TrimSpace(s)
			if cleanState != "" {
				// Escape single quotes in state for WIQL
				escapedState := strings.ReplaceAll(cleanState, "'", "''")
				stateConditions = append(stateConditions, fmt.Sprintf("'%s'", escapedState))
			}
		}
		if len(stateConditions) > 0 {
			conditions = append(conditions, fmt.Sprintf("[System.State] IN (%s)", strings.Join(stateConditions, ",")))
		}
	}

	if len(conditions) == 0 {
		return mcp.NewToolResultError("Search term is required to perform a search."), nil
	}

	// Always exclude removed items
	conditions = append(conditions, "[System.State] <> 'Removed'")

	query := fmt.Sprintf("SELECT [System.Id], [System.Title], [System.WorkItemType], [System.State] FROM WorkItems WHERE %s ORDER BY [System.ChangedDate] DESC", strings.Join(conditions, " AND "))

	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql:    &workitemtracking.Wiql{Query: &query},
		Project: &tool.config.Project,
		Top:     &limit, // Apply limit
	}

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		return HandleError(err, "Failed to execute search query"), nil
	}

	var searchResults []SearchWorkItemOutput
	if queryResult.WorkItems != nil && len(*queryResult.WorkItems) > 0 {
		var itemIDs []int
		for _, wiRef := range *queryResult.WorkItems {
			if wiRef.Id != nil {
				itemIDs = append(itemIDs, *wiRef.Id)
			}
		}

		// Fetch details for the found items to get all necessary fields for output
		// We use GetWorkItems which we know works for batches
		if len(itemIDs) > 0 {
			args := workitemtracking.GetWorkItemsArgs{
				Ids:     &itemIDs,
				Project: &tool.config.Project,
				Fields:  &[]string{"System.Id", "System.Title", "System.WorkItemType", "System.State"},
				Expand:  &workitemtracking.WorkItemExpandValues.None,
			}
			workItems, err := tool.client.GetWorkItems(ctx, args)
			if err != nil {
				return HandleError(err, "Failed to get details for searched work items"), nil
			}

			if workItems != nil {
				for _, item := range *workItems {
					if item.Id == nil || item.Fields == nil {
						continue
					}
					fields := *item.Fields
					output := SearchWorkItemOutput{
						ID:  *item.Id,
						URL: GetWorkItemURL(tool.config.OrganizationURL, *item.Id),
					}
					if title, ok := fields["System.Title"].(string); ok {
						output.Title = title
					}
					if wiType, ok := fields["System.WorkItemType"].(string); ok {
						output.Type = wiType
					}
					if state, ok := fields["System.State"].(string); ok {
						output.State = state
					}
					searchResults = append(searchResults, output)
				}
			}
		}
	}

	if strings.ToLower(format) == "json" {
		if len(searchResults) == 0 {
			return mcp.NewToolResultText("[]"), nil // Empty array for JSON
		}
		jsonBytes, err := json.MarshalIndent(searchResults, "", "  ")
		if err != nil {
			return HandleError(err, "Failed to serialize search results to JSON"), nil
		}
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}

	// Text Output
	if len(searchResults) == 0 {
		return mcp.NewToolResultText("No work items found matching your search criteria."), nil
	}
	var textOutputLines []string
	textOutputLines = append(textOutputLines, fmt.Sprintf("Found %d work item(s) matching '%s':", len(searchResults), searchTerm))
	for _, item := range searchResults {
		textOutputLines = append(textOutputLines, fmt.Sprintf("- [%d] %s (Type: %s, State: %s) URL: %s", item.ID, item.Title, item.Type, item.State, item.URL))
	}
	return mcp.NewToolResultText(strings.Join(textOutputLines, "\n")), nil
}

// GetWorkItemURL, HandleError, GetStringArg, etc., are assumed to be in common.go or accessible.
