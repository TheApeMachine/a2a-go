package cmd

import (
	"fmt"
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
					log.Info("Retrying catalog connection...", "attempt", retry+1, "maxRetries", maxRetries)
					time.Sleep(retryDelay)
				}

				agents, err = catalogClient.GetAgents()

				if err == nil && len(agents) > 0 {
					break
				}

				if err != nil {
					log.Warn("Failed to get agents from catalog, will retry", "error", err, "attempt", retry+1)
				} else if len(agents) == 0 {
					log.Warn("No agents found in catalog, will retry", "attempt", retry+1)
				}
			}

			if err != nil {
				log.Error("Failed to get agents from catalog after multiple attempts", "error", err)
				return err
			}

			fmt.Printf("📋 Discovered %d agents in the catalog:\n\n", len(agents))

			if len(agents) == 0 {
				fmt.Println("❌ No agents found in the catalog. Please ensure the agents are running and registered.")
				return fmt.Errorf("no agents found in catalog")
			}

			promptInput := "Create a simple Go web server that serves Hello World on port 8080"
			fmt.Printf("🤖 Automated demo using task: \"%s\"\n\n", promptInput)

			// Find a planner agent and developer agent
			var plannerAgent *client.AgentClient

			for _, agent := range agents {
				if agent.Name == "Planner Agent" {
					fmt.Printf("✅ Found planner agent: %s\n", agent.Name)

					// Make sure the URL is properly formatted
					if agent.URL != "" {
						plannerAgent = client.NewAgentClient(agent)
					} else {
						log.Warn("Planner agent has no URL configured", "agent", agent.Name)
					}
				}
			}

			if plannerAgent == nil {
				return fmt.Errorf("no planner agent found in catalog")
			}

			// Since we are the "user", all we need to do is send the task to the planner
			// and the planner will handle the rest. Since this is a long-running task,
			// we expect to receive at the very least a task ID, so we can use it to request
			// updates on the task, or streaming updates as they are produced.
			response, err := plannerAgent.SendTaskRequest(promptInput)
			if err != nil {
				log.Error("Failed to get plan from planner agent", "error", err)
				return err
			}

			fmt.Println("🤖 Planner response:")
			fmt.Println(response)

			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(demoCmd)
}
