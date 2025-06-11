package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// AzureWorkItemCommentsTool provides functionality to manage comments on work items
type AzureWorkItemCommentsTool struct {
	handle mcp.Tool
	client workitemtracking.Client
	config AzureDevOpsConfig
}

// CommentOutput defines the structure for a single comment's output.
type CommentOutput struct {
	ID           int    `json:"id"`
	Text         string `json:"text"`
	CreatedBy    string `json:"created_by"`
	CreatedDate  string `json:"created_date"`
	ModifiedBy   string `json:"modified_by,omitempty"`
	ModifiedDate string `json:"modified_date,omitempty"`
}

// NewAzureWorkItemCommentsTool creates a new tool instance for managing work item comments
func NewAzureWorkItemCommentsTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	client, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		return nil
	}

	tool := &AzureWorkItemCommentsTool{
		client: client,
		config: config,
	}

	tool.handle = mcp.NewTool(
		"azure_work_item_comments",
		mcp.WithDescription("Manage comments on Azure DevOps work items"),
		mcp.WithString(
			"operation",
			mcp.Required(),
			mcp.Description("Operation to perform: 'add' or 'get'"),
			mcp.Enum("add", "get"),
		),
		mcp.WithString(
			"id",
			mcp.Required(),
			mcp.Description("ID of the work item"),
		),
		mcp.WithString(
			"text",
			mcp.Description("Text of the comment to add (required for 'add' operation)"),
		),
		mcp.WithString(
			"format",
			mcp.Description("Response format: 'text' (default) or 'json'"),
			mcp.Enum("text", "json"),
		),
		mcp.WithString(
			"page_size",
			mcp.Description("Number of comments to return (for 'get' operation, default: 10, max: 200)"),
		),
		mcp.WithString(
			"continuation_token",
			mcp.Description("Token to retrieve the next page of comments (for 'get' operation)"),
		),
	)

	return tool
}

func (tool *AzureWorkItemCommentsTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *AzureWorkItemCommentsTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get required parameters
	operation, err := GetStringArg(request, "operation")
	if err != nil {
		return mcp.NewToolResultError(`
Missing "operation" parameter. Please specify the operation to perform.

Valid operations:
- "add": Add a new comment to a work item
- "get": Get comments from a work item
`), nil
	}

	idStr, err := GetStringArg(request, "id")
	if err != nil {
		return mcp.NewToolResultError(`
Missing "id" parameter. Please specify the ID of the work item.

Example: "id": "123"
`), nil
	}

	// Parse work item ID
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid ID format: %s", idStr)), nil
	}

	// Get format if provided
	format, _ := GetStringArg(request, "format")

	// Handle operations
	switch operation {
	case "add":
		return tool.handleAddComment(ctx, request, id, format)
	case "get":
		return tool.handleGetComments(ctx, request, id, format)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Unknown operation: %s", operation)), nil
	}
}

// formatCommentAddResponseToJSON formats the add comment response to JSON.
func formatCommentAddResponseToJSON(workItemID int, orgURL string) (string, error) {
	result := map[string]any{
		"work_item_id": workItemID,
		"success":      true,
		"message":      fmt.Sprintf("Successfully added comment to work item #%d", workItemID),
		"url":          fmt.Sprintf("%s/_workitems/edit/%d", orgURL, workItemID),
	}
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize JSON: %v", err)
	}
	return string(jsonData), nil
}

// formatCommentAddResponseToText formats the add comment response to text.
func formatCommentAddResponseToText(workItemID int, orgURL string) string {
	return fmt.Sprintf("Successfully added comment to work item #%d\n\nURL: %s/_workitems/edit/%d",
		workItemID, orgURL, workItemID)
}

func (tool *AzureWorkItemCommentsTool) handleAddComment(ctx context.Context, request mcp.CallToolRequest, id int, format string) (*mcp.CallToolResult, error) {
	// Get the comment text
	text, err := GetStringArg(request, "text")
	if err != nil {
		return mcp.NewToolResultError(`
Missing "text" parameter. Please provide the text for the comment.

Example: "text": "This is a comment on the work item."
`), nil
	}

	if text == "" {
		return mcp.NewToolResultError("Comment text cannot be empty"), nil
	}

	// Add comment as a discussion by updating the History field
	updateArgs := workitemtracking.UpdateWorkItemArgs{
		Id:      &id,
		Project: &tool.config.Project,
		Document: &[]webapi.JsonPatchOperation{
			AddOperation("System.History", text),
		},
	}

	_, err = tool.client.UpdateWorkItem(ctx, updateArgs)
	if err != nil {
		return HandleError(err, "Failed to add comment"), nil
	}

	// Format response based on requested format
	if strings.ToLower(format) == "json" {
		jsonResponse, err := formatCommentAddResponseToJSON(id, tool.config.OrganizationURL)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(jsonResponse), nil
	}

	// Default text response
	textResponse := formatCommentAddResponseToText(id, tool.config.OrganizationURL)
	return mcp.NewToolResultText(textResponse), nil
}

// formatCommentGetResultsToJSON formats the get comments results to JSON.
func formatCommentGetResultsToJSON(workItemID int, comments []CommentOutput, totalComments, commentsReturned int, nextToken *string, orgURL string) (string, error) {
	responseMap := map[string]any{
		"work_item_id":      workItemID,
		"total_comments":    totalComments,
		"comments_returned": commentsReturned,
		"comments":          comments,
		"url":               fmt.Sprintf("%s/_workitems/edit/%d", orgURL, workItemID),
	}
	if nextToken != nil && *nextToken != "" {
		responseMap["next_continuation_token"] = *nextToken
	}
	jsonData, err := json.MarshalIndent(responseMap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize JSON: %v", err)
	}
	return string(jsonData), nil
}

// formatCommentGetResultsToText formats the get comments results to text.
func formatCommentGetResultsToText(workItemID int, comments []CommentOutput, totalComments, commentsReturned int, nextToken *string, pageSize int, orgURL string) string {
	var resultsText []string
	resultsText = append(resultsText, fmt.Sprintf("## Comments for Work Item #%d\n", workItemID))
	resultsText = append(resultsText, fmt.Sprintf("Total comments available (approx.): %d\n", totalComments))
	resultsText = append(resultsText, fmt.Sprintf("Comments in this page: %d\n", commentsReturned))

	for i, comment := range comments {
		cmtText := fmt.Sprintf("### Comment %d (ID: %d)\n", i+1, comment.ID)
		cmtText += fmt.Sprintf("From: %s\n", comment.CreatedBy)
		cmtText += fmt.Sprintf("Date: %s\n", comment.CreatedDate)
		if comment.ModifiedBy != "" && comment.ModifiedDate != "" {
			cmtText += fmt.Sprintf("Edited by: %s on %s\n", comment.ModifiedBy, comment.ModifiedDate)
		}
		cmtText += fmt.Sprintf("\n%s\n---\n", comment.Text)
		resultsText = append(resultsText, cmtText)
	}

	if nextToken != nil && *nextToken != "" {
		resultsText = append(resultsText, fmt.Sprintf("\nNext page token: `%s`\n", *nextToken))
		resultsText = append(resultsText, fmt.Sprintf("To get next page, use: `{\"operation\": \"get\", \"id\": \"%d\", \"page_size\": \"%d\", \"continuation_token\": \"%s\"}`\n", workItemID, pageSize, *nextToken))
	}
	resultsText = append(resultsText, fmt.Sprintf("\nURL: %s/_workitems/edit/%d", orgURL, workItemID))
	return strings.Join(resultsText, "\n")
}

func (tool *AzureWorkItemCommentsTool) handleGetComments(ctx context.Context, request mcp.CallToolRequest, id int, format string) (*mcp.CallToolResult, error) {
	// Get page size if provided
	pageSizeStr, _ := GetStringArg(request, "page_size")
	pageSize := 10 // Default page size
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
			if pageSize > 200 { // Max page size as per Azure DevOps API for comments
				pageSize = 200
			}
		}
	}

	// Get continuation token if provided
	continuationTokenStr, _ := GetStringArg(request, "continuation_token")
	var continuationToken *string
	if continuationTokenStr != "" {
		continuationToken = &continuationTokenStr
	}

	// Get comments for the work item using server-side pagination
	commentsResult, err := tool.client.GetComments(ctx, workitemtracking.GetCommentsArgs{
		Project:           &tool.config.Project,
		WorkItemId:        &id,
		Top:               &pageSize,
		ContinuationToken: continuationToken,
	})

	if err != nil {
		return HandleError(err, "Failed to get comments"), nil
	}

	// Handle case with no comments
	if commentsResult == nil || commentsResult.Comments == nil || len(*commentsResult.Comments) == 0 {
		if strings.ToLower(format) == "json" {
			// Use the new formatter for consistency, even for no comments
			jsonResponse, err := formatCommentGetResultsToJSON(id, []CommentOutput{}, 0, 0, nil, tool.config.OrganizationURL)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON for empty result: %v", err)), nil
			}
			return mcp.NewToolResultText(jsonResponse), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("No comments found for work item #%d", id)), nil
	}

	rawCommentList := *commentsResult.Comments
	nextContinuationToken := commentsResult.ContinuationToken
	totalComments := 0
	if commentsResult.TotalCount != nil { // TotalCount is a pointer
		totalComments = *commentsResult.TotalCount
	}

	outputComments := []CommentOutput{}
	for _, rawComment := range rawCommentList {
		comment := CommentOutput{
			ID: *rawComment.Id,
		}
		if rawComment.Text != nil { // Ensure text is not nil
			comment.Text = *rawComment.Text
		} else {
			comment.Text = "[Comment text not available]"
		}
		if rawComment.CreatedBy != nil && rawComment.CreatedBy.DisplayName != nil {
			comment.CreatedBy = *rawComment.CreatedBy.DisplayName
		}
		if rawComment.CreatedDate != nil {
			comment.CreatedDate = rawComment.CreatedDate.String()
		}
		if rawComment.ModifiedBy != nil && rawComment.ModifiedBy.DisplayName != nil {
			comment.ModifiedBy = *rawComment.ModifiedBy.DisplayName
		}
		if rawComment.ModifiedDate != nil {
			comment.ModifiedDate = rawComment.ModifiedDate.String()
		}
		outputComments = append(outputComments, comment)
	}

	// Format response based on requested format
	if strings.ToLower(format) == "json" {
		jsonResponse, err := formatCommentGetResultsToJSON(id, outputComments, totalComments, len(outputComments), nextContinuationToken, tool.config.OrganizationURL)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil // Already formatted by the helper
		}
		return mcp.NewToolResultText(jsonResponse), nil
	}

	// Default text format
	textResponse := formatCommentGetResultsToText(id, outputComments, totalComments, len(outputComments), nextContinuationToken, pageSize, tool.config.OrganizationURL)
	return mcp.NewToolResultText(textResponse), nil
}
