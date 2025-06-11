package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// AzureSprintItemsTool provides functionality to find work items in the current sprint
type AzureSprintItemsTool struct {
	handle         mcp.Tool
	trackingClient workitemtracking.Client
	workClient     work.Client
	config         AzureDevOpsConfig
}

// SprintWorkItemOutput defines the structure for work item details for output.
type SprintWorkItemOutput struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	State         string `json:"state"`
	Type          string `json:"type"`
	URL           string `json:"url"`
	AssignedTo    string `json:"assigned_to,omitempty"`
	IterationPath string `json:"iteration_path,omitempty"`
	ParentIDs     []int  `json:"parent_ids,omitempty"`
}

// NewAzureSprintItemsTool creates a new tool instance for finding items in the current sprint
func NewAzureSprintItemsTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	tClient, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating workitemtracking client for AzureSprintItemsTool: %v\n", err)
		return nil
	}

	wClient, err := work.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating work client for AzureSprintItemsTool: %v\n", err)
		return nil
	}

	tool := &AzureSprintItemsTool{
		trackingClient: tClient,
		workClient:     wClient,
		config:         config,
	}

	tool.handle = mcp.NewTool(
		"azure_sprint_items",
		mcp.WithDescription("Find work items in a specified or current Azure DevOps sprint."),
		mcp.WithString(
			"iteration_path",
			mcp.Description("Optional. The iteration path of the sprint. If not provided, defaults to the current sprint for the configured team."),
		),
		mcp.WithString(
			"states",
			mcp.Description("Optional comma-separated list of states to filter by (e.g., 'DOING,REVIEW')"),
			mcp.Enum("TODO", "DOING", "REVIEW", "ACCEPTED", "DONE"),
		),
		mcp.WithString(
			"types",
			mcp.Description("Optional comma-separated list of work item types to filter by (e.g., 'Task,Bug')"),
			mcp.Enum("Task", "Bug", "User Story", "Epic"),
		),
		mcp.WithString(
			"format",
			mcp.Description("Response format: 'text' (default) or 'json'"),
			mcp.Enum("text", "json"),
		),
		mcp.WithString(
			"page_size",
			mcp.Description("Number of items per page (default: 50)"),
		),
		mcp.WithString("page", mcp.Description("Page number (default: 1)")),
	)

	return tool
}

func (tool *AzureSprintItemsTool) Handle() mcp.Tool {
	return tool.handle
}

// formatSprintItemsToJSON formats the work items into a JSON string.
func formatSprintItemsToJSON(sprintDetails map[string]any, items []SprintWorkItemOutput, page, pageSize, totalResults int) (string, error) {
	jsonResponse := map[string]any{
		"sprint":        sprintDetails,
		"total_results": totalResults,
		"page":          page,
		"page_size":     pageSize,
		"results":       items,
	}

	jsonData, err := json.MarshalIndent(jsonResponse, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// formatSprintItemsToText formats the work items into a text string.
func formatSprintItemsToText(sprintDetails map[string]any, items []SprintWorkItemOutput, page, pageSize, totalResults int) string {
	var results []string
	results = append(results, fmt.Sprintf("## Work Items in Current Sprint: %s\n", sprintDetails["name"]))
	results = append(results, fmt.Sprintf("Sprint Period: %s to %s\n",
		sprintDetails["start_date"],
		sprintDetails["end_date"]))
	results = append(results, fmt.Sprintf("Found %d work items\n", totalResults))

	for _, item := range items {
		result := fmt.Sprintf("### [%d] %s\n", item.ID, item.Title)
		result += fmt.Sprintf("Type: %s | State: %s\n", item.Type, item.State)

		if item.AssignedTo != "" {
			result += fmt.Sprintf("Assigned to: %s\n", item.AssignedTo)
		}
		if item.IterationPath != "" {
			result += fmt.Sprintf("Iteration: %s\n", item.IterationPath)
		}
		if len(item.ParentIDs) > 0 {
			var parentIDStrs []string
			for _, pid := range item.ParentIDs {
				parentIDStrs = append(parentIDStrs, strconv.Itoa(pid))
			}
			result += fmt.Sprintf("Parent IDs: %s\n", strings.Join(parentIDStrs, ", "))
		}
		result += fmt.Sprintf("URL: %s\n", item.URL)
		result += "---\n"
		results = append(results, result)
	}

	if totalResults > 0 && (page > 1 || totalResults == pageSize) {
		paginationInfo := fmt.Sprintf("\n## Pagination\nPage %d, %d items per page\n\n", page, pageSize)
		if page > 1 {
			paginationInfo += fmt.Sprintf("Previous page: `{\"page\": \"%d\", \"page_size\": \"%d\"}`\n\n",
				page-1, pageSize)
		}
		if totalResults == pageSize {
			paginationInfo += fmt.Sprintf("Next page: `{\"page\": \"%d\", \"page_size\": \"%d\"}`",
				page+1, pageSize)
		}
		results = append(results, paginationInfo)
	}
	return strings.Join(results, "\n")
}

func (tool *AzureSprintItemsTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get optional iteration_path parameter
	iterationPath, _ := GetStringArg(request, "iteration_path")

	// Get current sprint information IF iterationPath is not provided
	var sprintNameForQuery string
	var sprintDetailsForOutput map[string]any
	teamID := tool.config.Team // Team ID for @CurrentIteration context

	if iterationPath == "" {
		// Pass the workClient to GetCurrentSprint
		sprintInfo, err := GetCurrentSprint(ctx, tool.workClient, tool.config.Project, tool.config.Team)
		if err != nil {
			return HandleError(err, "Failed to get current sprint details when iteration_path is not specified"), nil
		}
		sprintNameForQuery = "@CurrentIteration" // Query still uses @CurrentIteration macro
		sprintDetailsForOutput = sprintInfo      // This now contains time.Time objects for dates
	} else {
		sprintNameForQuery = iterationPath
		// When a specific iteration_path is given, we might not have full start/end date details easily
		// without another API call. For now, the output will reflect the provided path.
		sprintDetailsForOutput = map[string]any{
			"name": iterationPath,
			"path": iterationPath,
		}
	}

	// Get optional parameters
	var workItemTypes string
	if typesStr, ok := request.Params.Arguments["types"].(string); ok && typesStr != "" {
		workItemTypes = typesStr // User provided: e.g., "Task,Bug"
	} else {
		// Default to common work item types, unquoted
		workItemTypes = "Task,Bug,User Story,Epic"
	}

	var states string
	if statesStr, ok := request.Params.Arguments["states"].(string); ok && statesStr != "" {
		states = statesStr // User provided: e.g., "TODO,DOING"
	} else {
		// Default to active states, unquoted
		states = "TODO,DOING,REVIEW,ACCEPTED,DONE"
	}

	// Parse types and states
	// Ensure elements are clean (no extra quotes) before FormatStringList adds its own.
	rawTypes := strings.Split(workItemTypes, ",")
	types := make([]string, 0, len(rawTypes))
	for _, t := range rawTypes {
		cleanType := strings.TrimSpace(t)
		// Remove any surrounding single or double quotes from individual items from user input or old defaults
		cleanType = strings.Trim(cleanType, "'")
		cleanType = strings.Trim(cleanType, "\"")
		if cleanType != "" {
			types = append(types, cleanType)
		}
	}

	rawStates := strings.Split(states, ",")
	statesList := make([]string, 0, len(rawStates))
	for _, s := range rawStates {
		cleanState := strings.TrimSpace(s)
		// Remove any surrounding single or double quotes
		cleanState = strings.Trim(cleanState, "'")
		cleanState = strings.Trim(cleanState, "\"")
		if cleanState != "" {
			statesList = append(statesList, cleanState)
		}
	}

	// Handle paging
	pageSize := 50 // Default page size
	page := 1      // Default page number

	if pageSizeStr, ok := request.Params.Arguments["page_size"].(string); ok && pageSizeStr != "" {
		if pageSizeInt, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = pageSizeInt
		}
	}

	if pageStr, ok := request.Params.Arguments["page"].(string); ok && pageStr != "" {
		if pageInt, err := strconv.Atoi(pageStr); err == nil {
			page = pageInt
		}
	}

	// Build the WIQL query to find items in the specified or current sprint/iteration
	var query string
	if sprintNameForQuery == "@CurrentIteration" {
		query = fmt.Sprintf(`SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo] FROM WorkItems WHERE [System.WorkItemType] IN (%s) AND [System.State] IN (%s) AND [System.IterationPath] = @CurrentIteration ORDER BY [System.ChangedDate] DESC`,
			FormatStringList(types), FormatStringList(statesList))
	} else {
		// Escape single quotes for non-macro iteration paths
		escapedIterationPath := strings.ReplaceAll(sprintNameForQuery, "'", "''")
		query = fmt.Sprintf(`SELECT [System.Id], [System.Title], [System.State], [System.WorkItemType], [System.AssignedTo] FROM WorkItems WHERE [System.WorkItemType] IN (%s) AND [System.State] IN (%s) AND [System.IterationPath] UNDER '%s' ORDER BY [System.ChangedDate] DESC`,
			FormatStringList(types), FormatStringList(statesList), escapedIterationPath)
	}

	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql: &workitemtracking.Wiql{
			Query: &query,
		},
		Project: &tool.config.Project,
	}

	// Add Team context only if using @CurrentIteration
	if iterationPath == "" {
		wiqlArgs.Team = &teamID
	}

	// Apply limit using the Top field if pageSize is specified
	if pageSize > 0 {
		top := pageSize
		wiqlArgs.Top = &top
	}

	queryResult, err := tool.trackingClient.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		return HandleError(err, "Failed to query work items"), nil
	}

	// If no items found, return a helpful message
	if len(*queryResult.WorkItems) == 0 {
		message := fmt.Sprintf("No work items found in sprint: %s", sprintNameForQuery)
		if iterationPath == "" && sprintDetailsForOutput != nil && sprintDetailsForOutput["name"] != nil {
			endDateStr := "unknown"
			startDateStr := "unknown"

			if dateVal, ok := sprintDetailsForOutput["start_date"].(time.Time); ok && !dateVal.IsZero() {
				startDateStr = dateVal.Format("2006-01-02")
			} else if strVal, ok := sprintDetailsForOutput["start_date"].(string); ok { // Fallback if already string
				startDateStr = strVal
			}

			if dateVal, ok := sprintDetailsForOutput["end_date"].(time.Time); ok && !dateVal.IsZero() {
				endDateStr = dateVal.Format("2006-01-02")
			} else if strVal, ok := sprintDetailsForOutput["end_date"].(string); ok { // Fallback if already string
				endDateStr = strVal
			}

			message = fmt.Sprintf("No work items found in the current sprint: %s (%s to %s).",
				sprintDetailsForOutput["name"], startDateStr, endDateStr)
		}
		return mcp.NewToolResultText(message), nil
	}

	// Get IDs of matching work items
	ids := make([]int, len(*queryResult.WorkItems))
	for i, item := range *queryResult.WorkItems {
		ids[i] = *item.Id
	}

	// Get details of the work items
	workItems, err := tool.trackingClient.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &ids,
		Project: &tool.config.Project,
		Expand:  &workitemtracking.WorkItemExpandValues.Relations,
	})

	if err != nil {
		return HandleError(err, "Failed to get work item details"), nil
	}

	outputItems := []SprintWorkItemOutput{}
	for _, item := range *workItems {
		fields := *item.Fields
		outputItem := SprintWorkItemOutput{
			ID:  *item.Id,
			URL: fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, *item.Id),
		}

		if title, ok := fields["System.Title"].(string); ok {
			outputItem.Title = title
		}
		if state, ok := fields["System.State"].(string); ok {
			outputItem.State = state
		}
		if workItemType, ok := fields["System.WorkItemType"].(string); ok {
			outputItem.Type = workItemType
		}

		if assignedTo, ok := fields["System.AssignedTo"].(map[string]any); ok {
			if displayName, ok := assignedTo["displayName"].(string); ok {
				outputItem.AssignedTo = displayName
			}
		}

		if iterationPath, ok := fields["System.IterationPath"].(string); ok {
			outputItem.IterationPath = iterationPath
		}

		if item.Relations != nil {
			parentIDs := []int{}
			for _, relation := range *item.Relations {
				if *relation.Rel == "System.LinkTypes.Hierarchy-Reverse" {
					// Assuming ExtractWorkItemIDFromURL is robust enough or used as before.
					// For a more robust solution, handle potential error from ExtractWorkItemIDFromURL.
					parentID, _ := ExtractWorkItemIDFromURL(*relation.Url)
					// if err == nil { // Proper error handling would be here
					parentIDs = append(parentIDs, parentID)
					// }
				}
			}
			if len(parentIDs) > 0 {
				outputItem.ParentIDs = parentIDs
			}
		}
		outputItems = append(outputItems, outputItem)
	}

	// Adjust sprint details for output based on whether it was current or specified
	finalSprintDetails := sprintDetailsForOutput // This map comes from GetCurrentSprint

	if iterationPath == "" {
		if finalSprintDetails != nil {
			name, _ := finalSprintDetails["name"].(string)
			idVal, _ := finalSprintDetails["id"].(string)     // Retrieve id
			pathVal, _ := finalSprintDetails["path"].(string) // Retrieve path

			startDateVal, okStartDateVal := finalSprintDetails["start_date"]
			endDateVal, okEndDateVal := finalSprintDetails["end_date"]

			startDateStr := "N/A"
			endDateStr := "N/A"

			if okStartDateVal {
				if startDate, okCast := startDateVal.(time.Time); okCast && !startDate.IsZero() {
					startDateStr = startDate.Format("2006-01-02")
				}
			}

			if okEndDateVal {
				if endDate, okCast := endDateVal.(time.Time); okCast && !endDate.IsZero() {
					endDateStr = endDate.Format("2006-01-02")
				}
			}
			finalSprintDetails = map[string]any{
				"id":         idVal,
				"name":       name,
				"path":       pathVal,
				"start_date": startDateStr,
				"end_date":   endDateStr,
			}
		} else {
			// Fallback if GetCurrentSprint returned nil (error should have been handled earlier)
			finalSprintDetails = map[string]any{
				"name":       "@CurrentIteration (details unavailable)",
				"start_date": "N/A",
				"end_date":   "N/A",
			}
		}
	} else { // iterationPath is provided
		// For a specified path, we only have the path itself as the name for now
		// Dates are not fetched for specified iteration_path in current logic for sprint_items
		finalSprintDetails = map[string]any{
			"name":       iterationPath,
			"path":       iterationPath,
			"start_date": "N/A", // Explicitly state N/A as dates are not fetched here
			"end_date":   "N/A", // Explicitly state N/A as dates are not fetched here
		}
	}

	format, _ := GetStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		jsonDataString, err := formatSprintItemsToJSON(finalSprintDetails, outputItems, page, pageSize, len(outputItems))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(jsonDataString), nil
	}

	textDataString := formatSprintItemsToText(finalSprintDetails, outputItems, page, pageSize, len(outputItems))
	return mcp.NewToolResultText(textDataString), nil
}
