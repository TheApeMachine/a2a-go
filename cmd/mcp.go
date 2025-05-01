package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/service/sse"
)

var (
	mcpCmd = &cobra.Command{
		Use:   "mcp",
		Short: "Run MCP services",
		Long:  longMCP,
		RunE: func(cmd *cobra.Command, args []string) error {
			if configFlag == "" {
				return errors.New("config is required")
			}

			return sse.NewMCPBroker().Start()
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
