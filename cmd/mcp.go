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

			// Construct the hostname for the current service
			// e.g., if configFlag is "browser", hostname is "browsertool"
			hostname := configFlag + "tool"
			broker := sse.NewMCPBroker(hostname) // Pass the hostname

			switch configFlag {
			case "browser":
				browserToolDefinition, err := tools.Aquire("web-browsing")
				if err != nil {
					return fmt.Errorf("failed to acquire browser tool definition: %w", err)
				}
				if browserToolDefinition.Name != configFlag {
					return fmt.Errorf("acquired tool for 'web-browsing' skill is not named '%s', got: %s", configFlag, browserToolDefinition.Name)
				}

				browserToolHandlerInstance := &tools.BrowserTool{}
				stdio, sseSrv := broker.MCPServer()

				stdio.AddTool(*browserToolDefinition, browserToolHandlerInstance.Handle)
				// Attempt to get BaseURL for logging, assuming it's accessible. This might need adjustment based on mcp-go library internals.
				// For now, we focus on the core logic. If sseSrv.BaseURL is not public, this part of the log can be removed.
				log.Info("Registered 'browser' tool with MCP server", "hostname", hostname)

				if err := sseSrv.Start("0.0.0.0:3210"); err != nil {
					log.Error("failed to start sse server", "error", err)
					return err
				}

			case "docker":
				dockerToolDefinition, err := tools.Aquire("development")
				if err != nil {
					return fmt.Errorf("failed to acquire docker tool definition: %w", err)
				}

				if dockerToolDefinition.Name != configFlag {
					return fmt.Errorf("acquired tool for 'development' skill is not named '%s', got: %s", configFlag, dockerToolDefinition.Name)
				}

				dockerToolHandlerInstance := &tools.DockerTool{}
				stdio, sseSrv := broker.MCPServer()

				stdio.AddTool(*dockerToolDefinition, dockerToolHandlerInstance.Handle)
				log.Info("Registered 'docker' tool with MCP server", "hostname", hostname)

				if err := sseSrv.Start("0.0.0.0:3210"); err != nil {
					log.Error("failed to start sse server", "error", err)
					return err
				}
			default:
				return fmt.Errorf("unsupported tool config for mcp command: %s", configFlag)
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
