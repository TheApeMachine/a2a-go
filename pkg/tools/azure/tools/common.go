package tools

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
)

// AzureDevOpsConfig contains configuration for Azure DevOps integration
type AzureDevOpsConfig struct {
	OrganizationURL     string
	PersonalAccessToken string
	Project             string
	Team                string
}

// Helper to extract a string argument.
func GetStringArg(req mcp.CallToolRequest, key string) (string, error) {
	var (
		val any
		str string
		ok  bool
	)

	if val, ok = req.GetArguments()[key]; !ok {
		return "", fmt.Errorf("missing argument: %s", key)
	}

	str, ok = val.(string)

	if !ok {
		return "", fmt.Errorf("argument %s is not a string", key)
	}

	return str, nil
}

// Helper to extract a float64 argument.
func GetFloat64Arg(req mcp.CallToolRequest, key string) (float64, error) {
	var (
		val any
		f   float64
		ok  bool
	)

	if val, ok = req.GetArguments()[key]; !ok {
		return 0, fmt.Errorf("missing argument: %s", key)
	}

	f, ok = val.(float64)

	if !ok {
		return 0, fmt.Errorf("argument %s is not a number", key)
	}

	return f, nil
}

// Helper to extract an int argument from a float64.
func GetIntArg(req mcp.CallToolRequest, key string) (int, error) {
	var (
		f   float64
		err error
	)

	if f, err = GetFloat64Arg(req, key); err != nil {
		return 0, err
	}

	return int(f), nil
}

// Helper for common error formatting
func HandleError(err error, message string) *mcp.CallToolResult {
	return mcp.NewToolResultError(fmt.Sprintf("%s: %v", message, err))
}

// Helper to parse a comma-separated list of IDs
func ParseIDs(idsStr string) ([]int, error) {
	var (
		idStrs []string
		ids    []int
	)

	idStrs = strings.Split(idsStr, ",")

	for _, idStr := range idStrs {
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			return nil, fmt.Errorf("invalid ID format: %s", idStr)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// Format a list of strings for WIQL queries
func FormatStringList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("'%s'", item)
	}
	return strings.Join(quoted, ", ")
}

// Helper function for min value
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper function to create a string pointer
func StringPtr(s string) *string {
	return &s
}

// Helper function to add an operation
func AddOperation(field string, value any) webapi.JsonPatchOperation {
	return webapi.JsonPatchOperation{
		Op:    &webapi.OperationValues.Add,
		Path:  StringPtr("/fields/" + field),
		Value: value,
	}
}

// Relation type mapping
var RelationTypeMap = map[string]string{
	"parent":   "System.LinkTypes.Hierarchy-Reverse",
	"child":    "System.LinkTypes.Hierarchy-Forward",
	"children": "System.LinkTypes.Hierarchy-Forward", // Alias
	"related":  "System.LinkTypes.Related",
}

// Field name mapping
var FieldMap = map[string]string{
	"Title":       "System.Title",
	"Description": "System.Description",
	"State":       "System.State",
	"Priority":    "Microsoft.VSTS.Common.Priority",
}

// GetCurrentSprint uses the Azure DevOps SDK to get current sprint information.
func GetCurrentSprint(ctx context.Context, workClient work.Client, project string, team string) (map[string]any, error) {
	if workClient == nil {
		return nil, fmt.Errorf("workClient is nil")
	}
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	if team == "" {
		return nil, fmt.Errorf("team is required")
	}

	iterations, err := workClient.GetTeamIterations(ctx, work.GetTeamIterationsArgs{
		Project:   &project,
		Team:      &team,
		Timeframe: StringPtr("Current"), // SDK uses string pointer for timeframe
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get current sprint iterations using SDK: %w", err)
	}

	if iterations == nil || len(*iterations) == 0 {
		return nil, fmt.Errorf("no current sprint found for project '%s' and team '%s'", project, team)
	}

	// The first iteration in the "current" timeframe is the current sprint.
	currentSprint := (*iterations)[0]

	// Ensure attributes and their embedded pointers are not nil before dereferencing
	var id, name, path, sprintURL, timeFrame string
	var startDate, endDate time.Time

	if currentSprint.Id != nil {
		id = currentSprint.Id.String() // Assuming Id is a UUID, convert to string
	}
	if currentSprint.Name != nil {
		name = *currentSprint.Name
	}
	if currentSprint.Path != nil {
		path = *currentSprint.Path
	}
	if currentSprint.Url != nil { // This is the API URL of the iteration
		sprintURL = *currentSprint.Url
	}

	// Attributes might contain StartDate and FinishDate
	if currentSprint.Attributes != nil {
		if currentSprint.Attributes.StartDate != nil {
			startDate = currentSprint.Attributes.StartDate.Time
		}
		if currentSprint.Attributes.FinishDate != nil {
			endDate = currentSprint.Attributes.FinishDate.Time
		}
		if currentSprint.Attributes.TimeFrame != nil {
			timeFrame = string(*currentSprint.Attributes.TimeFrame) // TimeFrame is an enum, cast to string
		}
	}

	return map[string]any{
		"id":         id,
		"name":       name,
		"path":       path,
		"start_date": startDate, // Return as time.Time
		"end_date":   endDate,   // Return as time.Time
		"time_frame": timeFrame,
		"url":        sprintURL, // API URL for the sprint iteration
	}, nil
}

// GetWorkItemURL constructs the UI URL for a work item.
func GetWorkItemURL(orgURL string, workItemID int) string {
	return fmt.Sprintf("%s/_workitems/edit/%d", strings.TrimRight(orgURL, "/"), workItemID)
}

// ExtractWorkItemIDFromURL extracts the work item ID from a standard Azure DevOps work item URL.
// Example URL: https://dev.azure.com/org/project/_apis/wit/workItems/123
// or https://org.visualstudio.com/project/_apis/wit/workItems/123
func ExtractWorkItemIDFromURL(itemURL string) (int, error) {
	parsedURL, err := url.Parse(itemURL)
	if err != nil {
		return 0, fmt.Errorf("failed to parse URL '%s': %w", itemURL, err)
	}
	pathSegments := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	// Expected structure: org/project/_apis/wit/workItems/ID or project/_apis/wit/workItems/ID or _apis/wit/workItems/ID
	// We need the last segment if it's a number.
	for i := len(pathSegments) - 1; i >= 0; i-- {
		if pathSegments[i] == "workItems" && i+1 < len(pathSegments) {
			idStr := pathSegments[i+1]
			id, err := strconv.Atoi(idStr)
			if err == nil {
				return id, nil
			}
			// If it's not an int after workItems, it might be a more complex URL or not an ID.
			return 0, fmt.Errorf("could not find numeric ID after 'workItems' segment in URL '%s'", itemURL)
		}
	}
	// Fallback: try to get the last segment if it's numeric, less reliable
	if len(pathSegments) > 0 {
		lastSegment := pathSegments[len(pathSegments)-1]
		id, err := strconv.Atoi(lastSegment)
		if err == nil {
			return id, nil // This assumes the ID is simply the last part of the path
		}
	}
	return 0, fmt.Errorf("could not extract work item ID from URL path '%s'", parsedURL.Path)
}
