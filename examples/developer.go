package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/r3labs/sse/v2"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
DeveloperExample is a naive implementation of a developer agent.
It is used to demonstrate the capabilities of combining A2A with MCP.

You need to have a running Docker daemon for this to work, as the agent will
use a Docker container as a tool to have a working environment.
*/
type DeveloperExample struct {
	devSkill types.AgentSkill
	agent    *ai.Agent
	client   *jsonrpc.RPCClient
	task     types.Task
}

/*
NewDeveloperExample creates a new DeveloperExample instance.
*/
func NewDeveloperExample() *DeveloperExample {
	return &DeveloperExample{}
}

/*
Initialize the DeveloperExample instance, by setting up the agent,
and any skills it needs.

Skills are defined in the A2A spec, and are used to describe the capabilities
of the agent, which in turn will map to the tools it can use.

To get a better understanding of how skills work, have a look at
types/card.go, specifically the Tools() method of the AgentCard type.
*/
func (example *DeveloperExample) Initialize(v *viper.Viper) {
	example.devSkill = types.AgentSkill{
		ID:   v.GetString("skills.development.id"),
		Name: v.GetString("skills.development.name"),
		Description: utils.Ptr(
			v.GetString("skills.development.description"),
		),
		Examples:    v.GetStringSlice("skills.development.examples"),
		InputModes:  v.GetStringSlice("skills.development.input_modes"),
		OutputModes: v.GetStringSlice("skills.development.output_modes"),
	}

	example.agent = ai.NewAgentFromCard(
		&types.AgentCard{
			Name:    v.GetString("agent.developer.name"),
			Version: v.GetString("agent.developer.version"),
			Description: utils.Ptr(
				v.GetString("agent.developer.description"),
			),
			URL: v.GetString("agent.developer.url"),
			Provider: &types.AgentProvider{
				Organization: v.GetString("agent.developer.provider.organization"),
				URL:          utils.Ptr(v.GetString("agent.developer.provider.url")),
			},
			Capabilities: types.AgentCapabilities{
				Streaming:              true,
				PushNotifications:      true,
				StateTransitionHistory: true,
			},
			Skills: []types.AgentSkill{
				example.devSkill,
			},
		},
	)

	// Use the client to communicate with the agent. We are no longer
	// calling methods on the agent directly, but rather through the
	// client, which follows the A2A protocol.
	example.client = jsonrpc.NewRPCClient("http://localhost:3210/rpc")
}

func (example *DeveloperExample) Run(interactive bool) error {
	var (
		v      = viper.GetViper()
		prompt string
	)

	example.Initialize(v)

	// Start the agent as a service, so it can be used by the client.
	// We run it in a goroutine to avoid blocking; in real usage you might
	// run "a2a-go serve" separately.
	go func() {
		srv := service.NewA2AServer(example.agent)
		if err := srv.Start(); err != nil {
			log.Error("agent service exited with error", "error", err)
		}
	}()
	// Give the server a moment to start and bind to the port
	time.Sleep(500 * time.Millisecond)

	prompt = "Develop an echo server in Go, and run it to show it works."

	if interactive {
		huh.NewInput().
			Title("Prompt?").
			Value(&prompt).
			Run()
	}

	example.setTask(prompt)
	example.processTask(example.client)

	return nil
}

func (example *DeveloperExample) processTask(
	client *jsonrpc.RPCClient,
) {
	// Use tasks/sendSubscribe to initiate the task and streaming
	// We don't expect a direct result payload from the RPC call itself for streaming.
	// The result comes via the SSE connection established afterwards.
	rpcCtx, cancelRpc := context.WithTimeout(context.Background(), 10*time.Second) // Timeout for RPC call
	defer cancelRpc()

	// Prepare parameters according to spec
	sendParams := types.TaskSendParams{
		ID:        example.task.ID,
		SessionID: &example.task.SessionID,
		Message:   example.task.History[len(example.task.History)-1], // Send the last message (user prompt)
		// Let the agent handle history based on the task ID / session ID implicitly.
		// HistoryLength can be used with tasks/get, maybe not needed for sendSubscribe?
		// PushNotification: nil, // Not configured in this example
		// Metadata: nil, // No specific metadata to send initially
	}

	log.Info("Sending tasks/sendSubscribe request", "taskID", example.task.ID)
	if err := client.Call(
		rpcCtx, "tasks/sendSubscribe", sendParams, nil, // Pass sendParams, expect no direct result
	); err != nil {
		log.Error("Failed to send task via tasks/sendSubscribe", "taskID", example.task.ID, "error", err)
		return
	}
	log.Info("tasks/sendSubscribe call successful, attempting to connect to SSE stream", "taskID", example.task.ID)

	// --- SSE Event Handling ---
	// The A2A spec doesn't explicitly define the SSE endpoint path.
	// Common patterns: Use base URL, /events, /stream, /events/{taskID}
	// Let's *assume* the stream is available at the agent's base URL for now.
	// This needs clarification from the spec or server implementation.
	sseURL := fmt.Sprintf("%s/events/%s", example.agent.Card().URL, example.task.ID) // Use specific event stream for task
	log.Info("Connecting to SSE stream", "url", sseURL, "taskID", example.task.ID)

	// Note: The r3labs SSE client automatically handles reconnections.
	sseClient := sse.NewClient(sseURL)

	// Add authentication headers if needed, based on agent card
	if example.agent.Card().Authentication != nil {
		for _, scheme := range example.agent.Card().Authentication.Schemes {
			if scheme == "Bearer" && example.agent.Card().Authentication.Credentials != nil {
				sseClient.Headers["Authorization"] = fmt.Sprintf("Bearer %s", *example.agent.Card().Authentication.Credentials)
			}
		}
	}

	ctx, cancelSse := context.WithCancel(context.Background())
	defer cancelSse() // Ensure context is cancelled on exit

	// Channel to signal completion or error from SSE handler
	done := make(chan bool)
	errChan := make(chan error, 1)

	go func() {
		err := sseClient.SubscribeWithContext(ctx, "", func(msg *sse.Event) {
			// We can receive different message types (status, artifact)
			// Determine type by inspecting the JSON data
			data := string(msg.Data)
			log.Debug("SSE event received", "data", data)

			if data == "" {
				return // Ignore empty messages
			}

			// Try parsing as TaskStatusUpdateEvent
			var statusEvent types.TaskStatusUpdateEvent
			if err := json.Unmarshal(msg.Data, &statusEvent); err == nil && statusEvent.ID == example.task.ID {
				log.Info("Received status update", "taskID", statusEvent.ID, "state", statusEvent.Status.State)
				example.task.Status = statusEvent.Status // Update task status

				// Print status message if any
				if statusEvent.Status.Message != nil && len(statusEvent.Status.Message.Parts) > 0 {
					if statusEvent.Status.Message.Parts[0].Type == types.PartTypeText {
						fmt.Println("\n[Agent Status: ", statusEvent.Status.Message.Parts[0].Text, "]")
					}
				}

				if statusEvent.Final {
					log.Info("Received final event flag", "taskID", statusEvent.ID)
					done <- true // Signal completion
					return
				}
				return // Processed as status event
			}

			// Try parsing as TaskArtifactUpdateEvent
			var artifactEvent types.TaskArtifactUpdateEvent
			if err := json.Unmarshal(msg.Data, &artifactEvent); err == nil && artifactEvent.ID == example.task.ID {
				log.Info("Received artifact update", "taskID", artifactEvent.ID, "artifactName", artifactEvent.Artifact.Name)
				// TODO: Handle artifact updates properly (appending parts, etc.)
				// For now, just print text parts
				for _, part := range artifactEvent.Artifact.Parts {
					if part.Type == types.PartTypeText {
						fmt.Print(part.Text) // Stream text output
					}
				}
				// If it's the last chunk of the artifact, print a newline?
				if artifactEvent.Artifact.LastChunk != nil && *artifactEvent.Artifact.LastChunk {
					fmt.Println()
				}
				return // Processed as artifact event
			}

			log.Warn("Received unknown SSE event structure", "data", data)
		})

		if err != nil {
			log.Error("SSE subscription failed", "error", err)
			errChan <- err
		} else {
			log.Info("SSE stream closed normally.")
			// If stream closes without error BUT we didn't get a final event flag,
			// it might be an unexpected closure. Signal done anyway?
			done <- true
		}
	}()

	// Wait for completion signal or error
	select {
	case <-done:
		log.Info("Task processing finished via SSE stream.", "taskID", example.task.ID, "finalState", example.task.Status.State)
	case err := <-errChan:
		log.Error("Task processing failed due to SSE error.", "taskID", example.task.ID, "error", err)
	case <-time.After(5 * time.Minute): // Add a safety timeout
		log.Warn("Task processing timed out waiting for SSE completion.", "taskID", example.task.ID)
		cancelSse() // Cancel the context to stop the SSE subscription goroutine
	}

	// Final newline for clean output
	fmt.Println()
}

func (example *DeveloperExample) setTask(prompt string) {
	v := viper.GetViper()

	example.task = types.Task{
		ID:        uuid.NewString(),
		SessionID: uuid.NewString(),
		History: []types.Message{
			{
				Role: "system",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: v.GetString("agent.developer.system"),
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
}
