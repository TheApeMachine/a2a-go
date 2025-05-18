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

			switch configFlag {
			case "browser":
				browserToolDefinition, err := tools.Acquire("web-browsing")
				if err != nil {
					return fmt.Errorf("failed to acquire browser tool definition: %w", err)
				}
				if browserToolDefinition.Name != configFlag {
					return fmt.Errorf("acquired tool for 'web-browsing' skill is not named '%s', got: %s", configFlag, browserToolDefinition.Name)
				}

				browserToolHandlerInstance := &tools.BrowserTool{}

				stdio.AddTool(*browserToolDefinition, browserToolHandlerInstance.Handle)
				log.Info("Registered 'browser' tool with MCP server", "hostname", hostname)

			case "docker":
				dockerToolDefinition, err := tools.Acquire("development")
				if err != nil {
					return fmt.Errorf("failed to acquire docker tool definition: %w", err)
				}

				if dockerToolDefinition.Name != configFlag {
					return fmt.Errorf("acquired tool for 'development' skill is not named '%s', got: %s", configFlag, dockerToolDefinition.Name)
				}

				dockerToolHandlerInstance := &tools.DockerTool{}

				stdio.AddTool(*dockerToolDefinition, dockerToolHandlerInstance.Handle)
				log.Info("Registered 'docker' tool with MCP server", "hostname", hostname)
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
