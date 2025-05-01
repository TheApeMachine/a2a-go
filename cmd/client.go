package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/client"
)

var (
	catalogURLFlag string

	clientCmd = &cobra.Command{
		Use:   "client",
		Short: "A2A client operations",
		Long:  `Run client operations against A2A agents`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	demoCmd = &cobra.Command{
		Use:   "demo",
		Short: "Run a demonstration of A2A Protocol client interactions",
		Long:  `Demonstrates how to discover agents from a catalog and interact with them using the A2A Protocol`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetReportCaller(true)
			log.SetCallerOffset(0)
			log.SetLevel(log.DebugLevel)

			catalogURL := catalogURLFlag
			if catalogURL == "" {
				catalogURL = os.Getenv("CATALOG_URL")
				if catalogURL == "" {
					catalogURL = "http://localhost:3210"
				}
			}

			// Give the catalog and agents a moment to start up and retry a few times
			log.Info("Waiting for catalog and agents to start...", "catalogURL", catalogURL)

			// Create a catalog client
			catalogClient := catalog.NewCatalogClient(catalogURL)

			// Retry logic for getting agents
			var agents []catalog.AgentCard
			var err error
			maxRetries := 5
			retryDelay := 3 * time.Second

			for retry := 0; retry < maxRetries; retry++ {
				if retry > 0 {
					log.Info("Retrying catalog connection...", "attempt", retry+1, "maxRetries", maxRetries)
					time.Sleep(retryDelay)
				}

				// Discover all available agents
				agents, err = catalogClient.GetAgents()
				if err == nil && len(agents) > 0 {
					// Success - we have agents
					break
				}

				if err != nil {
					log.Warn("Failed to get agents from catalog, will retry", "error", err, "attempt", retry+1)
				} else if len(agents) == 0 {
					log.Warn("No agents found in catalog, will retry", "attempt", retry+1)
				}
			}

			// Start the demo
			fmt.Println("\n🚀 A2A Protocol Client Demo")
			fmt.Println("==========================")
			fmt.Println("This demo shows how to discover agents from a catalog and interact with them.\n")

			// Error handling after all retries
			if err != nil {
				log.Error("Failed to get agents from catalog after multiple attempts", "error", err)
				return err
			}

			// Print the discovered agents
			fmt.Printf("📋 Discovered %d agents in the catalog:\n\n", len(agents))
			for i, agent := range agents {
				fmt.Printf("%d. %s (%s)\n", i+1, agent.Name, agent.URL)
				fmt.Printf("   Description: %s\n", *agent.Description)
				fmt.Printf("   Skills: %d\n", len(agent.Skills))
				fmt.Println()
			}

			// No agents found
			if len(agents) == 0 {
				fmt.Println("❌ No agents found in the catalog. Please ensure the agents are running and registered.")
				return fmt.Errorf("no agents found in catalog")
			}

			// User selects what to build
			var promptInput string
			prompt := huh.NewInput().
				Title("What would you like the agents to build for you?").
				Value(&promptInput).
				Placeholder("Create a simple Go web server that serves Hello World on port 8080").
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("please enter a request")
					}
					return nil
				})

			// Run the prompt
			err = prompt.Run()
			if err != nil {
				return err
			}

			// Find a planner agent
			var plannerAgent, developerAgent *client.AgentClient
			for _, agent := range agents {
				agentClient := client.NewAgentClient(agent)

				// Look for agents with specific skills
				if plannerAgent == nil && hasSkill(agent, "planning") {
					fmt.Printf("✅ Found planner agent: %s\n", agent.Name)
					plannerAgent = agentClient
				}

				if developerAgent == nil && hasSkill(agent, "development") {
					fmt.Printf("✅ Found developer agent: %s\n", agent.Name)
					developerAgent = agentClient
				}

				if plannerAgent != nil && developerAgent != nil {
					break
				}
			}

			if plannerAgent == nil {
				return fmt.Errorf("no planner agent found")
			}

			if developerAgent == nil {
				return fmt.Errorf("no developer agent found")
			}

			// Step 1: Ask the planner to analyze and create a plan
			fmt.Println("\n🧩 Step 1: Creating a plan with the Planner agent...")
			plan, err := plannerAgent.SendTaskRequest(promptInput)
			if err != nil {
				log.Error("Failed to get plan from planner agent", "error", err)
				return err
			}

			fmt.Println("\n📝 Plan created by the planner agent:")
			fmt.Println("--------------------------------------")
			fmt.Println(plan)
			fmt.Println("--------------------------------------")

			// Step 2: Give the plan to the developer to implement
			fmt.Println("\n🛠️ Step 2: Implementing the solution with the Developer agent...")
			developerPrompt := fmt.Sprintf("Implement the following plan:\n\n%s", plan)
			solution, err := developerAgent.SendTaskRequest(developerPrompt)
			if err != nil {
				log.Error("Failed to get solution from developer agent", "error", err)
				return err
			}

			fmt.Println("\n💻 Implementation by the developer agent:")
			fmt.Println("------------------------------------------")
			fmt.Println(solution)
			fmt.Println("------------------------------------------")

			fmt.Println("\n✨ A2A Protocol demonstration completed successfully!")
			fmt.Println("The agents discovered and collaborated following the A2A Protocol.")

			return nil
		},
	}
)

// hasSkill checks if an agent has a specific skill by ID
func hasSkill(agent catalog.AgentCard, skillID string) bool {
	for _, skill := range agent.Skills {
		if skill.ID == skillID {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(clientCmd)
	clientCmd.AddCommand(demoCmd)

	demoCmd.Flags().StringVarP(&catalogURLFlag, "catalog", "c", "", "URL of the agent catalog (default: $CATALOG_URL or http://localhost:3210)")
}
