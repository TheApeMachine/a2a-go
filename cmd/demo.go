package cmd

import (
	"errors"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/client"
)

var (
	demoCmd = &cobra.Command{
		Use:          "demo",
		Short:        "Run a demonstration of A2A Protocol client interactions",
		Long:         `Demonstrates how to discover agents from a catalog and interact with them using the A2A Protocol`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetReportCaller(true)
			log.SetCallerOffset(0)
			log.SetLevel(log.DebugLevel)

			catalogURL := "http://catalog:3210"
			catalogClient := catalog.NewCatalogClient(catalogURL)

			var agents []catalog.AgentCard
			var err error
			maxRetries := 5
			retryDelay := 3 * time.Second

			for retry := range maxRetries {
				if retry > 0 {
					log.Info(
						"Retrying catalog connection...",
						"attempt", retry+1,
						"maxRetries", maxRetries,
					)

					time.Sleep(retryDelay)
				}

				agents, err = catalogClient.GetAgents()

				if err == nil && len(agents) > 0 {
					break
				}

				if err != nil {
					log.Warn(
						"failed to retrieve agents from catalog",
						"error", err,
						"attempt", retry+1,
					)
				} else if len(agents) == 0 {
					log.Warn(
						"no agents found in catalog",
						"attempt", retry+1,
					)
				}
			}

			if err != nil {
				log.Error("failed to retrieve agents from catalog", "error", err)
				return err
			}

			promptInput := `
			Create a simple Go web server that serves Hello World on port 8080.
			Make sure to use the correct port and handle all the usual Go web server boilerplate.
			You must run the server at least once and confirm it's working before finishing.
			`

			var plannerAgent *client.AgentClient

			for _, agent := range agents {
				if agent.Name == "Planner Agent" {
					log.Info("found planner agent", "agent", agent.Name)
					plannerAgent = client.NewAgentClient(agent)
				}
			}

			if plannerAgent == nil {
				log.Error("no planner agent found in catalog")
				return errors.New("no planner agent found in catalog")
			}

			// Since we are the "user", all we need to do is send the task to the planner
			// and the planner will handle the rest. Since this is a long-running task,
			// we expect to receive at the very least a task ID, so we can use it to request
			// updates on the task, or streaming updates as they are produced.
			response, err := plannerAgent.SendTaskRequest(promptInput)
			if err != nil {
				log.Error("failed to get response from planner agent", "error", err)
				return err
			}

			log.Info("planner response", "response", response)
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(demoCmd)
}
