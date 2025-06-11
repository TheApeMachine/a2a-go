package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7" // Alias for core package
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// AzureCreateSprintTool provides functionality to create new sprints (iterations).
type AzureCreateSprintTool struct {
	handle mcp.Tool
	client work.Client // Using work.Client for iteration management
	config AzureDevOpsConfig
}

// NewAzureCreateSprintTool creates a new tool instance for creating sprints.
func NewAzureCreateSprintTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	workClient, err := work.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating work client for AzureCreateSprintTool: %v\n", err)
		return nil
	}

	tool := &AzureCreateSprintTool{
		client: workClient,
		config: config,
	}

	tool.handle = mcp.NewTool(
		"azure_create_sprint",
		mcp.WithDescription("Create a new sprint (iteration) in Azure DevOps."),
		mcp.WithString(
			"name",
			mcp.Required(),
			mcp.Description("Name of the new sprint."),
		),
		mcp.WithString(
			"start_date",
			mcp.Required(),
			mcp.Description("Start date of the sprint in YYYY-MM-DD format."),
		),
		mcp.WithString(
			"finish_date",
			mcp.Required(),
			mcp.Description("End date of the sprint in YYYY-MM-DD format."),
		),
		mcp.WithString(
			"format",
			mcp.Description("Response format: 'text' (default) or 'json'."),
			mcp.Enum("text", "json"),
		),
	)
	return tool
}

func (tool *AzureCreateSprintTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *AzureCreateSprintTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := GetStringArg(request, "name")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: name."), nil
	}

	startDateStr, err := GetStringArg(request, "start_date")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: start_date."), nil
	}

	finishDateStr, err := GetStringArg(request, "finish_date")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: finish_date."), nil
	}

	layout := "2006-01-02"
	parsedStartDate, err := time.Parse(layout, startDateStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid start_date format (%s). Expected YYYY-MM-DD. Error: %v", startDateStr, err)), nil
	}
	parsedFinishDate, err := time.Parse(layout, finishDateStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid finish_date format (%s). Expected YYYY-MM-DD. Error: %v", finishDateStr, err)), nil
	}

	iterationToCreate := work.TeamSettingsIteration{
		Name: &name,
		Attributes: &work.TeamIterationAttributes{
			StartDate:  &azuredevops.Time{Time: parsedStartDate},
			FinishDate: &azuredevops.Time{Time: parsedFinishDate},
		},
	}

	args := work.PostTeamIterationArgs{
		Iteration: &iterationToCreate,
		Team:      &tool.config.Team,
		Project:   &tool.config.Project,
	}

	createdIteration, err := tool.client.PostTeamIteration(ctx, args)
	if err != nil {
		return HandleError(err, "Failed to create sprint"), nil
	}

	output := map[string]any{
		"id":   *createdIteration.Id,
		"name": *createdIteration.Name,
		"path": *createdIteration.Path,
		"url":  *createdIteration.Url,
	}
	if createdIteration.Attributes != nil {
		if createdIteration.Attributes.StartDate != nil {
			output["start_date"] = createdIteration.Attributes.StartDate.Time.Format("2006-01-02")
		}
		if createdIteration.Attributes.FinishDate != nil {
			output["finish_date"] = createdIteration.Attributes.FinishDate.Time.Format("2006-01-02")
		}
		if createdIteration.Attributes.TimeFrame != nil {
			output["time_frame"] = string(*createdIteration.Attributes.TimeFrame)
		}
	}

	format, _ := GetStringArg(request, "format")
	if strings.ToLower(format) == "json" {
		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonData)), nil
	}

	textResponse := fmt.Sprintf("Successfully created sprint:\nID: %s\nName: %s\nPath: %s",
		output["id"], output["name"], output["path"])
	if sd, ok := output["start_date"]; ok {
		textResponse += fmt.Sprintf("\nStart Date: %s", sd)
	}
	if ed, ok := output["finish_date"]; ok {
		textResponse += fmt.Sprintf("\nEnd Date: %s", ed)
	}
	if url, ok := output["url"]; ok {
		textResponse += fmt.Sprintf("\nURL: %s", url)
	}
	return mcp.NewToolResultText(textResponse), nil
}

// GetStringArg, HandleError would typically be in a common package.
// For now, assuming they are accessible or will be moved.
// If not, they need to be defined or imported here.
// For brevity, let's assume GetStringArg is available from a shared context or `tools.GetStringArg` if that's the pattern.
// HandleError too.
