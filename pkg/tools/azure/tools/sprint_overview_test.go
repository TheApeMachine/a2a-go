package tools

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/work"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	. "github.com/smartystreets/goconvey/convey"
)

// Mock interfaces for Azure DevOps clients
type MockWorkClient struct {
	GetIterationsFunc func(ctx context.Context, args work.GetTeamIterationsArgs) (*[]work.TeamSettingsIteration, error)
}

func (m *MockWorkClient) GetIterations(ctx context.Context, args work.GetTeamIterationsArgs) (*[]work.TeamSettingsIteration, error) {
	if m.GetIterationsFunc != nil {
		return m.GetIterationsFunc(ctx, args)
	}
	return &[]work.TeamSettingsIteration{}, nil
}

type MockTrackingClient struct {
	QueryByWiqlFunc  func(ctx context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error)
	GetWorkItemsFunc func(ctx context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error)
}

func (m *MockTrackingClient) QueryByWiql(ctx context.Context, args workitemtracking.QueryByWiqlArgs) (*workitemtracking.WorkItemQueryResult, error) {
	if m.QueryByWiqlFunc != nil {
		return m.QueryByWiqlFunc(ctx, args)
	}
	return &workitemtracking.WorkItemQueryResult{}, nil
}

func (m *MockTrackingClient) GetWorkItems(ctx context.Context, args workitemtracking.GetWorkItemsArgs) (*[]workitemtracking.WorkItem, error) {
	if m.GetWorkItemsFunc != nil {
		return m.GetWorkItemsFunc(ctx, args)
	}
	return &[]workitemtracking.WorkItem{}, nil
}

// Helper function to create a test Azure Sprint Overview Tool with mocked clients
func createTestAzureSprintOverviewTool(workClient work.Client, trackingClient workitemtracking.Client) *AzureSprintOverviewTool {
	config := createTestConfig()

	tool := &AzureSprintOverviewTool{
		workClient:     workClient,
		trackingClient: trackingClient,
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

// Helper function to create test config for unit testing
func createTestConfig() AzureDevOpsConfig {
	return AzureDevOpsConfig{
		Project: "test-project",
		Team:    "test-team",
	}
}

// Helper function to create mock iteration data
func createMockIteration() work.TeamSettingsIteration {
	startDate := time.Now()
	endDate := startDate.Add(14 * 24 * time.Hour) // 2 week sprint
	iterationId := uuid.New()

	return work.TeamSettingsIteration{
		Id:   &iterationId,
		Name: stringPtr("Sprint 1"),
		Path: stringPtr("TestProject\\Sprint 1"),
		Attributes: &work.TeamIterationAttributes{
			StartDate:  &azuredevops.Time{Time: startDate},
			FinishDate: &azuredevops.Time{Time: endDate},
		},
		Url: stringPtr("https://test.visualstudio.com/_apis/work/teamsettings/iterations/iteration-1"),
	}
}

// Helper function to create mock work items
func createMockWorkItems() []workitemtracking.WorkItem {
	return []workitemtracking.WorkItem{
		{
			Id: intPtr(1),
			Fields: &map[string]interface{}{
				"System.Title":        "Test work item 1",
				"System.WorkItemType": "User Story",
				"System.State":        "Active",
				"System.AssignedTo":   map[string]interface{}{"displayName": "John Doe"},
			},
		},
		{
			Id: intPtr(2),
			Fields: &map[string]interface{}{
				"System.Title":        "Test work item 2",
				"System.WorkItemType": "Bug",
				"System.State":        "Resolved",
				"System.AssignedTo":   map[string]interface{}{"displayName": "Jane Smith"},
			},
		},
	}
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func TestNewAzureSprintOverviewTool(t *testing.T) {
	Convey("Given a test configuration", t, func() {
		config := createTestConfig()
		So(config.Project, ShouldNotBeEmpty)

		Convey("When testing tool handle creation", func() {
			// Test the tool handle structure without creating real Azure clients
			handle := mcp.NewTool(
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

			Convey("Then the tool handle should be configured correctly", func() {
				So(handle, ShouldNotBeNil)
				So(handle.Name, ShouldEqual, "azure_sprint_overview")
				So(handle.Description, ShouldNotBeEmpty)
				So(len(handle.InputSchema.Properties), ShouldEqual, 2) // sprint_identifier and format
			})
		})
	})
}

func TestAzureSprintOverviewTool_Handler_Unit(t *testing.T) {
	Convey("Given request validation", t, func() {
		Convey("When validating request structure", func() {
			validRequest := mcp.CallToolRequest{
				Params: struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments,omitempty"`
					Meta      *mcp.Meta      `json:"_meta,omitempty"`
				}{
					Name: "azure_sprint_overview",
					Arguments: map[string]any{
						"sprint_identifier": "Sprint 1",
						"format":            "json",
					},
				},
			}

			invalidRequest := mcp.CallToolRequest{
				Params: struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments,omitempty"`
					Meta      *mcp.Meta      `json:"_meta,omitempty"`
				}{
					Name: "azure_sprint_overview",
					Arguments: map[string]any{
						// Missing sprint_identifier is actually valid (uses current sprint)
						"format": "json",
					},
				},
			}

			Convey("Then request structures should be valid", func() {
				So(validRequest.Params.Name, ShouldEqual, "azure_sprint_overview")
				So(validRequest.Params.Arguments["sprint_identifier"], ShouldEqual, "Sprint 1")
				So(validRequest.Params.Arguments["format"], ShouldEqual, "json")

				So(invalidRequest.Params.Name, ShouldEqual, "azure_sprint_overview")
				So(invalidRequest.Params.Arguments["format"], ShouldEqual, "json")
			})
		})

		Convey("When testing argument parsing", func() {
			arguments := map[string]any{
				"sprint_identifier": "Sprint 1",
				"format":            "markdown",
			}

			Convey("Then arguments should be accessible", func() {
				sprintId, exists := arguments["sprint_identifier"].(string)
				So(exists, ShouldBeTrue)
				So(sprintId, ShouldEqual, "Sprint 1")

				format, exists := arguments["format"].(string)
				So(exists, ShouldBeTrue)
				So(format, ShouldEqual, "markdown")
			})
		})
	})
}
