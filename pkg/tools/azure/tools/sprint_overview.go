package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// SprintOverviewOutput defines the structure for the sprint overview.
type SprintOverviewOutput struct {
	ID                   string         `json:"id,omitempty"`
	Name                 string         `json:"name"`
	IterationPath        string         `json:"iteration_path"`
	StartDate            string         `json:"start_date,omitempty"`
	EndDate              string         `json:"end_date,omitempty"`
	TimeFrame            string         `json:"time_frame,omitempty"`
	URL                  string         `json:"url,omitempty"`
	Goal                 string         `json:"goal,omitempty"` // Sprint Goal, if available
	TotalWorkItems       int            `json:"total_work_items"`
	WorkItemsByState     map[string]int `json:"work_items_by_state"`
	WorkItemsByType      map[string]int `json:"work_items_by_type"`
	UniqueAssignees      []string       `json:"unique_assignees,omitempty"`
	SampleWorkItemTitles []string       `json:"sample_work_item_titles,omitempty"` // A few titles for context
}

// AzureSprintOverviewTool provides functionality to get an overview of a sprint.
type AzureSprintOverviewTool struct {
	handle         mcp.Tool
	workClient     work.Client             // For sprint details
	trackingClient workitemtracking.Client // For work items
	config         AzureDevOpsConfig
}

// NewAzureSprintOverviewTool creates a new tool instance.
func NewAzureSprintOverviewTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	wClient, err := work.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating work client for AzureSprintOverviewTool: %v\n", err)
		return nil
	}
	tClient, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating workitemtracking client for AzureSprintOverviewTool: %v\n", err)
		return nil
	}

	tool := &AzureSprintOverviewTool{
		workClient:     wClient,
		trackingClient: tClient,
		config:         config,
	}

	tool.handle = mcp.NewTool(
		"azure_sprint_overview",
		mcp.WithDescription("Get an overview of a specified or current Azure DevOps sprint, including item counts by state/type."),
		mcp.WithString(
			"sprint_identifier",
			mcp.Description("Optional. The iteration path or ID (GUID) of the sprint. If not provided, defaults to the current sprint for the configured team."),
		),
		mcp.WithString(
			"format",
			mcp.Description("Response format: 'text' (default) or 'json'."),
			mcp.Enum("text", "json"),
		),
	)
	return tool
}

func (tool *AzureSprintOverviewTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *AzureSprintOverviewTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sprintIdentifier, _ := GetStringArg(request, "sprint_identifier")
	format, _ := GetStringArg(request, "format")

	var sprintDetailsOutput SprintOutput // Reusing from get_sprints
	var iterationPathForQuery string

	// 1. Get Sprint Details
	if sprintIdentifier == "" || strings.ToLower(sprintIdentifier) == "@currentiteration" {
		// Get current sprint details
		currentSprintData, err := GetCurrentSprint(ctx, tool.workClient, tool.config.Project, tool.config.Team) // Updated call
		if err != nil {
			return HandleError(err, "Failed to get current sprint details"), nil
		}
		sprintDetailsOutput.Name = currentSprintData["name"].(string)
		sprintDetailsOutput.IterationPath = currentSprintData["path"].(string)
		if startDate, ok := currentSprintData["start_date"].(time.Time); ok {
			sprintDetailsOutput.StartDate = startDate.Format("2006-01-02")
		}
		if endDate, ok := currentSprintData["end_date"].(time.Time); ok {
			sprintDetailsOutput.EndDate = endDate.Format("2006-01-02")
		}
		// Assuming time_frame might also be time.Time, adjust if it's a different type or format.
		if timeFrame, ok := currentSprintData["time_frame"].(time.Time); ok {
			sprintDetailsOutput.TimeFrame = timeFrame.Format("2006-01-02") // Or a more suitable format
		} else if timeFrameStr, ok := currentSprintData["time_frame"].(string); ok {
			sprintDetailsOutput.TimeFrame = timeFrameStr
		}
		// ID and URL might not be in GetCurrentSprint's map, may need separate fetch or be part of its return.
		// For now, we prioritize path for the query.
		iterationPathForQuery = sprintDetailsOutput.IterationPath
		if idVal, ok := currentSprintData["id"].(string); ok {
			sprintDetailsOutput.ID = idVal
		}
		if urlVal, ok := currentSprintData["url"].(string); ok {
			sprintDetailsOutput.URL = urlVal
		}
	} else {
		// Try to get sprint by ID or Path. This is a simplified approach.
		// A robust way would be to list sprints and find by ID/Path or use a more direct API if available.
		// For now, assume sprintIdentifier can be an iteration path for querying items.
		// And attempt to get full details if it looks like a GUID (ID).
		// This part might need enhancement based on how GetTeamIterations works with specific IDs.

		iterationPathForQuery = sprintIdentifier // Assume it's a path first for WIQL

		// Attempt to fetch specific iteration if sprintIdentifier might be an ID (GUID)
		// Note: GetTeamIterations typically takes a timeframe. Getting a single iteration by ID might need a different call or iteration through all.
		// For simplicity, we'll rely on the identifier being the path for item query, and if it's an ID, fill what we can.
		sprintDetailsOutput.IterationPath = sprintIdentifier
		sprintDetailsOutput.Name = sprintIdentifier // Placeholder if full details not fetched this way

		// This is a placeholder. A real implementation would fetch the specific sprint by ID/Path
		// from the 'work' client to populate StartDate, EndDate, Goal, URL, etc.
		// For now, we will proceed assuming the iterationPathForQuery is the primary need for item fetching.
		// We can enhance sprint detail fetching later if needed.
	}

	if iterationPathForQuery == "" {
		return mcp.NewToolResultError("Could not determine sprint iteration path for querying work items."), nil
	}

	// 2. Get Work Items in the Sprint
	// Query to get IDs of work items in the sprint
	query := fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE [System.IterationPath] = '%s'", iterationPathForQuery)
	if tool.config.Team != "" && (sprintIdentifier == "" || strings.ToLower(sprintIdentifier) == "@currentiteration") {
		// Add team context for @CurrentIteration for some Azure DevOps setups.
		// The path itself should be team-specific if it's a full iteration path.
		query = fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = '%s' AND [System.IterationPath] = '%s' AND [System.State] <> 'Removed'", tool.config.Project, iterationPathForQuery)

	}

	wiqlArgs := workitemtracking.QueryByWiqlArgs{
		Wiql:    &workitemtracking.Wiql{Query: &query},
		Project: &tool.config.Project,
	}
	if tool.config.Team != "" && (sprintIdentifier == "" || strings.ToLower(sprintIdentifier) == "@currentiteration") {
		wiqlArgs.Team = &tool.config.Team // Context for @CurrentIteration
	}

	queryResult, err := tool.trackingClient.QueryByWiql(ctx, wiqlArgs)
	if err != nil {
		return HandleError(err, fmt.Sprintf("Failed to query work items for sprint path '%s'", iterationPathForQuery)), nil
	}

	var workItemIDs []int
	if queryResult.WorkItems != nil {
		for _, wiRef := range *queryResult.WorkItems {
			if wiRef.Id != nil {
				workItemIDs = append(workItemIDs, *wiRef.Id)
			}
		}
	}

	overview := SprintOverviewOutput{
		ID:                   sprintDetailsOutput.ID,
		Name:                 sprintDetailsOutput.Name,
		IterationPath:        sprintDetailsOutput.IterationPath,
		StartDate:            sprintDetailsOutput.StartDate,
		EndDate:              sprintDetailsOutput.EndDate,
		TimeFrame:            sprintDetailsOutput.TimeFrame,
		URL:                  sprintDetailsOutput.URL,
		TotalWorkItems:       len(workItemIDs),
		WorkItemsByState:     make(map[string]int),
		WorkItemsByType:      make(map[string]int),
		UniqueAssignees:      []string{},
		SampleWorkItemTitles: []string{},
	}

	// 3. Get Details for each Work Item to aggregate overview data
	if len(workItemIDs) > 0 {
		// Batch get work item details
		chunkSize := 200 // Max IDs per request for GetWorkItemsBatch
		assigneeSet := make(map[string]bool)

		for i := 0; i < len(workItemIDs); i += chunkSize {
			end := i + chunkSize
			if end > len(workItemIDs) {
				end = len(workItemIDs)
			}
			batchIDs := workItemIDs[i:end]

			// Use GetWorkItemsArgs, as GetWorkItemsBatchArgs seems incorrect or non-existent based on linter errors
			// and sprint_items.go uses GetWorkItemsArgs for batch retrieval.
			args := workitemtracking.GetWorkItemsArgs{
				Ids:     &batchIDs, // Capitalized 'Ids' as seen in sprint_items.go
				Project: &tool.config.Project,
				Fields: &[]string{ // Capitalized 'Fields'
					"System.Id", "System.Title", "System.State", "System.WorkItemType", "System.AssignedTo",
				},
				Expand: &workitemtracking.WorkItemExpandValues.None, // Explicitly set expand, common in these args structs
				// AsOf: nil, // Other optional fields if needed
				// ErrorPolicy: nil,
			}
			// The method is GetWorkItems, not GetWorkItemsBatch
			batchResult, err := tool.trackingClient.GetWorkItems(ctx, args)
			if err != nil {
				fmt.Printf("Warning: Failed to get batch details for some work items: %v\n", err)
				continue
			}

			if batchResult != nil { // batchResult is *[]workitemtracking.WorkItem
				for _, item := range *batchResult { // Dereference to get the slice []workitemtracking.WorkItem
					if item.Fields == nil {
						continue
					}
					fields := *item.Fields

					if state, ok := fields["System.State"].(string); ok {
						overview.WorkItemsByState[state]++
					}
					if itemType, ok := fields["System.WorkItemType"].(string); ok {
						overview.WorkItemsByType[itemType]++
					}
					if title, ok := fields["System.Title"].(string); ok && len(overview.SampleWorkItemTitles) < 5 { // Get up to 5 sample titles
						overview.SampleWorkItemTitles = append(overview.SampleWorkItemTitles, title)
					}
					if assignedToRaw, ok := fields["System.AssignedTo"].(map[string]any); ok {
						if displayName, ok := assignedToRaw["displayName"].(string); ok && displayName != "" {
							if !assigneeSet[displayName] {
								overview.UniqueAssignees = append(overview.UniqueAssignees, displayName)
								assigneeSet[displayName] = true
							}
						}
					}
				}
			}
		}
	}

	// 4. Format Output
	if strings.ToLower(format) == "json" {
		jsonBytes, err := json.MarshalIndent(overview, "", "  ")
		if err != nil {
			return HandleError(err, "Failed to serialize sprint overview to JSON"), nil
		}
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}

	// Text Output
	var textOutput []string
	textOutput = append(textOutput, fmt.Sprintf("## Sprint Overview: %s", overview.Name))
	if overview.ID != "" {
		textOutput = append(textOutput, fmt.Sprintf("ID: %s", overview.ID))
	}
	textOutput = append(textOutput, fmt.Sprintf("Iteration Path: %s", overview.IterationPath))
	if overview.StartDate != "" && overview.EndDate != "" {
		textOutput = append(textOutput, fmt.Sprintf("Dates: %s to %s (%s)", overview.StartDate, overview.EndDate, overview.TimeFrame))
	}
	if overview.URL != "" {
		textOutput = append(textOutput, fmt.Sprintf("URL: %s", overview.URL))
	}
	if overview.Goal != "" {
		textOutput = append(textOutput, fmt.Sprintf("Goal: %s", overview.Goal))
	}
	textOutput = append(textOutput, fmt.Sprintf("Total Work Items: %d", overview.TotalWorkItems))

	textOutput = append(textOutput, "\n### Work Items by State:")
	if len(overview.WorkItemsByState) > 0 {
		for state, count := range overview.WorkItemsByState {
			textOutput = append(textOutput, fmt.Sprintf("- %s: %d", state, count))
		}
	} else {
		textOutput = append(textOutput, "- No work items found or states not categorized.")
	}

	textOutput = append(textOutput, "\n### Work Items by Type:")
	if len(overview.WorkItemsByType) > 0 {
		for itemType, count := range overview.WorkItemsByType {
			textOutput = append(textOutput, fmt.Sprintf("- %s: %d", itemType, count))
		}
	} else {
		textOutput = append(textOutput, "- No work items found or types not categorized.")
	}

	if len(overview.UniqueAssignees) > 0 {
		textOutput = append(textOutput, "\n### Unique Assignees:")
		for _, assignee := range overview.UniqueAssignees {
			textOutput = append(textOutput, fmt.Sprintf("- %s", assignee))
		}
	}

	if len(overview.SampleWorkItemTitles) > 0 {
		textOutput = append(textOutput, "\n### Sample Work Item Titles:")
		for _, title := range overview.SampleWorkItemTitles {
			textOutput = append(textOutput, fmt.Sprintf("- %s", title))
		}
	}

	return mcp.NewToolResultText(strings.Join(textOutput, "\n")), nil
}
