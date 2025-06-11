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

// DetailedWorkItemRelationOutput defines the structure for work item relations for output.
type DetailedWorkItemRelationOutput struct {
	Parents  []int `json:"parents,omitempty"`
	Children []int `json:"children,omitempty"`
	Related  []int `json:"related,omitempty"`
	// We can add more specific relation types if needed, e.g., Predecessor, Successor
}

// DetailedWorkItemOutput defines the structure for detailed work item information for output.
type DetailedWorkItemOutput struct {
	ID        int                             `json:"id"`
	URL       string                          `json:"url"`
	Fields    map[string]any                  `json:"fields"` // All raw fields, includes Tags if fetched
	Relations *DetailedWorkItemRelationOutput `json:"relations,omitempty"`
	Comments  []string                        `json:"comments,omitempty"`
	Tags      string                          `json:"tags,omitempty"` // Extracted for convenience

	// Specific, commonly used fields for text formatting, can be extracted from Fields
	Title         string `json:"-"`
	Type          string `json:"-"`
	State         string `json:"-"`
	AssignedTo    string `json:"-"`
	IterationPath string `json:"-"`
	AreaPath      string `json:"-"`
	Description   string `json:"-"`
	// Tags      string `json:"-"` // Already defined above for JSON output as well
}

// AzureGetWorkItemsTool provides functionality to get work item details
type AzureGetWorkItemsTool struct { // Renamed from AzureGetWorkItemTool
	handle mcp.Tool
	client workitemtracking.Client
	config AzureDevOpsConfig
}

// NewAzureGetWorkItemsTool creates a new tool instance for getting work item details
func NewAzureGetWorkItemsTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool { // Renamed
	client, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating workitemtracking client for NewAzureGetWorkItemsTool: %v\n", err)
		return nil
	}

	tool := &AzureGetWorkItemsTool{ // Renamed
		client: client,
		config: config,
	}

	tool.handle = mcp.NewTool(
		"azure_get_work_items",
		mcp.WithDescription("Get detailed information about one or more work items in Azure DevOps, including fields, tags, relations, and comments."),
		mcp.WithString(
			"ids",
			mcp.Required(),
			mcp.Description("Comma-separated list of work item IDs (e.g., '123,456,789')."),
		),
		mcp.WithString(
			"include_relations",
			mcp.Description("Whether to include relations (parent, child, related). Default: true"),
			mcp.Enum("true", "false"),
		),
		mcp.WithString(
			"include_comments",
			mcp.Description("Whether to include comments. Default: true"),
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

func (tool *AzureGetWorkItemsTool) Handle() mcp.Tool {
	return tool.handle
}

// formatDetailedWorkItemsToJSON formats the detailed work items into a JSON string.
func formatDetailedWorkItemsToJSON(items []DetailedWorkItemOutput) (string, error) {
	jsonData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize JSON: %v", err)
	}
	return string(jsonData), nil
}

// formatDetailedWorkItemsToText formats the detailed work items into a text string.
func formatDetailedWorkItemsToText(items []DetailedWorkItemOutput) string {
	var results []string
	results = append(results, "## Work Item Details\n")

	for _, item := range items {
		result := fmt.Sprintf("### [%d] %s\n", item.ID, item.Title)
		result += fmt.Sprintf("Type: %s | State: %s\n", item.Type, item.State)

		if item.AssignedTo != "" {
			result += fmt.Sprintf("Assigned to: %s\n", item.AssignedTo)
		}
		if item.IterationPath != "" {
			result += fmt.Sprintf("Iteration: %s\n", item.IterationPath)
		}
		if item.AreaPath != "" {
			result += fmt.Sprintf("Area: %s\n", item.AreaPath)
		}
		if item.Tags != "" { // Check if Tags field is populated
			result += fmt.Sprintf("Tags: %s\n", item.Tags)
		}
		result += fmt.Sprintf("URL: %s\n", item.URL)

		if item.Relations != nil {
			result += "\nLinked Items:\n"
			if len(item.Relations.Parents) > 0 {
				result += fmt.Sprintf("  Parent IDs: %s\n", strings.Join(intSliceToStringSlice(item.Relations.Parents), ", "))
			}
			if len(item.Relations.Children) > 0 {
				result += fmt.Sprintf("  Child IDs: %s\n", strings.Join(intSliceToStringSlice(item.Relations.Children), ", "))
			}
			if len(item.Relations.Related) > 0 {
				result += fmt.Sprintf("  Related IDs: %s\n", strings.Join(intSliceToStringSlice(item.Relations.Related), ", "))
			}
		}

		if item.Description != "" {
			result += fmt.Sprintf("\nDescription:\n%s\n", item.Description)
		}

		if len(item.Comments) > 0 {
			result += "\nComments:\n"
			for i, comment := range item.Comments {
				result += fmt.Sprintf("--- Comment %d ---\n%s\n", i+1, comment)
			}
		}
		result += "---\n"
		results = append(results, result)
	}
	return strings.Join(results, "\n")
}

// Helper function to convert int slice to string slice for joining
func intSliceToStringSlice(intSlice []int) []string {
	strSlice := make([]string, len(intSlice))
	for i, v := range intSlice {
		strSlice[i] = strconv.Itoa(v)
	}
	return strSlice
}

func (tool *AzureGetWorkItemsTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idsStr, err := GetStringArg(request, "ids")
	if err != nil {
		return mcp.NewToolResultError("Missing \"ids\" parameter. Provide comma-separated work item IDs."), nil
	}

	parsedIDs, err := ParseIDs(idsStr)
	if err != nil || len(parsedIDs) == 0 {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid or empty IDs provided: '%s'. Error: %v", idsStr, err)), nil
	}

	includeRelationsStr, _ := GetStringArg(request, "include_relations")
	includeCommentsStr, _ := GetStringArg(request, "include_comments")
	format, _ := GetStringArg(request, "format")

	includeRelations := true // Default to true as per new description
	if includeRelationsStr == "false" {
		includeRelations = false
	}

	includeComments := true // Default to true as per new description
	if includeCommentsStr == "false" {
		includeComments = false
	}

	var finalFieldsToFetch *[]string
	expandValue := workitemtracking.WorkItemExpandValues.None

	if includeRelations {
		expandValue = workitemtracking.WorkItemExpandValues.All // Fetch all, including relations and links
		finalFieldsToFetch = nil                                // Do not specify fields when expand is All
	} else {
		// When not expanding relations, explicitly fetch desired fields.
		// Expand.Fields might be an option if only fields are needed without relations.
		// However, to be safe and explicit, using Expand.None and specifying fields is clearer.
		expandValue = workitemtracking.WorkItemExpandValues.None
		fieldsToFetch := &[]string{
			"System.Id", "System.Title", "System.WorkItemType", "System.State",
			"System.AssignedTo", "System.IterationPath", "System.AreaPath",
			"System.Description", "System.Tags", "System.CreatedDate", "System.CreatedBy",
			"System.ChangedDate", "System.ChangedBy", "Microsoft.VSTS.Common.Priority",
			"Microsoft.VSTS.Common.Severity",
		}
		finalFieldsToFetch = fieldsToFetch
	}

	workItems, err := tool.client.GetWorkItems(ctx, workitemtracking.GetWorkItemsArgs{
		Ids:     &parsedIDs,
		Project: &tool.config.Project,
		Expand:  &expandValue,
		Fields:  finalFieldsToFetch,
	})

	if err != nil {
		return HandleError(err, "Failed to get work items"), nil
	}

	if workItems == nil || len(*workItems) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No work items found for IDs: %s", idsStr)), nil
	}

	commentsMap := make(map[int][]string)
	if includeComments {
		// GetCommentsBatch API might be limited or less efficient for many items.
		// The workitem expand might already bring some comment data or latest N comments if API supports.
		// For now, using GetCommentsBatch as it was.
		commentsBatchResult, err := tool.client.GetCommentsBatch(ctx, workitemtracking.GetCommentsBatchArgs{
			Project: &tool.config.Project,
			Ids:     &parsedIDs, // Max 200 IDs for GetCommentsBatch
		})
		if err != nil {
			// Log error but continue, comments might be missing for some.
			fmt.Printf("Warning: Failed to get comments batch: %v\n", err)
		} else if commentsBatchResult != nil && commentsBatchResult.Comments != nil {
			for _, comment := range *commentsBatchResult.Comments {
				if comment.WorkItemId == nil || comment.Text == nil {
					continue
				}
				workItemID := *comment.WorkItemId
				author := "Unknown" // Default if CreatedBy or DisplayName is nil
				if comment.CreatedBy != nil && comment.CreatedBy.DisplayName != nil {
					author = *comment.CreatedBy.DisplayName
				}
				dateStr := "Unknown Date"
				if comment.CreatedDate != nil {
					dateStr = comment.CreatedDate.Time.Format("2006-01-02 15:04")
				}
				formattedComment := fmt.Sprintf("Author: %s | Date: %s\n%s", author, dateStr, *comment.Text)
				commentsMap[workItemID] = append(commentsMap[workItemID], formattedComment)
			}
		}
	}

	outputItems := []DetailedWorkItemOutput{}
	for _, item := range *workItems {
		if item.Id == nil || item.Fields == nil {
			continue
		}
		fields := *item.Fields
		id := *item.Id

		outputItem := DetailedWorkItemOutput{
			ID:     id,
			URL:    GetWorkItemURL(tool.config.OrganizationURL, id), // Use common GetWorkItemURL
			Fields: fields,
		}

		if title, ok := fields["System.Title"].(string); ok {
			outputItem.Title = title
		}
		if typ, ok := fields["System.WorkItemType"].(string); ok {
			outputItem.Type = typ
		}
		if st, ok := fields["System.State"].(string); ok {
			outputItem.State = st
		}
		if desc, ok := fields["System.Description"].(string); ok {
			outputItem.Description = desc // Keep raw HTML, text formatting can strip/simplify if needed
		}
		if assignedToMap, ok := fields["System.AssignedTo"].(map[string]any); ok {
			if displayName, ok := assignedToMap["displayName"].(string); ok {
				outputItem.AssignedTo = displayName
			}
		}
		if iterPath, ok := fields["System.IterationPath"].(string); ok {
			outputItem.IterationPath = iterPath
		}
		if areaP, ok := fields["System.AreaPath"].(string); ok {
			outputItem.AreaPath = areaP
		}
		if tagStr, ok := fields["System.Tags"].(string); ok {
			outputItem.Tags = tagStr // Populate for JSON and text
		}

		if includeRelations && item.Relations != nil && len(*item.Relations) > 0 {
			relationsOutput := DetailedWorkItemRelationOutput{Parents: []int{}, Children: []int{}, Related: []int{}}
			for _, relation := range *item.Relations {
				if relation.Url == nil || relation.Rel == nil {
					continue
				}
				relatedID, relErr := ExtractWorkItemIDFromURL(*relation.Url)
				if relErr != nil {
					fmt.Printf("Warning: Failed to parse related work item ID from URL '%s': %v\n", *relation.Url, relErr)
					continue
				}
				switch *relation.Rel {
				case "System.LinkTypes.Hierarchy-Reverse":
					relationsOutput.Parents = append(relationsOutput.Parents, relatedID)
				case "System.LinkTypes.Hierarchy-Forward":
					relationsOutput.Children = append(relationsOutput.Children, relatedID)
				case "System.LinkTypes.Related": // This is a common one, but there are others
					relationsOutput.Related = append(relationsOutput.Related, relatedID)
					// Could add more specific link types here if necessary
				}
			}
			if len(relationsOutput.Parents) > 0 || len(relationsOutput.Children) > 0 || len(relationsOutput.Related) > 0 {
				outputItem.Relations = &relationsOutput
			}
		}

		if includeComments {
			if comments, ok := commentsMap[id]; ok {
				outputItem.Comments = comments
			}
		}
		outputItems = append(outputItems, outputItem)
	}

	if strings.ToLower(format) == "json" {
		jsonString, err := formatDetailedWorkItemsToJSON(outputItems)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(jsonString), nil
	}

	textString := formatDetailedWorkItemsToText(outputItems)
	return mcp.NewToolResultText(textString), nil
}

// Ensure ParseIDs, GetStringArg, HandleError, GetWorkItemURL, ExtractWorkItemIDFromURL
// are defined in common.go or accessible.
