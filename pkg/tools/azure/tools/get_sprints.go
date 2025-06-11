package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// SprintOutput defines the structure for a single sprint's details for output.
// This structure is adapted from the previous pkg/tools/azure/sprint.go
type SprintOutput struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IterationPath string `json:"iteration_path"`
	StartDate     string `json:"start_date,omitempty"`
	EndDate       string `json:"end_date,omitempty"`
	TimeFrame     string `json:"time_frame,omitempty"`
	URL           string `json:"url,omitempty"`
}

// AzureGetSprintsTool provides functionality to list sprints (iterations).
type AzureGetSprintsTool struct {
	handle mcp.Tool
	client work.Client // Using work.Client for iteration management
	config AzureDevOpsConfig
}

// NewAzureGetSprintsTool creates a new tool instance for listing sprints.
func NewAzureGetSprintsTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	workClient, err := work.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating work client for AzureGetSprintsTool: %v\n", err)
		return nil
	}

	tool := &AzureGetSprintsTool{
		client: workClient,
		config: config,
	}

	tool.handle = mcp.NewTool(
		"azure_get_sprints",
		mcp.WithDescription("Get sprints (iterations) in Azure DevOps for the configured team."),
		mcp.WithString(
			"include_completed",
			mcp.Description("Whether to include completed sprints (default: false). Set to 'true' to include them."),
			mcp.Enum("true", "false"),
		),
		mcp.WithString(
			"format",
			mcp.Description("Response format: 'text' (default) or 'json'."),
			mcp.Enum("text", "json"),
		),
	)
	return tool
}

func (tool *AzureGetSprintsTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *AzureGetSprintsTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	includeCompletedStr, _ := GetStringArg(request, "include_completed")
	includeCompleted := strings.ToLower(includeCompletedStr) == "true"

	if includeCompleted {
		var allIterations []work.TeamSettingsIteration

		// Fetch Past
		pastTimeframe := "past"
		pastArgs := work.GetTeamIterationsArgs{Project: &tool.config.Project, Team: &tool.config.Team, Timeframe: &pastTimeframe}
		pastIterations, err := tool.client.GetTeamIterations(ctx, pastArgs)
		if err == nil && pastIterations != nil {
			allIterations = append(allIterations, *pastIterations...)
		}

		// Fetch Current
		currentTimeframe := "current"
		currentArgs := work.GetTeamIterationsArgs{Project: &tool.config.Project, Team: &tool.config.Team, Timeframe: &currentTimeframe}
		currentIterations, err := tool.client.GetTeamIterations(ctx, currentArgs)
		if err == nil && currentIterations != nil {
			allIterations = append(allIterations, *currentIterations...)
		}

		// Fetch Future
		futureTimeframe := "future"
		futureArgs := work.GetTeamIterationsArgs{Project: &tool.config.Project, Team: &tool.config.Team, Timeframe: &futureTimeframe}
		futureIterations, err := tool.client.GetTeamIterations(ctx, futureArgs)
		if err == nil && futureIterations != nil {
			allIterations = append(allIterations, *futureIterations...)
		}

		// Deduplicate (iterations might overlap if "current" also appears in "past" or "future" conceptually, though API should handle this)
		seenIDs := make(map[string]bool)
		deduplicatedIterations := []work.TeamSettingsIteration{}
		for _, iteration := range allIterations {
			if iteration.Id != nil && !seenIDs[iteration.Id.String()] {
				deduplicatedIterations = append(deduplicatedIterations, iteration)
				seenIDs[iteration.Id.String()] = true
			}
		}
		return tool.formatAndReturnIterations(deduplicatedIterations, request)

	} else { // Only current and future, not including past/completed
		var currentAndFutureIterations []work.TeamSettingsIteration

		// Fetch Current
		currentTimeframe := "current"
		currentArgs := work.GetTeamIterationsArgs{Project: &tool.config.Project, Team: &tool.config.Team, Timeframe: &currentTimeframe}
		currentIterations, err := tool.client.GetTeamIterations(ctx, currentArgs)
		if err == nil && currentIterations != nil {
			currentAndFutureIterations = append(currentAndFutureIterations, *currentIterations...)
		} else if err != nil {
			// Log or handle error for current timeframe fetch, but try to proceed if possible
			fmt.Printf("Warning: Failed to get current sprints: %v\n", err)
		}

		// Fetch Future
		futureTimeframe := "future"
		futureArgs := work.GetTeamIterationsArgs{Project: &tool.config.Project, Team: &tool.config.Team, Timeframe: &futureTimeframe}
		futureIterations, err := tool.client.GetTeamIterations(ctx, futureArgs)
		if err == nil && futureIterations != nil {
			currentAndFutureIterations = append(currentAndFutureIterations, *futureIterations...)
		} else if err != nil {
			// Log or handle error for future timeframe fetch
			fmt.Printf("Warning: Failed to get future sprints: %v\n", err)
		}

		// Deduplicate, as current might overlap if API behaves unexpectedly, though usually distinct
		seenIDs := make(map[string]bool)
		deduplicatedCurrentFuture := []work.TeamSettingsIteration{}
		for _, iteration := range currentAndFutureIterations {
			if iteration.Id != nil && !seenIDs[iteration.Id.String()] {
				deduplicatedCurrentFuture = append(deduplicatedCurrentFuture, iteration)
				seenIDs[iteration.Id.String()] = true
			}
		}

		if len(deduplicatedCurrentFuture) == 0 && err != nil {
			// If both calls failed or returned nothing and there was an error from one of them
			return HandleError(err, "Failed to get current or future sprints after attempting both timeframes"), nil
		}
		if len(deduplicatedCurrentFuture) == 0 {
			return mcp.NewToolResultText("No current or future sprints found for the team."), nil
		}

		return tool.formatAndReturnIterations(deduplicatedCurrentFuture, request)
	}
}

func (tool *AzureGetSprintsTool) formatAndReturnIterations(iterations []work.TeamSettingsIteration, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var sprintOutputs []SprintOutput
	for _, iteration := range iterations {
		iterationIdStr := ""
		if iteration.Id != nil {
			iterationIdStr = iteration.Id.String()
		}
		sprint := SprintOutput{
			ID:            iterationIdStr,
			Name:          SafeString(iteration.Name),
			IterationPath: SafeString(iteration.Path),
			URL:           SafeString(iteration.Url),
		}
		if iteration.Attributes != nil {
			if iteration.Attributes.StartDate != nil {
				sprint.StartDate = iteration.Attributes.StartDate.Time.Format("2006-01-02")
			}
			if iteration.Attributes.FinishDate != nil {
				sprint.EndDate = iteration.Attributes.FinishDate.Time.Format("2006-01-02")
			}
			if iteration.Attributes.TimeFrame != nil {
				sprint.TimeFrame = string(*iteration.Attributes.TimeFrame)
			}
		}
		sprintOutputs = append(sprintOutputs, sprint)
	}

	format, _ := GetStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		if len(sprintOutputs) == 0 {
			// Return empty array for JSON if no sprints found
			return mcp.NewToolResultText("[]"), nil
		}
		jsonStr, err := json.MarshalIndent(sprintOutputs, "", "  ")
		if err != nil {
			return HandleError(err, "Failed to serialize sprints to JSON"), nil
		}
		return mcp.NewToolResultText(string(jsonStr)), nil
	}

	// Text format
	if len(sprintOutputs) == 0 {
		return mcp.NewToolResultText("No sprints found matching the criteria."), nil
	}
	var results []string
	results = append(results, "## Sprints List\n")
	for _, sprint := range sprintOutputs {
		line := fmt.Sprintf("Name: %s\n  ID: %s\n  Iteration Path: %s", sprint.Name, sprint.ID, sprint.IterationPath)
		if sprint.StartDate != "" {
			line += fmt.Sprintf("\n  Start Date: %s", sprint.StartDate)
		}
		if sprint.EndDate != "" {
			line += fmt.Sprintf("\n  End Date: %s", sprint.EndDate)
		}
		if sprint.TimeFrame != "" {
			line += fmt.Sprintf("\n  TimeFrame: %s", sprint.TimeFrame)
		}
		if sprint.URL != "" {
			line += fmt.Sprintf("\n  URL: %s", sprint.URL)
		}
		line += "\n---"
		results = append(results, line)
	}
	return mcp.NewToolResultText(strings.Join(results, "\n")), nil
}

// SafeString dereferences a string pointer and returns its value, or an empty string if nil.
func SafeString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// GetStringArg, HandleError would typically be in a common package (e.g., tools.common.go)
// For now, ensure they are accessible or defined if this is the first tool file.
// Let's assume they are in tools/common.go
