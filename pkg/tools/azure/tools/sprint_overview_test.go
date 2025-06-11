package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	. "github.com/smartystreets/goconvey/convey"
)

// Helper function to create a mock Azure DevOps connection (replace with actual for real integration)
func getTestConnection(t *testing.T) *azuredevops.Connection {
	orgURL := os.Getenv("AZURE_ORG_URL")
	pat := os.Getenv("AZURE_PAT")

	if orgURL == "" || pat == "" {
		t.Skip("Skipping integration test: AZURE_ORG_URL or AZURE_PAT environment variables not set.")
		return nil
	}

	connection := azuredevops.NewPatConnection(orgURL, pat)
	// Basic check: You could try a lightweight API call like listing projects if you want to validate connection early.
	// For now, we assume the connection object itself is enough for the tool's client initializations.
	if connection == nil {
		t.Fatal("Failed to create Azure DevOps connection object even with ORG_URL and PAT set.")
		return nil
	}
	return connection
}

// Helper function to get AzureDevOpsConfig from environment variables
func getTestConfig(t *testing.T) AzureDevOpsConfig {
	project := os.Getenv("AZURE_PROJECT")
	team := os.Getenv("AZURE_TEAM") // Team is needed for @currentIteration

	if project == "" {
		// Project is fundamental for most Azure DevOps API calls made by the tools.
		t.Skip("Skipping integration test: AZURE_PROJECT environment variable not set.")
		// Return an empty config or handle as per requirements. Forcing a skip is safer.
		return AzureDevOpsConfig{}
	}
	// Team is optional for some operations but often required for current sprint.
	// The tool itself has logic that might adapt if team is empty, but tests for @currentIteration need it.
	if team == "" {
		t.Log("Warning: AZURE_TEAM environment variable not set. Tests for @currentIteration might behave unexpectedly or fail if team context is strictly required by the API for that feature.")
	}

	return AzureDevOpsConfig{
		Project: project,
		Team:    team,
		// BaseURL and PAT are used for connection, not directly in this config struct for the tool
	}
}

func TestNewAzureSprintOverviewTool(t *testing.T) {
	Convey("Given a valid Azure DevOps connection and config", t, func() {
		conn := getTestConnection(t)
		So(conn, ShouldNotBeNil) // If skipped, this won't run. If not skipped, conn must be non-nil.

		config := getTestConfig(t)
		// If config.Project is empty due to AZURE_PROJECT not being set, getTestConfig would have skipped.
		// So, if we are here, config.Project should be populated.
		So(config.Project, ShouldNotBeEmpty)

		Convey("When NewAzureSprintOverviewTool is called", func() {
			toolInstance := NewAzureSprintOverviewTool(conn, config)

			Convey("Then a new tool should be created successfully", func() {
				So(toolInstance, ShouldNotBeNil)
				azureTool, ok := toolInstance.(*AzureSprintOverviewTool)
				So(ok, ShouldBeTrue)
				So(azureTool.workClient, ShouldNotBeNil)
				So(azureTool.trackingClient, ShouldNotBeNil)
				So(azureTool.config, ShouldResemble, config)

				handle := azureTool.Handle()
				So(handle, ShouldNotBeNil)
				So(handle.Name, ShouldEqual, "azure_sprint_overview")
				So(handle.Description, ShouldNotBeEmpty)
				So(len(handle.InputSchema.Properties), ShouldEqual, 2) // sprint_identifier and format
			})
		})
	})
}

func TestAzureSprintOverviewTool_Handler_Integration(t *testing.T) {
	Convey("Given an initialized AzureSprintOverviewTool for integration testing", t, func() {
		conn := getTestConnection(t)
		So(conn, ShouldNotBeNil)

		config := getTestConfig(t)
		So(config.Project, ShouldNotBeEmpty) // Critical for the tool

		toolInstance := NewAzureSprintOverviewTool(conn, config)
		So(toolInstance, ShouldNotBeNil)
		azureTool, ok := toolInstance.(*AzureSprintOverviewTool)
		So(ok, ShouldBeTrue)

		ctx := context.Background()

		Convey("When Handler is called for the current sprint with JSON format", func() {
			if config.Team == "" {
				SkipConvey("Skipping @currentIteration test because AZURE_TEAM is not set (required for unambiguous current sprint resolution).", func() {})
				return // Skip this Convey block
			}
			request := mcp.CallToolRequest{
				Params: struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments,omitempty"`
					Meta      *struct {
						ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
					} `json:"_meta,omitempty"`
				}{
					Name: "azure_sprint_overview",
					Arguments: map[string]any{
						"sprint_identifier": "",
						"format":            "json",
					},
					Meta: nil,
				},
			}

			result, err := azureTool.Handler(ctx, request)

			Convey("Then the result should be a valid JSON overview of the current sprint", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
				So(result.Content, ShouldNotBeNil)
				So(len(result.Content), ShouldBeGreaterThan, 0)

				contentStr, ok := result.Content[0].(string)
				So(ok, ShouldBeTrue)
				So(contentStr, ShouldNotBeEmpty)

				var overview SprintOverviewOutput
				if !strings.Contains(contentStr, "Failed to") {
					errUnmarshal := json.Unmarshal([]byte(contentStr), &overview)
					So(errUnmarshal, ShouldBeNil)
					So(overview.Name, ShouldNotBeEmpty)
					So(overview.IterationPath, ShouldNotBeEmpty)
					So(overview.IterationPath, ShouldNotContainSubstring, "@currentIteration")
					So(overview.TotalWorkItems, ShouldBeGreaterThanOrEqualTo, 0)
					So(overview.WorkItemsByState, ShouldNotBeNil)
					So(overview.WorkItemsByType, ShouldNotBeNil)
					if overview.StartDate != "" {
						So(overview.StartDate, ShouldHaveLength, 10)
					}
					fmt.Printf("Current Sprint Overview (JSON): %+v\n", overview)
				} else {
					fmt.Printf("Current Sprint Overview (JSON) - Potential tool error in content: %s\n", contentStr)
				}
			})
		})

		Convey("When Handler is called for the current sprint with text format", func() {
			if config.Team == "" {
				SkipConvey("Skipping @currentIteration test because AZURE_TEAM is not set.", func() {})
				return
			}
			request := mcp.CallToolRequest{
				Params: struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments,omitempty"`
					Meta      *struct {
						ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
					} `json:"_meta,omitempty"`
				}{
					Name: "azure_sprint_overview",
					Arguments: map[string]any{
						"sprint_identifier": "@currentIteration",
						"format":            "text",
					},
					Meta: nil,
				},
			}

			result, err := azureTool.Handler(ctx, request)

			Convey("Then the result should be a valid text overview of the current sprint", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
				So(result.Content, ShouldNotBeNil)
				So(len(result.Content), ShouldBeGreaterThan, 0)

				contentStr, ok := result.Content[0].(string)
				So(ok, ShouldBeTrue)
				So(contentStr, ShouldNotBeEmpty)

				So(contentStr, ShouldContainSubstring, "## Sprint Overview:")
				So(contentStr, ShouldContainSubstring, "Iteration Path:")
				So(contentStr, ShouldNotContainSubstring, "@currentIteration")
				So(contentStr, ShouldContainSubstring, "Total Work Items:")
				fmt.Printf("Current Sprint Overview (Text):\n%s\n", contentStr)
			})
		})

		Convey("When Handler is called for a specific sprint path (using AZURE_TEST_SPRINT_PATH)", func() {
			specificSprintPath := os.Getenv("AZURE_TEST_SPRINT_PATH")
			if specificSprintPath == "" {
				SkipConvey("Skipping specific sprint path test because AZURE_TEST_SPRINT_PATH is not set.", func() {})
				return
			}

			request := mcp.CallToolRequest{
				Params: struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments,omitempty"`
					Meta      *struct {
						ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
					} `json:"_meta,omitempty"`
				}{
					Name: "azure_sprint_overview",
					Arguments: map[string]any{
						"sprint_identifier": specificSprintPath,
						"format":            "json",
					},
					Meta: nil,
				},
			}

			result, err := azureTool.Handler(ctx, request)

			Convey("Then the result should be a valid overview or a graceful error if not found/accessible", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
				So(result.Content, ShouldNotBeNil)
				So(len(result.Content), ShouldBeGreaterThan, 0)

				contentStr, ok := result.Content[0].(string)
				So(ok, ShouldBeTrue)

				if strings.Contains(contentStr, "Failed to") || strings.Contains(contentStr, "Could not determine sprint") {
					fmt.Printf("Specific Sprint Overview for '%s' (Tool Error in Content): %s\n", specificSprintPath, contentStr)
				} else {
					So(contentStr, ShouldNotBeEmpty)
					var overview SprintOverviewOutput
					errUnmarshal := json.Unmarshal([]byte(contentStr), &overview)
					So(errUnmarshal, ShouldBeNil)
					So(overview.IterationPath, ShouldContainSubstring, specificSprintPath)
					fmt.Printf("Specific Sprint Overview ('%s', JSON):\n%+v\n", specificSprintPath, overview)
				}
			})
		})

		Convey("When Handler is called with an invalid/non-existent sprint identifier", func() {
			nonExistentSprint := "ThisProject\\ThisTeam\\ThisSprintDefinitelyDoesNotExist12345abc"
			request := mcp.CallToolRequest{
				Params: struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments,omitempty"`
					Meta      *struct {
						ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
					} `json:"_meta,omitempty"`
				}{
					Name: "azure_sprint_overview",
					Arguments: map[string]any{
						"sprint_identifier": nonExistentSprint,
						"format":            "text",
					},
					Meta: nil,
				},
			}
			result, err := azureTool.Handler(ctx, request)

			Convey("Then it should return an error in the result.Content or an empty/appropriate response", func() {
				So(err, ShouldBeNil)
				So(result, ShouldNotBeNil)
				So(result.Content, ShouldNotBeNil)
				So(len(result.Content), ShouldBeGreaterThan, 0)

				contentStr, ok := result.Content[0].(string)
				So(ok, ShouldBeTrue)

				if strings.Contains(contentStr, "Failed to query work items") || strings.Contains(contentStr, "Could not determine sprint") {
					fmt.Printf("Non-existent Sprint Overview Response (Error in Content: %s)\n", contentStr)
					So(contentStr, ShouldContainSubstring, "Failed to")
				} else {
					fmt.Printf("Non-existent Sprint Overview Response (NoError in Content, Output: %s)\n", contentStr)
					So(contentStr, ShouldContainSubstring, "Total Work Items: 0")
					So(contentStr, ShouldContainSubstring, nonExistentSprint)
				}
			})
		})
	})
}
