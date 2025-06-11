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

// AzureFindItemsByStatusTool provides functionality to find work items by status
type AzureFindItemsByStatusTool struct {
	handle mcp.Tool
	client workitemtracking.Client
	config AzureDevOpsConfig
}

// NewAzureFindItemsByStatusTool creates a new tool instance for finding items by status
func NewAzureFindItemsByStatusTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	client, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		return nil
	}

	tool := &AzureFindItemsByStatusTool{
		client: client,
		config: config,
	}

	tool.handle = mcp.NewTool(
		"azure_find_items_by_status",
		mcp.WithDescription("Find work items by status in Azure DevOps"),
		mcp.WithString(
			"states",
			mcp.Required(),
			mcp.Description("Comma-separated list of states to filter by (e.g., 'DOING,REVIEW')"),
			mcp.Enum("TODO", "DOING", "REVIEW", "ACCEPTED", "DONE"),
		),
		mcp.WithString(
			"types", mcp.Description("Optional comma-separated list of work item types to filter by (e.g., 'Task,Bug')"),
		),
		mcp.WithString(
			"has_parent",
			mcp.Description("Filter by parent relationship ('true' or 'false')"),
		),
		mcp.WithString(
			"parent_type",
			mcp.Description("Type of parent to check for (e.g., 'Epic') - used with has_parent"),
			mcp.Enum("Epic", "Task", "Bug", "User Story"),
		),
		mcp.WithString(
			"format",
			mcp.Description("Response format: 'text' (default) or 'json'"),
		),
		mcp.WithString(
			"page_size",
			mcp.Description("Number of items per page (default: 50, max: 200). Use '0' for no limit (fetches all, up to API limits)."),
		),
	)

	return tool
}

func (tool *AzureFindItemsByStatusTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *AzureFindItemsByStatusTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get states parameter (required)
	statesStr, err := GetStringArg(request, "states")
	if err != nil {
		return mcp.NewToolResultError(`
Missing "states" parameter. Please provide a comma-separated list of states to search for.

Example: "states": "DOING,REVIEW"
`), nil
	}

	// Parse states
	states := strings.Split(statesStr, ",")
	for i, state := range states {
		states[i] = strings.TrimSpace(state)
	}

	// Get optional types parameter
	var typesStr string
	var types []string
	if val, ok := request.Params.Arguments["types"].(string); ok && val != "" {
		typesStr = val
		types = strings.Split(typesStr, ",")
		for i, t := range types {
			types[i] = strings.TrimSpace(t)
		}
	}

	// Handle paging
	pageSize := 50 // Default page size

	if pageSizeStr, ok := request.Params.Arguments["page_size"].(string); ok && pageSizeStr != "" {
		if pageSizeInt, err := strconv.Atoi(pageSizeStr); err == nil {
			if pageSizeInt == 0 { // User explicitly wants all results (up to API hard limits)
				pageSize = -1 // Indicate no SDK-side paging, rely on API's max if any
			} else if pageSizeInt > 0 {
				pageSize = pageSizeInt
				if pageSize > 200 { // Cap at Azure DevOps API typical limit for TOP
					pageSize = 200
				}
			}
		}
	}

	// Build state condition for WIQL
	var stateCondition string
	if len(states) == 1 {
		stateCondition = fmt.Sprintf("[System.State] = '%s'", states[0])
	} else {
		stateCondition = fmt.Sprintf("[System.State] IN (%s)", FormatStringList(states))
	}

	// Build type condition for WIQL if types were provided
	var typeCondition string
	if len(types) > 0 {
		if len(types) == 1 {
			typeCondition = fmt.Sprintf(" AND [System.WorkItemType] = '%s'", types[0])
		} else {
			typeCondition = fmt.Sprintf(" AND [System.WorkItemType] IN (%s)", FormatStringList(types))
		}
	}

	// Check if we need to filter by parent relationship
	var parentCondition string
	if hasParentStr, ok := request.Params.Arguments["has_parent"].(string); ok && hasParentStr != "" {
		hasParent := strings.ToLower(hasParentStr) == "true"
		parentType := "Epic" // Default parent type

		if parentTypeStr, ok := request.Params.Arguments["parent_type"].(string); ok && parentTypeStr != "" {
			parentType = parentTypeStr
		}

		if hasParent {
			// Find items WITH parents
			parentCondition = fmt.Sprintf(" AND [System.Id] IN (SELECT [System.Id] FROM WorkItemLinks WHERE [Target].[System.WorkItemType] = '%s' AND [System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Reverse' MODE (MustContain))", parentType)
		} else {
			// Find items WITHOUT parents
			parentCondition = fmt.Sprintf(" AND [System.Id] NOT IN (SELECT [System.Id] FROM WorkItemLinks WHERE [Target].[System.WorkItemType] = '%s' AND [System.Links.LinkType] = 'System.LinkTypes.Hierarchy-Reverse' MODE (MustContain))", parentType)
		}
	}

	// Build complete WIQL query
	query := fmt.Sprintf(`
SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo]
FROM WorkItems
WHERE %s%s%s
ORDER BY [System.ChangedDate] DESC
`, stateCondition, typeCondition, parentCondition)

	// Create WIQL query
	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
		Project: &tool.config.Project,
	}

	// Apply limit using the Top field if pageSize is specified and not -1 (for all)
	if pageSize > 0 {
		top := pageSize
		wiqlArgs.Top = &top
	}
	// If pageSize is -1, wiqlArgs.Top remains nil, fetching without TOP N from SDK side.

	queryResult, err := tool.client.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		return HandleError(err, fmt.Sprintf("Failed to query work items: %v", err)), nil
	}

	if len(*queryResult.WorkItems) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No work items found matching the criteria.\nStates: %s", statesStr)), nil
	}

	// Get IDs of matching work items
	ids := make([]int, len(*queryResult.WorkItems))
	for i, item := range *queryResult.WorkItems {
		ids[i] = *item.Id
	}

	// Get details of the work items
	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})

	if err != nil {
		return HandleError(err, "Failed to get work item details"), nil
	}

	// Check if JSON format is requested
	format, _ := GetStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		jsonResults := []map[string]any{}

		for _, item := range *workItems {
			fields := *item.Fields
			result := map[string]any{
				"id":    *item.Id,
				"title": fields["System.Title"],
				"state": fields["System.State"],
				"type":  fields["System.WorkItemType"],
				"url":   fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, *item.Id),
			}

			// Add assignee if available
			if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
				if displayName, ok := assignedTo["displayName"].(string); ok {
					result["assigned_to"] = displayName
				}
			}

			// Check for parent relations
			if item.Relations != nil {
				parentIDs := []int{}
				for _, relation := range *item.Relations {
					if *relation.Rel == "System.LinkTypes.Hierarchy-Reverse" {
						parentID, _ := ExtractWorkItemIDFromURL(*relation.Url)
						parentIDs = append(parentIDs, parentID)
					}
				}

				if len(parentIDs) > 0 {
					result["parent_ids"] = parentIDs
				}
			}

			jsonResults = append(jsonResults, result)
		}

		// Create response with metadata
		jsonResponse := map[string]any{
			"states":        states,
			"types":         types,
			"total_results": len(jsonResults),
			"page_size":     pageSize,
			"results":       jsonResults,
		}

		jsonData, err := json.MarshalIndent(jsonResponse, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}

	// Format results as text
	var results []string
	results = append(results, fmt.Sprintf("## Work Items in States: %s\n", statesStr))
	if typesStr != "" {
		results = append(results, fmt.Sprintf("Types: %s\n", typesStr))
	}
	results = append(results, fmt.Sprintf("Found %d work items\n", len(*workItems)))

	for _, item := range *workItems {
		fields := *item.Fields
		id := *item.Id
		title, _ := fields["System.Title"].(string)
		state, _ := fields["System.State"].(string)
		workItemType, _ := fields["System.WorkItemType"].(string)

		// Format the result
		result := fmt.Sprintf("### [%d] %s\n", id, title)
		result += fmt.Sprintf("Type: %s | State: %s\n", workItemType, state)

		// Add assignee if available
		if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
			if displayName, ok := assignedTo["displayName"].(string); ok {
				result += fmt.Sprintf("Assigned to: %s\n", displayName)
			}
		}

		// Check for parent links
		if item.Relations != nil {
			var parentIDs []string
			for _, relation := range *item.Relations {
				if *relation.Rel == "System.LinkTypes.Hierarchy-Reverse" {
					parentID, _ := ExtractWorkItemIDFromURL(*relation.Url)
					parentIDs = append(parentIDs, fmt.Sprintf("%d", parentID))
				}
			}

			if len(parentIDs) > 0 {
				result += fmt.Sprintf("Parent IDs: %s\n", strings.Join(parentIDs, ", "))
			}
		}

		// Add URL to the work item
		result += fmt.Sprintf("URL: %s/_workitems/edit/%d\n", tool.config.OrganizationURL, id)

		// Add separator
		result += "---\n"

		results = append(results, result)
	}

	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}
