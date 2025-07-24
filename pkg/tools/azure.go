package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	azuretools "github.com/theapemachine/a2a-go/pkg/tools/azure/tools"
)

type AzureEnrichWorkItemTool struct {
	tool *mcp.Tool
}

func NewAzureEnrichWorkItemTool() *mcp.Tool {
	// This will need to be customized based on the actual tool parameters
	tool := mcp.NewTool(
		"azure_EnrichWorkItem",
		mcp.WithDescription("Azure DevOps EnrichWorkItem tool"),
		// TODO: Add specific parameters for this tool
	)

	return &tool
}

func (at *AzureEnrichWorkItemTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_EnrichWorkItem tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureEnrichWorkItemTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps EnrichWorkItem tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureExecuteWiqlTool struct {
	tool *mcp.Tool
}

func NewAzureExecuteWiqlTool() *mcp.Tool {
	tool := mcp.NewTool(
		"azure_execute_wiql",
		mcp.WithDescription("Execute a WIQL query on Azure DevOps, returning the results."),
		mcp.WithString(
			"query",
			mcp.Required(),
			mcp.Description("WIQL query string for searching work items."),
		),
	)

	return &tool
}

func (at *AzureExecuteWiqlTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_execute_wiql tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureExecuteWiqlTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps execute WIQL tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureGetGithubFileContentTool struct {
	tool *mcp.Tool
}

func NewAzureGetGithubFileContentTool() *mcp.Tool {
	// This will need to be customized based on the actual tool parameters
	tool := mcp.NewTool(
		"azure_GetGithubFileContent",
		mcp.WithDescription("Azure DevOps GetGithubFileContent tool"),
		// TODO: Add specific parameters for this tool
	)

	return &tool
}

func (at *AzureGetGithubFileContentTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_GetGithubFileContent tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	// No Azure DevOps connection needed for GitHub file content
	azureTool := azuretools.NewAzureGetGitHubFileContentTool()
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps GetGithubFileContent tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureSearchWorkItemsTool struct {
	tool *mcp.Tool
}

func NewAzureSearchWorkItemsTool() *mcp.Tool {
	// This will need to be customized based on the actual tool parameters
	tool := mcp.NewTool(
		"azure_SearchWorkItems",
		mcp.WithDescription("Azure DevOps SearchWorkItems tool"),
		// TODO: Add specific parameters for this tool
	)

	return &tool
}

func (at *AzureSearchWorkItemsTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_SearchWorkItems tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureSearchWorkItemsTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps SearchWorkItems tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureSprintItemsTool struct {
	tool *mcp.Tool
}

func NewAzureSprintItemsTool() *mcp.Tool {
	// This will need to be customized based on the actual tool parameters
	tool := mcp.NewTool(
		"azure_SprintItems",
		mcp.WithDescription("Azure DevOps SprintItems tool"),
		// TODO: Add specific parameters for this tool
	)

	return &tool
}

func (at *AzureSprintItemsTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_SprintItems tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureSprintItemsTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps SprintItems tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureSprintOverviewTool struct {
	tool *mcp.Tool
}

func NewAzureSprintOverviewTool() *mcp.Tool {
	// This will need to be customized based on the actual tool parameters
	tool := mcp.NewTool(
		"azure_SprintOverview",
		mcp.WithDescription("Azure DevOps SprintOverview tool"),
		// TODO: Add specific parameters for this tool
	)

	return &tool
}

func (at *AzureSprintOverviewTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_SprintOverview tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureSprintOverviewTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps SprintOverview tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureFindItemsByStatusTool struct {
	tool *mcp.Tool
}

func NewAzureFindItemsByStatusTool() *mcp.Tool {
	// This will need to be customized based on the actual tool parameters
	tool := mcp.NewTool(
		"azure_FindItemsByStatus",
		mcp.WithDescription("Azure DevOps FindItemsByStatus tool"),
		// TODO: Add specific parameters for this tool
	)

	return &tool
}

func (at *AzureFindItemsByStatusTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_FindItemsByStatus tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureFindItemsByStatusTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps FindItemsByStatus tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureGetWorkItemsTool struct {
	tool *mcp.Tool
}

func NewAzureGetWorkItemsTool() *mcp.Tool {
	// This will need to be customized based on the actual tool parameters
	tool := mcp.NewTool(
		"azure_GetWorkItems",
		mcp.WithDescription("Azure DevOps GetWorkItems tool"),
		// TODO: Add specific parameters for this tool
	)

	return &tool
}

func (at *AzureGetWorkItemsTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_GetWorkItems tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureGetWorkItemsTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps GetWorkItems tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureUpdateWorkItemsTool struct {
	tool *mcp.Tool
}

func NewAzureUpdateWorkItemsTool() *mcp.Tool {
	// This will need to be customized based on the actual tool parameters
	tool := mcp.NewTool(
		"azure_UpdateWorkItems",
		mcp.WithDescription("Azure DevOps UpdateWorkItems tool"),
		// TODO: Add specific parameters for this tool
	)

	return &tool
}

func (at *AzureUpdateWorkItemsTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_UpdateWorkItems tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureUpdateWorkItemsTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps UpdateWorkItems tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureGetSprintsTool struct {
	tool *mcp.Tool
}

func NewAzureGetSprintsTool() *mcp.Tool {
	tool := mcp.NewTool(
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

	return &tool
}

func (at *AzureGetSprintsTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_get_sprints tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)
	_ = conn // connection not required for this tool
	workClient, err := work.NewClient(ctx, conn)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create work client: %v", err)), nil
	}

	// Parse arguments
	includeCompletedStr, _ := azuretools.GetStringArg(req, "include_completed")
	includeCompleted := strings.ToLower(includeCompletedStr) == "true"

	var allIterations []work.TeamSettingsIteration

	if includeCompleted {
		// Fetch Past
		pastTimeframe := "past"
		pastArgs := work.GetTeamIterationsArgs{Project: &config.Project, Team: &config.Team, Timeframe: &pastTimeframe}
		pastIterations, err := workClient.GetTeamIterations(ctx, pastArgs)
		if err == nil && pastIterations != nil {
			allIterations = append(allIterations, *pastIterations...)
		}

		// Fetch Current
		currentTimeframe := "current"
		currentArgs := work.GetTeamIterationsArgs{Project: &config.Project, Team: &config.Team, Timeframe: &currentTimeframe}
		currentIterations, err := workClient.GetTeamIterations(ctx, currentArgs)
		if err == nil && currentIterations != nil {
			allIterations = append(allIterations, *currentIterations...)
		}

		// Fetch Future
		futureTimeframe := "future"
		futureArgs := work.GetTeamIterationsArgs{Project: &config.Project, Team: &config.Team, Timeframe: &futureTimeframe}
		futureIterations, err := workClient.GetTeamIterations(ctx, futureArgs)
		if err == nil && futureIterations != nil {
			allIterations = append(allIterations, *futureIterations...)
		}
	} else {
		// Only current and future
		currentTimeframe := "current"
		currentArgs := work.GetTeamIterationsArgs{Project: &config.Project, Team: &config.Team, Timeframe: &currentTimeframe}
		currentIterations, err := workClient.GetTeamIterations(ctx, currentArgs)
		if err == nil && currentIterations != nil {
			allIterations = append(allIterations, *currentIterations...)
		}

		futureTimeframe := "future"
		futureArgs := work.GetTeamIterationsArgs{Project: &config.Project, Team: &config.Team, Timeframe: &futureTimeframe}
		futureIterations, err := workClient.GetTeamIterations(ctx, futureArgs)
		if err == nil && futureIterations != nil {
			allIterations = append(allIterations, *futureIterations...)
		}
	}

	// Deduplicate iterations
	seenIDs := make(map[string]bool)
	var deduplicatedIterations []work.TeamSettingsIteration
	for _, iteration := range allIterations {
		if iteration.Id != nil && !seenIDs[iteration.Id.String()] {
			deduplicatedIterations = append(deduplicatedIterations, iteration)
			seenIDs[iteration.Id.String()] = true
		}
	}

	if len(deduplicatedIterations) == 0 {
		return mcp.NewToolResultText("No sprints found for the team."), nil
	}

	// Format output
	var sprintOutputs []azuretools.SprintOutput
	for _, iteration := range deduplicatedIterations {
		sprint := azuretools.SprintOutput{
			ID:            iteration.Id.String(),
			Name:          azuretools.SafeString(iteration.Name),
			IterationPath: azuretools.SafeString(iteration.Path),
			URL:           azuretools.SafeString(iteration.Url),
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

	format, _ := azuretools.GetStringArg(req, "format")
	if strings.ToLower(format) == "json" {
		jsonStr, err := json.MarshalIndent(sprintOutputs, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize sprints to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonStr)), nil
	}

	// Text format
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

type AzureCreateWorkItemsTool struct {
	tool *mcp.Tool
}

func NewAzureCreateWorkItemsTool() *mcp.Tool {
	tool := mcp.NewTool(
		"azure_create_work_items",
		mcp.WithDescription("Create one or more new work items in Azure DevOps, with support for custom fields and parent linking."),
		mcp.WithString(
			"items_json",
			mcp.Required(),
			mcp.Description("A JSON string representing an array of work items to create. Each item object should define 'type', 'title', and optionally 'description' (using HTML, not Markdown), 'state', 'priority', 'parent_id', 'assigned_to', 'iteration', 'area', 'tags', and 'custom_fields' (as a map)."),
		),
		mcp.WithString("format", mcp.Description("Response format: 'text' (default) or 'json'")),
	)

	return &tool
}

func (at *AzureCreateWorkItemsTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_create_work_items tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)
	client, err := workitemtracking.NewClient(ctx, conn)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create work item tracking client: %v", err)), nil
	}

	// Parse arguments
	itemsJSON, err := azuretools.GetStringArg(req, "items_json")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: items_json."), nil
	}
	format, _ := azuretools.GetStringArg(req, "format")

	var itemsToCreate []azuretools.WorkItemDefinition
	if err := json.Unmarshal([]byte(itemsJSON), &itemsToCreate); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid JSON format for items_json: %v. Expected an array of work item objects.", err)), nil
	}

	if len(itemsToCreate) == 0 {
		return mcp.NewToolResultError("No work items provided in items_json array."), nil
	}

	var results []map[string]any
	var textResults []string

	for _, itemDef := range itemsToCreate {
		if itemDef.Type == "" || itemDef.Title == "" {
			errMsg := fmt.Sprintf("Skipped item due to missing 'type' or 'title'. Provided: %+v", itemDef)
			results = append(results, map[string]any{"error": errMsg})
			textResults = append(textResults, errMsg)
			continue
		}

		document := []webapi.JsonPatchOperation{
			azuretools.AddOperation("System.Title", itemDef.Title),
		}

		if itemDef.Description != "" {
			document = append(document, azuretools.AddOperation("System.Description", itemDef.Description))
		}
		if itemDef.State != "" {
			document = append(document, azuretools.AddOperation("System.State", itemDef.State))
		}
		if itemDef.Priority != "" {
			document = append(document, azuretools.AddOperation("Microsoft.VSTS.Common.Priority", itemDef.Priority))
		}
		if itemDef.AssignedTo != "" {
			document = append(document, azuretools.AddOperation("System.AssignedTo", itemDef.AssignedTo))
		}
		if itemDef.Iteration != "" {
			document = append(document, azuretools.AddOperation("System.IterationPath", itemDef.Iteration))
		}
		if itemDef.Area != "" {
			document = append(document, azuretools.AddOperation("System.AreaPath", itemDef.Area))
		}
		if itemDef.Tags != "" {
			document = append(document, azuretools.AddOperation("System.Tags", itemDef.Tags))
		}

		// Add custom fields
		for fieldName, fieldValue := range itemDef.CustomFields {
			document = append(document, azuretools.AddOperation(fieldName, fieldValue))
		}

		createArgs := workitemtracking.CreateWorkItemArgs{
			Type:     &itemDef.Type,
			Project:  &config.Project,
			Document: &document,
		}

		createdWorkItem, err := client.CreateWorkItem(ctx, createArgs)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to create work item '%s' of type '%s': %v", itemDef.Title, itemDef.Type, err)
			results = append(results, map[string]any{"title": itemDef.Title, "type": itemDef.Type, "error": errMsg})
			textResults = append(textResults, errMsg)
			continue
		}

		workItemID := *createdWorkItem.Id
		var parentLinkMsg string

		// If parent ID is provided, create the relationship
		if itemDef.ParentID != "" {
			parentIDInt, convErr := strconv.Atoi(itemDef.ParentID)
			if convErr != nil {
				parentLinkMsg = fmt.Sprintf("Work item #%d created, but failed to link to parent: Invalid parent ID format '%s'", workItemID, itemDef.ParentID)
			} else {
				relationOps := []webapi.JsonPatchOperation{
					{
						Op:   &webapi.OperationValues.Add,
						Path: azuretools.StringPtr("/relations/-"),
						Value: map[string]any{
							"rel": "System.LinkTypes.Hierarchy-Reverse",
							"url": fmt.Sprintf("%s/_apis/wit/workItems/%d", config.OrganizationURL, parentIDInt),
							"attributes": map[string]any{
								"comment": "Linked during creation by MCP",
							},
						},
					},
				}
				updateArgs := workitemtracking.UpdateWorkItemArgs{
					Id:       &workItemID,
					Project:  &config.Project,
					Document: &relationOps,
				}
				_, linkErr := client.UpdateWorkItem(ctx, updateArgs)
				if linkErr != nil {
					parentLinkMsg = fmt.Sprintf("Work item #%d created, but failed to link to parent ID %d: %v", workItemID, parentIDInt, linkErr)
				} else {
					parentLinkMsg = fmt.Sprintf("Work item #%d created and linked to parent ID %d.", workItemID, parentIDInt)
				}
			}
		}

		itemResult := map[string]any{
			"id":      workItemID,
			"title":   azuretools.SafeString((*createdWorkItem.Fields)["System.Title"].(*string)),
			"type":    azuretools.SafeString((*createdWorkItem.Fields)["System.WorkItemType"].(*string)),
			"url":     fmt.Sprintf("%s/_workitems/edit/%d", config.OrganizationURL, workItemID),
			"message": fmt.Sprintf("Successfully created %s #%d.", azuretools.SafeString((*createdWorkItem.Fields)["System.WorkItemType"].(*string)), workItemID),
		}
		if parentLinkMsg != "" {
			itemResult["parent_linking_status"] = parentLinkMsg
		}
		results = append(results, itemResult)

		if format != "json" {
			text := fmt.Sprintf("Successfully created %s #%d: %s. URL: %s/_workitems/edit/%d",
				azuretools.SafeString((*createdWorkItem.Fields)["System.WorkItemType"].(*string)),
				workItemID,
				azuretools.SafeString((*createdWorkItem.Fields)["System.Title"].(*string)),
				config.OrganizationURL, workItemID)
			if parentLinkMsg != "" {
				text += ". " + parentLinkMsg
			}
			textResults = append(textResults, text)
		}
	}

	if strings.ToLower(format) == "json" {
		jsonData, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonData)), nil
	}

	return mcp.NewToolResultText(strings.Join(textResults, "\n---\n")), nil
}

type AzureWorkItemCommentsTool struct {
	tool *mcp.Tool
}

func NewAzureWorkItemCommentsTool() *mcp.Tool {
	tool := mcp.NewTool(
		"azure_work_item_comments",
		mcp.WithDescription("Get comments for a work item in Azure DevOps."),
		mcp.WithString("work_item_id", mcp.Required(), mcp.Description("The ID of the work item to get comments for.")),
	)
	return &tool
}

func (at *AzureWorkItemCommentsTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_WorkItemComments tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureWorkItemCommentsTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps WorkItemComments tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}

type AzureCreateSprintTool struct {
	tool *mcp.Tool
}

func NewAzureCreateSprintTool() *mcp.Tool {
	// This will need to be customized based on the actual tool parameters
	tool := mcp.NewTool(
		"azure_CreateSprint",
		mcp.WithDescription("Azure DevOps CreateSprint tool"),
		// TODO: Add specific parameters for this tool
	)

	return &tool
}

func (at *AzureCreateSprintTool) Handle(
	ctx context.Context, req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	log.Info("azure_CreateSprint tool executing")

	// Get Azure DevOps configuration from environment
	orgName := os.Getenv("AZURE_DEVOPS_ORG")
	pat := os.Getenv("AZDO_PAT")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	team := os.Getenv("AZURE_DEVOPS_TEAM")

	if orgName == "" || pat == "" || project == "" || team == "" {
		return mcp.NewToolResultError("Azure DevOps environment variables not set correctly. Required: AZURE_DEVOPS_ORG, AZDO_PAT, AZURE_DEVOPS_PROJECT, AZURE_DEVOPS_TEAM"), nil
	}

	config := azuretools.AzureDevOpsConfig{
		OrganizationURL:     "https://dev.azure.com/" + orgName,
		PersonalAccessToken: pat,
		Project:             project,
		Team:                team,
	}

	conn := azuredevops.NewPatConnection(config.OrganizationURL, config.PersonalAccessToken)

	// Create and execute the actual Azure tool
	azureTool := azuretools.NewAzureCreateSprintTool(conn, config)
	if azureTool == nil {
		return mcp.NewToolResultError("Failed to initialize Azure DevOps CreateSprint tool"), nil
	}

	// Execute the tool
	return azureTool.Handler(ctx, req)
}
