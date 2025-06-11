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

// AzureCreateWorkItemsTool provides functionality to create new work items in bulk with custom fields.
type AzureCreateWorkItemsTool struct {
	handle mcp.Tool
	client workitemtracking.Client
	config AzureDevOpsConfig
}

// WorkItemDefinition is used to parse the JSON input for each work item to be created.
type WorkItemDefinition struct {
	Type         string            `json:"type"`
	Title        string            `json:"title"`
	Description  string            `json:"description,omitempty"`
	State        string            `json:"state,omitempty"`
	Priority     string            `json:"priority,omitempty"`
	ParentID     string            `json:"parent_id,omitempty"`
	AssignedTo   string            `json:"assigned_to,omitempty"`
	Iteration    string            `json:"iteration,omitempty"`
	Area         string            `json:"area,omitempty"`
	Tags         string            `json:"tags,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// NewAzureCreateWorkItemsTool creates a new tool instance for creating work items.
func NewAzureCreateWorkItemsTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	client, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating workitemtracking client for AzureCreateWorkItemsTool: %v\n", err)
		return nil
	}

	tool := &AzureCreateWorkItemsTool{
		client: client,
		config: config,
	}

	tool.handle = mcp.NewTool(
		"azure_create_work_items",
		mcp.WithDescription("Create one or more new work items in Azure DevOps, with support for custom fields and parent linking."),
		mcp.WithString(
			"items_json",
			mcp.Required(),
			mcp.Description("A JSON string representing an array of work items to create. Each item object should define 'type', 'title', and optionally 'description' (using HTML, not Markdown), 'state', 'priority', 'parent_id', 'assigned_to', 'iteration', 'area', 'tags', and 'custom_fields' (as a map)."),
		),
		mcp.WithString("format", mcp.Description("Response format: 'text' (default) or 'json'")),
	)

	return tool
}

func (tool *AzureCreateWorkItemsTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *AzureCreateWorkItemsTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	itemsJSON, err := GetStringArg(request, "items_json")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: items_json."), nil
	}
	format, _ := GetStringArg(request, "format")

	var itemsToCreate []WorkItemDefinition
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
			AddOperation("System.Title", itemDef.Title),
		}

		if itemDef.Description != "" {
			document = append(document, AddOperation("System.Description", itemDef.Description))
		}
		if itemDef.State != "" {
			document = append(document, AddOperation("System.State", itemDef.State))
		}
		if itemDef.Priority != "" {
			// Ensure priority is a number if the field expects it. ADO priorities are typically ints 1,2,3,4.
			// For safety, assume string is fine based on previous tool, but this might need adjustment.
			document = append(document, AddOperation("Microsoft.VSTS.Common.Priority", itemDef.Priority))
		}
		if itemDef.AssignedTo != "" {
			document = append(document, AddOperation("System.AssignedTo", itemDef.AssignedTo))
		}
		if itemDef.Iteration != "" {
			document = append(document, AddOperation("System.IterationPath", itemDef.Iteration))
		}
		if itemDef.Area != "" {
			document = append(document, AddOperation("System.AreaPath", itemDef.Area))
		}
		if itemDef.Tags != "" {
			document = append(document, AddOperation("System.Tags", itemDef.Tags))
		}

		// Add custom fields
		for fieldName, fieldValue := range itemDef.CustomFields {
			// Custom fields might need their full reference name e.g., "Custom.MyField"
			// For now, assume the user provides the correct reference name.
			document = append(document, AddOperation(fieldName, fieldValue))
		}

		createArgs := workitemtracking.CreateWorkItemArgs{
			Type:     &itemDef.Type,
			Project:  &tool.config.Project,
			Document: &document,
		}

		createdWorkItem, err := tool.client.CreateWorkItem(ctx, createArgs)
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
						Path: StringPtr("/relations/-"),
						Value: map[string]any{
							"rel": "System.LinkTypes.Hierarchy-Reverse", // Parent link
							"url": fmt.Sprintf("%s/_apis/wit/workItems/%d", tool.config.OrganizationURL, parentIDInt),
							"attributes": map[string]any{
								"comment": "Linked during creation by MCP",
							},
						},
					},
				}
				updateArgs := workitemtracking.UpdateWorkItemArgs{
					Id:       &workItemID,
					Project:  &tool.config.Project,
					Document: &relationOps,
				}
				_, linkErr := tool.client.UpdateWorkItem(ctx, updateArgs)
				if linkErr != nil {
					parentLinkMsg = fmt.Sprintf("Work item #%d created, but failed to link to parent ID %d: %v", workItemID, parentIDInt, linkErr)
				} else {
					parentLinkMsg = fmt.Sprintf("Work item #%d created and linked to parent ID %d.", workItemID, parentIDInt)
				}
			}
			if format == "json" {
				// This message might be too verbose for JSON, handled in summary below
			} else {
				textResults = append(textResults, parentLinkMsg)
			}
		}

		itemResult := map[string]any{
			"id":      workItemID,
			"title":   SafeString((*createdWorkItem.Fields)["System.Title"].(*string)),
			"type":    SafeString((*createdWorkItem.Fields)["System.WorkItemType"].(*string)),
			"url":     fmt.Sprintf("%s/_workitems/edit/%d", tool.config.OrganizationURL, workItemID),
			"message": fmt.Sprintf("Successfully created %s #%d.", SafeString((*createdWorkItem.Fields)["System.WorkItemType"].(*string)), workItemID),
		}
		if parentLinkMsg != "" { // Add parent linking outcome to JSON if it happened
			itemResult["parent_linking_status"] = parentLinkMsg
		}
		results = append(results, itemResult)

		if format != "json" {
			text := fmt.Sprintf("Successfully created %s #%d: %s. URL: %s/_workitems/edit/%d",
				SafeString((*createdWorkItem.Fields)["System.WorkItemType"].(*string)),
				workItemID,
				SafeString((*createdWorkItem.Fields)["System.Title"].(*string)),
				tool.config.OrganizationURL, workItemID)
			if parentLinkMsg != "" && !strings.HasPrefix(parentLinkMsg, "Work item #"+strconv.Itoa(workItemID)+" created and linked") {
				// Append only if it's an error or a separate status for linking
				text += ". " + parentLinkMsg
			} else if parentLinkMsg != "" {
				// If linking was successful and message already implies creation.
			}

			textResults = append(textResults, text)
		}
	} // End of loop for itemsToCreate

	if strings.ToLower(format) == "json" {
		jsonData, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize JSON response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonData)), nil
	}

	return mcp.NewToolResultText(strings.Join(textResults, "\n---\n")), nil
}

// GetStringArg, AddOperation, StringPtr would typically be in tools/common.go
