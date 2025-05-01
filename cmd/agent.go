package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/types"
)

var (
	agentNameFlag string
	configFlag    string

	agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Run an A2A agent",
		Long:  longServe,
		RunE: func(cmd *cobra.Command, args []string) error {
			if configFlag == "" {
				return errors.New("config is required")
			}

			if agentNameFlag == "" {
				return errors.New("agent name is required")
			}

			v := viper.GetViper()

			skills := make([]types.AgentSkill, 0)

			for _, skill := range v.GetStringSlice(
				fmt.Sprintf("agent.%s.skills", configFlag),
			) {
				skills = append(skills, types.NewSkillFromConfig(skill))
			}

			service.NewA2AServer(ai.NewAgentFromCard(
				types.NewAgentCardFromConfig(configFlag),
			)).Start()

			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.PersistentFlags().StringVarP(&configFlag, "config", "c", "", "Configuration to use")
	agentCmd.Flags().StringVarP(&agentNameFlag, "name", "n", "A2A-Go Agent", "Name for the agent")
}

var longServe = `
Serve an A2A agent or MCP server with various configurations.

Examples:
  # Serve an A2A agent with the developer configuration.
  a2a-go agent --config developer
`
