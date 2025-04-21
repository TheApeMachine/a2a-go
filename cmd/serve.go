package cmd

import (
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/tools"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

var (
	portFlag      int
	hostFlag      string
	agentNameFlag string
	mcpModeFlag   bool
	configFlag    string

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
			if configFlag == "" {
				return errors.New("config is required")
			}

			v := viper.GetViper()

			skills := make([]types.AgentSkill, 0)

			for _, skill := range v.GetStringSlice(
				fmt.Sprintf("agent.%s.skills", configFlag),
			) {
				skills = append(skills, types.NewSkillFromConfig(skill))
			}

			ai.NewAgentFromCard(
				&types.AgentCard{
					Name:    "developer",
					Version: "0.0.1",
					Description: utils.Ptr(
						"A tool that can execute commands in a Docker container.",
					),
					URL: "http://localhost:3210/agents/" + configFlag,
					Provider: &types.AgentProvider{
						Organization: "theapemachine",
						URL:          utils.Ptr("https://github.com/theapemachine"),
					},
					Capabilities: types.AgentCapabilities{
						Streaming:              true,
						PushNotifications:      true,
						StateTransitionHistory: true,
					},
					Skills: skills,
				},
			)

			return nil
		},
	}

	mcpCmd = &cobra.Command{
		Use:   "mcp",
		Short: "Serve an MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := server.NewMCPServer(
				"Demo 🚀",
				"1.0.0",
				server.WithLogging(),
			)

			tools.RegisterDockerTools(s)
			return server.ServeStdio(s)
		},
	}
)

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.AddCommand(agentCmd)
	serveCmd.AddCommand(mcpCmd)

	serveCmd.PersistentFlags().StringVarP(&configFlag, "config", "c", "", "Configuration to use")
	serveCmd.PersistentFlags().IntVarP(&portFlag, "port", "p", 3210, "Port to serve on")
	serveCmd.PersistentFlags().StringVarP(&hostFlag, "host", "H", "0.0.0.0", "Host address to bind to")

	agentCmd.Flags().StringVarP(&agentNameFlag, "name", "n", "A2A-Go Agent", "Name for the agent")
	mcpCmd.Flags().BoolVar(&mcpModeFlag, "with-agent", false, "Serve with a builtin agent")
}

var longServe = `
Serve an A2A agent or MCP server with various configurations.

Examples:
  # Serve an A2A agent with the developer configuration.
  a2a-go serve agent --config developer

  # Serve an MCP server on port 3000.
  a2a-go serve mcp --port 3000
`
