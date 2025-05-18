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
			case "management":
				managementToolHandlerInstance := &tools.DelegateTool{}
				stdio.AddTool(*toolDefinition, managementToolHandlerInstance.Handle)
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
