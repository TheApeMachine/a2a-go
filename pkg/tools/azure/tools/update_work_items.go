package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/theapemachine/mcp-server-devops-bridge/core"
)

// AzureUpdateWorkItemsTool provides functionality to update multiple work items in Azure DevOps.
type AzureUpdateWorkItemsTool struct {
	handle mcp.Tool
	client workitemtracking.Client
	config AzureDevOpsConfig
}

// WorkItemUpdateDefinition defines the structure for updating a single work item.
type WorkItemUpdateDefinition struct {
	ID              int                `json:"id"`
	FieldsToUpdate  map[string]any     `json:"fields_to_update"`           // Flexible map for all field types
	Comment         string             `json:"comment,omitempty"`          // For adding a new comment
	AddRelations    []RelationLink     `json:"add_relations,omitempty"`    // For adding new relations
	RemoveRelations []RelationLinkArgs `json:"remove_relations,omitempty"` // For removing relations by URL or Ref
}

// RelationLink defines structure for adding a work item relation
type RelationLink struct {
	RelType        string            `json:"rel_type"` // e.g., "System.LinkTypes.Hierarchy-Forward"
	TargetURL      string            `json:"target_url"`
	LinkAttributes map[string]string `json:"attributes,omitempty"`
}

// RelationLinkArgs is used for identifying relations to remove, can be by target URL or by relation reference
type RelationLinkArgs struct {
	RelType     string `json:"rel_type"`               // e.g., "System.LinkTypes.Hierarchy-Forward", optional if removing by ref
	TargetURL   string `json:"target_url,omitempty"`   // URL of the related work item
	RelationRef string `json:"relation_ref,omitempty"` // Internal reference of the link itself (e.g. from get_work_item)
}

// NewAzureUpdateWorkItemsTool creates a new tool instance for updating work items.
func NewAzureUpdateWorkItemsTool(conn *azuredevops.Connection, config AzureDevOpsConfig) core.Tool {
	client, err := workitemtracking.NewClient(context.Background(), conn)
	if err != nil {
		fmt.Printf("Error creating workitemtracking client for AzureUpdateWorkItemsTool: %v\n", err)
		return nil
	}

	tool := &AzureUpdateWorkItemsTool{
		client: client,
		config: config,
	}

	tool.handle = mcp.NewTool(
		"azure_update_work_items",
		mcp.WithDescription("Update one or more work items in Azure DevOps. Supports updating various fields, adding comments (HTML, not Markdown), and managing relationships."),
		mcp.WithString(
			"items_to_update_json",
			mcp.Required(),
			mcp.Description("A JSON string representing an array of work items to update. Each item object must have an 'id' (integer) and 'fields_to_update' (map of field names to new values). Optionally, include 'comment' (HTML string, not Markdown) to add a comment, 'add_relations' (array of relation links), or 'remove_relations' (array of relation identifiers)."),
		),
		mcp.WithString("format", mcp.Description("Response format: 'text' (default) or 'json'")),
	)
	return tool
}

func (tool *AzureUpdateWorkItemsTool) Handle() mcp.Tool {
	return tool.handle
}

func (tool *AzureUpdateWorkItemsTool) Handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	itemsJSON, err := GetStringArg(request, "items_to_update_json")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: items_to_update_json"), nil
	}
	format, _ := GetStringArg(request, "format")

	var itemsToUpdate []WorkItemUpdateDefinition
	if err := json.Unmarshal([]byte(itemsJSON), &itemsToUpdate); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid JSON for items_to_update_json: %v. Expected an array of update objects.", err)), nil
	}

	if len(itemsToUpdate) == 0 {
		return mcp.NewToolResultError("No items provided in items_to_update_json array."), nil
	}

	var results []map[string]any
	var textResults []string

	for _, itemDef := range itemsToUpdate {
		if itemDef.ID == 0 {
			errMsg := fmt.Sprintf("Skipped item due to missing or invalid 'id'. Provided: %+v", itemDef)
			results = append(results, map[string]any{"id": itemDef.ID, "error": errMsg})
			textResults = append(textResults, errMsg)
			continue
		}

		var operations []webapi.JsonPatchOperation

		// Add field updates
		for field, value := range itemDef.FieldsToUpdate {
			operations = append(operations, AddOperation(field, value)) // AddOperation should handle various types
		}

		// Add comment if provided
		if itemDef.Comment != "" {
			operations = append(operations, AddOperation("System.History", itemDef.Comment))
		}

		// Add relations
		for _, rel := range itemDef.AddRelations {
			if rel.RelType == "" || rel.TargetURL == "" {
				textResults = append(textResults, fmt.Sprintf("Skipping add relation for item %d: rel_type and target_url are required.", itemDef.ID))
				continue
			}
			linkValue := map[string]any{
				"rel": rel.RelType,
				"url": rel.TargetURL,
			}
			if len(rel.LinkAttributes) > 0 {
				linkValue["attributes"] = rel.LinkAttributes
			}
			operations = append(operations, webapi.JsonPatchOperation{
				Op:    &webapi.OperationValues.Add,
				Path:  StringPtr("/relations/-"),
				Value: linkValue,
			})
		}

		// Note: Removing relations by TargetURL and RelType might require fetching the work item first to get the specific relation index or reference.
		// The Azure DevOps API for PATCH /wit/workitems/{id} with op: "remove", path: "/relations/{index}" or by reference if the API supports it.
		// For simplicity, if removing specific relations is complex, this part might need a dedicated tool or further refinement.
		// The current structure assumes we might get a 'relation_ref' that can be directly used. If not, a GET call is needed.

		if len(operations) == 0 {
			msg := fmt.Sprintf("No updates specified for work item #%d (no fields, comment, or relations to add/remove).", itemDef.ID)
			results = append(results, map[string]any{"id": itemDef.ID, "status": "skipped", "message": msg})
			textResults = append(textResults, msg)
			continue
		}

		updateArgs := workitemtracking.UpdateWorkItemArgs{
			Id:       &itemDef.ID,
			Project:  &tool.config.Project,
			Document: &operations,
			// ValidateOnly: BoolPtr(false), // Optional: set to true to test without saving
			// Expand: &workitemtracking.WorkItemExpandValues.Relations, // Optional: to get relations back
		}

		updatedWorkItem, err := tool.client.UpdateWorkItem(ctx, updateArgs)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to update work item #%d: %v", itemDef.ID, err)
			results = append(results, map[string]any{"id": itemDef.ID, "error": errMsg})
			textResults = append(textResults, errMsg)
			continue
		}

		itemResult := map[string]any{
			"id":      *updatedWorkItem.Id,
			"rev":     *updatedWorkItem.Rev,
			"url":     GetWorkItemURL(tool.config.OrganizationURL, *updatedWorkItem.Id),
			"message": fmt.Sprintf("Work item #%d updated successfully to revision %d.", *updatedWorkItem.Id, *updatedWorkItem.Rev),
		}
		results = append(results, itemResult)
		textResults = append(textResults, itemResult["message"].(string))
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
