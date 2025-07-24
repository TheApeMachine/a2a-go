package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/service/sse"
	"github.com/theapemachine/a2a-go/pkg/tools"
)

var (
	mcpCmd = &cobra.Command{
		Use:   "mcp",
		Short: "Run MCP services",
		Long:  longMCP,
		RunE: func(cmd *cobra.Command, args []string) error {
			if configFlag == "" {
				return errors.New("config flag is required for mcp command")
			}

			hostname := configFlag + "tool"
			broker := sse.NewMCPBroker(hostname)
			stdio, sseSrv := broker.MCPServer()
			toolDefinition, err := tools.Acquire(configFlag)

			if err != nil {
				return fmt.Errorf("failed to acquire tool definition: %w", err)
			}

			switch configFlag {
			case "browser":
				browserToolHandlerInstance := &tools.BrowserTool{}
				stdio.AddTool(*toolDefinition, browserToolHandlerInstance.Handle)
			case "docker":
				dockerToolHandlerInstance := &tools.DockerTool{}
				stdio.AddTool(*toolDefinition, dockerToolHandlerInstance.Handle)
			case "catalog":
				catalogToolHandlerInstance := &tools.CatalogTool{}
				stdio.AddTool(*toolDefinition, catalogToolHandlerInstance.Handle)
			case "azure_get_sprints":
				azureGetSprintsToolHandlerInstance := &tools.AzureGetSprintsTool{}
				stdio.AddTool(*toolDefinition, azureGetSprintsToolHandlerInstance.Handle)
			case "azure_create_sprint":
				azureCreateSprintToolHandlerInstance := &tools.AzureCreateSprintTool{}
				stdio.AddTool(*toolDefinition, azureCreateSprintToolHandlerInstance.Handle)
			case "azure_sprint_items":
				azureSprintItemsToolHandlerInstance := &tools.AzureSprintItemsTool{}
				stdio.AddTool(*toolDefinition, azureSprintItemsToolHandlerInstance.Handle)
			case "azure_sprint_overview":
				azureSprintOverviewToolHandlerInstance := &tools.AzureSprintOverviewTool{}
				stdio.AddTool(*toolDefinition, azureSprintOverviewToolHandlerInstance.Handle)
			case "azure_get_work_items":
				azureGetWorkItemsToolHandlerInstance := &tools.AzureGetWorkItemsTool{}
				stdio.AddTool(*toolDefinition, azureGetWorkItemsToolHandlerInstance.Handle)
			case "azure_create_work_items":
				azureCreateWorkItemsToolHandlerInstance := &tools.AzureCreateWorkItemsTool{}
				stdio.AddTool(*toolDefinition, azureCreateWorkItemsToolHandlerInstance.Handle)
			case "azure_update_work_items":
				azureUpdateWorkItemsToolHandlerInstance := &tools.AzureUpdateWorkItemsTool{}
				stdio.AddTool(*toolDefinition, azureUpdateWorkItemsToolHandlerInstance.Handle)
			case "azure_execute_wiql":
				azureExecuteWiqlToolHandlerInstance := &tools.AzureExecuteWiqlTool{}
				stdio.AddTool(*toolDefinition, azureExecuteWiqlToolHandlerInstance.Handle)
			case "azure_search_work_items":
				azureSearchWorkItemsToolHandlerInstance := &tools.AzureSearchWorkItemsTool{}
				stdio.AddTool(*toolDefinition, azureSearchWorkItemsToolHandlerInstance.Handle)
			case "azure_enrich_work_item":
				azureEnrichWorkItemToolHandlerInstance := &tools.AzureEnrichWorkItemTool{}
				stdio.AddTool(*toolDefinition, azureEnrichWorkItemToolHandlerInstance.Handle)
			case "azure_get_github_file_content":
				azureGetGithubFileContentToolHandlerInstance := &tools.AzureGetGithubFileContentTool{}
				stdio.AddTool(*toolDefinition, azureGetGithubFileContentToolHandlerInstance.Handle)
			case "azure_work_item_comments":
				azureWorkItemCommentsToolHandlerInstance := &tools.AzureWorkItemCommentsTool{}
				stdio.AddTool(*toolDefinition, azureWorkItemCommentsToolHandlerInstance.Handle)
			case "azure_find_items_by_status":
				azureFindItemsByStatusToolHandlerInstance := &tools.AzureFindItemsByStatusTool{}
				stdio.AddTool(*toolDefinition, azureFindItemsByStatusToolHandlerInstance.Handle)
			default:
				return fmt.Errorf("unsupported tool config for mcp command: %s", configFlag)
			}

			if err := sseSrv.Start("0.0.0.0:3210"); err != nil {
				log.Error("failed to start sse server", "error", err)
				return err
			}

			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(mcpCmd)

	mcpCmd.PersistentFlags().StringVarP(&configFlag, "config", "c", "", "Configuration to use")
}

var longMCP = `
Serve an MCP server with various configurations.

Examples:
  # Serve an MCP server with the developer configuration.
  a2a-go mcp --config docker
`
