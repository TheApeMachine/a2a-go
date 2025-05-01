package examples

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
CollaborationExample demonstrates two agents collaborating to solve a problem.
One agent analyzes requirements and creates a plan, while the developer agent
actually implements the solution.
*/
type CollaborationExample struct {
	plannerAgent   *ai.Agent
	developerAgent *ai.Agent
	currentTask    *types.Task
}

/*
NewCollaborationExample creates a new collaboration example
*/
func NewCollaborationExample() *CollaborationExample {
	return &CollaborationExample{}
}

/*
Initialize sets up the planner and developer agents
*/
func (example *CollaborationExample) Initialize(v *viper.Viper) {
	// Create the planner agent
	plannerSkill := types.AgentSkill{
		ID:   "planning",
		Name: "Solution Planning",
		Description: utils.Ptr(
			"Analyze requirements and create implementation plans",
		),
		Examples: []string{
			"Create a plan for implementing a REST API",
			"Design the architecture for a microservice",
		},
		InputModes:  []string{"text/plain"},
		OutputModes: []string{"text/plain"},
	}

	// Create the developer agent
	devSkill := types.AgentSkill{
		ID:   v.GetString("skills.development.id"),
		Name: v.GetString("skills.development.name"),
		Description: utils.Ptr(
			v.GetString("skills.development.description"),
		),
		Examples:    v.GetStringSlice("skills.development.examples"),
		InputModes:  v.GetStringSlice("skills.development.input_modes"),
		OutputModes: v.GetStringSlice("skills.development.output_modes"),
	}

	// Create the planner agent
	example.plannerAgent = ai.NewAgentFromCard(
		&types.AgentCard{
			Name:    "planner-agent",
			Version: "0.0.1",
			Description: utils.Ptr(
				"An agent that analyzes requirements and creates implementation plans",
			),
			URL: "http://localhost:3211/agents/planner",
			Provider: &types.AgentProvider{
				Organization: "theapemachine",
				URL:          utils.Ptr("https://github.com/theapemachine"),
			},
			Capabilities: types.AgentCapabilities{
				Streaming:              true,
				PushNotifications:      true,
				StateTransitionHistory: true,
			},
			Skills: []types.AgentSkill{
				plannerSkill,
			},
		},
	)

	// Create the developer agent
	example.developerAgent = ai.NewAgentFromCard(
		&types.AgentCard{
			Name:    "developer-agent",
			Version: "0.0.1",
			Description: utils.Ptr(
				"A tool that can execute commands in a Docker container",
			),
			URL: "http://localhost:3212/agents/developer",
			Provider: &types.AgentProvider{
				Organization: "theapemachine",
				URL:          utils.Ptr("https://github.com/theapemachine"),
			},
			Capabilities: types.AgentCapabilities{
				Streaming:              true,
				PushNotifications:      true,
				StateTransitionHistory: true,
			},
			Skills: []types.AgentSkill{
				devSkill,
			},
		},
	)
}

/*
Run executes the collaboration example
*/
func (example *CollaborationExample) Run(interactive bool) error {
	var (
		v      = viper.GetViper()
		prompt string
	)

	example.Initialize(v)

	// Start the planner agent service
	go func() {
		plannerSrv := service.NewA2AServer(example.plannerAgent)
		if err := plannerSrv.Start(); err != nil {
			log.Error("planner agent service exited with error", "error", err)
		}
	}()

	// Start the developer agent service
	go func() {
		developerSrv := service.NewA2AServer(example.developerAgent)
		if err := developerSrv.Start(); err != nil {
			log.Error("developer agent service exited with error", "error", err)
		}
	}()

	// Give the servers a moment to start
	time.Sleep(500 * time.Millisecond)

	prompt = "Create a simple Go program that implements a web server handling GET requests on / that returns 'Hello World'"

	if interactive {
		huh.NewInput().
			Title("What would you like the agents to build?").
			Value(&prompt).
			Run()
	}

	// Step 1: Send the user request to the planner agent
	plannerResult, err := example.sendToPlanner(prompt)
	if err != nil {
		log.Error("Failed to get plan from planner agent", "error", err)
		return err
	}

	fmt.Println("\n--- Plan created by the planner agent ---")
	fmt.Println(plannerResult)
	fmt.Println("--- End of plan ---")

	// Step 2: Send the plan to the developer agent
	developerPrompt := fmt.Sprintf("Implement the following plan:\n\n%s", plannerResult)
	implementationResult, err := example.sendToDeveloper(developerPrompt)
	if err != nil {
		log.Error("Failed to get implementation from developer agent", "error", err)
		return err
	}

	fmt.Println("\n--- Implementation by the developer agent ---")
	fmt.Println(implementationResult)
	fmt.Println("--- End of implementation ---")

	return nil
}

/*
sendToPlanner sends a task to the planner agent and returns the result
*/
func (example *CollaborationExample) sendToPlanner(prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	task := types.Task{
		ID:        uuid.NewString(),
		SessionID: uuid.NewString(),
		History: []types.Message{
			{
				Role: "system",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: "You are a planning agent that analyzes requirements and creates implementation plans. " +
							"Your plans should be detailed and clear for a developer to implement.",
					},
				},
			},
			{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: prompt,
					},
				},
			},
		},
	}

	result, rpcErr := example.plannerAgent.SendTask(ctx, task)
	if rpcErr != nil {
		return "", fmt.Errorf("planner agent error: %s", rpcErr.Message)
	}

	// Extract the text from the artifacts
	if len(result.Artifacts) > 0 && len(result.Artifacts[0].Parts) > 0 {
		return result.Artifacts[0].Parts[0].Text, nil
	}

	return "", fmt.Errorf("no output from planner agent")
}

/*
sendToDeveloper sends a task to the developer agent and returns the result
*/
func (example *CollaborationExample) sendToDeveloper(prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	task := types.Task{
		ID:        uuid.NewString(),
		SessionID: uuid.NewString(),
		History: []types.Message{
			{
				Role: "system",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: "You are a developer agent that can implement software solutions " +
							"based on requirements. You have access to a Docker container with " +
							"development tools. Always test your implementations.",
					},
				},
			},
			{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: prompt,
					},
				},
			},
		},
	}

	result, rpcErr := example.developerAgent.SendTask(ctx, task)
	if rpcErr != nil {
		return "", fmt.Errorf("developer agent error: %s", rpcErr.Message)
	}

	// Extract the text from the artifacts
	if len(result.Artifacts) > 0 && len(result.Artifacts[0].Parts) > 0 {
		return result.Artifacts[0].Parts[0].Text, nil
	}

	return "", fmt.Errorf("no output from developer agent")
}
