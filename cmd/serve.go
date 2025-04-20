package cmd

import (
	"github.com/spf13/cobra"
)

var (
	portFlag      int
	hostFlag      string
	agentNameFlag string
	mcpModeFlag   bool

	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Run A2A and MCP services",
		Long:  longServe,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Serve an A2A agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	mcpCmd = &cobra.Command{
		Use:   "mcp",
		Short: "Serve an MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.AddCommand(agentCmd)
	serveCmd.AddCommand(mcpCmd)

	serveCmd.PersistentFlags().IntVarP(&portFlag, "port", "p", 3210, "Port to serve on")
	serveCmd.PersistentFlags().StringVarP(&hostFlag, "host", "H", "0.0.0.0", "Host address to bind to")

	agentCmd.Flags().StringVarP(&agentNameFlag, "name", "n", "A2A-Go Agent", "Name for the agent")
	mcpCmd.Flags().BoolVar(&mcpModeFlag, "with-agent", false, "Serve with a builtin agent")
}

var longServe = `
Serve an A2A agent or MCP server with various configurations.

Examples:
  # Serve an A2A agent on port 8080
  caramba serve agent --port 8080

  # Serve an MCP server on port 3000
  caramba serve mcp --port 3000

  # Serve an MCP server with an embedded agent
  caramba serve mcp --with-agent --port 3000
`
