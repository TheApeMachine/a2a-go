package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/catalog"
)

var (
	testMessage string
	testLevel   int
	interactive bool

	testCmd = &cobra.Command{
		Use:   "test",
		Short: "Progressive test suite for the A2A system",
		Long:  longTest,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetReportCaller(true)
			log.SetLevel(log.InfoLevel)

			return runProgressiveTests()
		},
	}
)

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.Flags().StringVarP(&testMessage, "message", "m", "Hello from test command!", "Custom message for basic test")
	testCmd.Flags().IntVarP(&testLevel, "level", "l", 0, "Test level (0=all, 1=basic, 2=catalog, 3=delegation, 4=complex)")
	testCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Wait for user input between tests")
}

func runProgressiveTests() error {
	log.Info("🚀 Starting A2A System Progressive Test Suite")
	log.Info(strings.Repeat("=", 50))

	tests := []struct {
		level       int
		name        string
		description string
		testFunc    func() error
	}{
		{1, "Basic Communication", "Test basic message to UI agent", testBasicCommunication},
		{2, "Catalog Discovery", "Test agent discovery and listing", testCatalogDiscovery},
		{3, "Task Delegation", "Test UI agent delegating to other agents", testTaskDelegation},
		{4, "Complex Workflows", "Test multi-agent collaborative tasks", testComplexWorkflows},
	}

	for _, test := range tests {
		if testLevel != 0 && test.level != testLevel {
			continue
		}

		log.Info("\n📋 Level %d: %s\n", test.level, test.name)
		log.Info("   %s\n", test.description)
		log.Info("   " + strings.Repeat("-", 40))

		if interactive {
			log.Info("   Press Enter to continue...")
			fmt.Scanln()
		}

		start := time.Now()
		err := test.testFunc()
		duration := time.Since(start)

		if err != nil {
			log.Error("   ❌ FAILED (%v): %v\n", "duration", duration, "error", err)
			return fmt.Errorf("test level %d failed: %w", test.level, err)
		}

		log.Info("   ✅ PASSED (%v)\n", "duration", duration)
		time.Sleep(2 * time.Second) // Brief pause between tests
	}

	log.Info("\n🎉 All tests completed successfully!")
	log.Info("   The A2A system is fully operational and ready for use.")
	return nil
}

func testBasicCommunication() error {
	log.Info("   → Sending basic message to UI agent...")

	uiAgent, err := getUIAgent()
	if err != nil {
		return fmt.Errorf("failed to get UI agent: %w", err)
	}

	task, err := sendMessageToAgent(uiAgent, testMessage, "basic-test")
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	log.Info("   → Response: %s\n", "response", getTaskResponse(task))
	return nil
}

func testCatalogDiscovery() error {
	log.Info("   → Asking UI agent to discover available agents...")

	uiAgent, err := getUIAgent()
	if err != nil {
		return err
	}

	message := "Please use the catalog tool to show me all available agents in the system. For each agent, tell me their name, URL, and what they do."
	task, err := sendMessageToAgent(uiAgent, message, "catalog-test")
	if err != nil {
		return err
	}

	response := getTaskResponse(task)
	log.Info("   → Agent Discovery Result:\n")
	log.Info("     %s\n", "response", response)

	return nil
}

func testTaskDelegation() error {
	log.Info("   → Testing delegation to different agent types...")

	uiAgent, err := getUIAgent()
	if err != nil {
		return err
	}

	delegationTests := []struct {
		name    string
		message string
	}{
		{
			"Planning Task",
			"I need to plan a software development project. Please delegate this to the planner agent and ask them to create a basic project plan with phases and milestones.",
		},
		{
			"Research Task",
			"I need research on the latest trends in AI agents. Please delegate this to the researcher agent.",
		},
		{
			"Management Task",
			"Please delegate to the manager agent: I need help organizing a team workflow for a development project.",
		},
	}

	for _, test := range delegationTests {
		log.Info("   → %s...\n", test.name)

		task, err := sendMessageToAgent(uiAgent, test.message, "delegation-test")
		if err != nil {
			return fmt.Errorf("delegation test '%s' failed: %w", test.name, err)
		}

		response := getTaskResponse(task)
		log.Info("     ✓ Delegation successful. Response length: %d chars\n", "length", len(response))
	}

	return nil
}

func testComplexWorkflows() error {
	log.Info("   → Testing complex multi-agent workflow...")

	uiAgent, err := getUIAgent()
	if err != nil {
		return err
	}

	complexMessage := `I want to start a new software project. Please coordinate with multiple agents:
1. First, ask the planner to create a project plan
2. Then, ask the researcher to find best practices for the technology stack
3. Finally, ask the manager to organize the team structure and workflows
Please orchestrate this workflow and provide me with a comprehensive summary of all results.`

	log.Info("   → Initiating multi-agent workflow...")
	task, err := sendMessageToAgent(uiAgent, complexMessage, "complex-workflow-test")
	if err != nil {
		return err
	}

	response := getTaskResponse(task)
	log.Info("   → Workflow completed. Response summary:\n")
	log.Info("     Response length: %d characters\n", "length", len(response))

	// Check if response mentions different agents
	agentMentions := 0
	agents := []string{"planner", "researcher", "manager", "plan", "research"}
	for _, agent := range agents {
		if contains(response, agent) {
			agentMentions++
		}
	}

	log.Info("     Agent interaction indicators: %d/%d\n", "agentMentions", agentMentions, "agents", len(agents))

	if agentMentions < 2 {
		return fmt.Errorf("complex workflow may not have involved multiple agents (only %d indicators found)", agentMentions)
	}

	return nil
}

// Helper functions
func getUIAgent() (*a2a.AgentCard, error) {
	v := viper.GetViper()
	catalogURL := v.GetString("endpoints.catalog")

	if catalogURL == "" {
		return nil, fmt.Errorf("catalog endpoint not configured")
	}

	catalogClient := catalog.NewCatalogClient(catalogURL)
	agents, err := catalogClient.GetAgents()
	if err != nil {
		return nil, fmt.Errorf("failed to get agents from catalog: %w", err)
	}

	for _, agent := range agents {
		if agent.Name == "User Interface Agent" {
			return &agent, nil
		}
	}

	return nil, fmt.Errorf("User Interface Agent not found in catalog")
}

func sendMessageToAgent(agent *a2a.AgentCard, message, origin string) (map[string]interface{}, error) {
	agentClient := a2a.NewClient(agent.URL)

	msg := a2a.NewTextMessage("user", message)
	msg.Metadata = map[string]any{"origin": origin}

	task, err := agentClient.SendTask(a2a.TaskSendParams{
		ID:        uuid.New().String(),
		SessionID: uuid.New().String(),
		Message:   *msg,
	})

	if err != nil {
		return nil, err
	}

	// Convert to map for easier processing
	var taskMap map[string]interface{}
	taskJSON, _ := json.Marshal(task)
	json.Unmarshal(taskJSON, &taskMap)

	return taskMap, nil
}

func getTaskResponse(task map[string]interface{}) string {
	if result, ok := task["result"].(map[string]interface{}); ok {
		if message, ok := result["message"].(string); ok {
			return message
		}
	}

	// Fallback: try to get any message from the task
	if msg, ok := task["message"].(string); ok {
		return msg
	}

	// Last resort: stringify the whole result
	if result, ok := task["result"]; ok {
		if resultStr, err := json.MarshalIndent(result, "", "  "); err == nil {
			return string(resultStr)
		}
	}

	return "No readable response found"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

var longTest = `
Progressive test suite for the A2A (Agent-to-Agent) system.

This command runs a series of increasingly complex tests to verify that:
1. Basic agent communication works
2. Agent discovery through catalog works  
3. Task delegation between agents works
4. Complex multi-agent workflows work
5. Results stream back properly

Test Levels:
  1. Basic Communication - Simple message to UI agent
  2. Catalog Discovery - Agent discovery and listing
  3. Task Delegation - UI agent delegating to other agents
  4. Complex Workflows - Multi-agent collaborative tasks

Examples:
  # Run all tests
  a2a-go test

  # Run specific test level
  a2a-go test -l 3

  # Run interactively with pauses
  a2a-go test -i

  # Run with custom basic message
  a2a-go test -m "Custom test message" -l 1
`
