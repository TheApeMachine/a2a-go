package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/sse"
	"github.com/theapemachine/a2a-go/pkg/types"
)

// AgentClient provides an interface to interact with remote A2A agents.
type AgentClient struct {
	Card      types.AgentCard
	rpcURL    string
	rpcClient *jsonrpc.RPCClient
}

// NewAgentClient creates a new agent client from an AgentCard.
func NewAgentClient(card types.AgentCard) *AgentClient {
	// Use the agent's URL to connect to the RPC endpoint
	rpcURL := fmt.Sprintf("%s/rpc", card.URL)

	return &AgentClient{
		Card:      card,
		rpcURL:    rpcURL,
		rpcClient: jsonrpc.NewRPCClient(rpcURL),
	}
}

// SendTaskRequest sends a task to the agent and returns the result.
// This is a simplified helper for text-to-text interactions.
func (client *AgentClient) SendTaskRequest(prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create a new task
	task := types.Task{
		ID:        uuid.NewString(),
		SessionID: uuid.NewString(),
		History: []types.Message{
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

	log.Debug("Sending task to agent", "agentName", client.Card.Name, "taskID", task.ID)

	// Execute the RPC call with the complete task object
	var result types.Task
	if err := client.rpcClient.Call(ctx, "tasks/send", task, &result); err != nil {
		return "", fmt.Errorf("RPC call failed: %w", err)
	}

	// Extract the result from the artifacts
	if len(result.Artifacts) > 0 && len(result.Artifacts[0].Parts) > 0 {
		for _, part := range result.Artifacts[0].Parts {
			if part.Type == types.PartTypeText {
				return part.Text, nil
			}
		}
	}

	// Check if we have a completion message in history
	if len(result.History) > 0 {
		for _, msg := range result.History {
			if msg.Role == "agent" || msg.Role == "assistant" {
				for _, part := range msg.Parts {
					if part.Type == types.PartTypeText {
						return part.Text, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no text output received from agent")
}

// StreamTask initiates a streaming task with the agent.
// The callback function will be called for each update received.
func (client *AgentClient) StreamTask(prompt string, callback func(update types.Task)) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create a new task request object (doesn't hold state)
	taskRequest := types.Task{
		ID:        uuid.NewString(),
		SessionID: uuid.NewString(),
		History: []types.Message{
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

	log.Debug("Initiating streaming task", "agentName", client.Card.Name, "taskID", taskRequest.ID)

	// First, send the task via RPC to get the initial state
	var initialTaskState types.Task
	if err := client.rpcClient.Call(ctx, "tasks/sendSubscribe", taskRequest, &initialTaskState); err != nil {
		return fmt.Errorf("RPC call failed: %w", err)
	}

	// Create a copy of the task that we'll update with incoming events
	currentTask := initialTaskState

	// --- Call callback with initial state ---
	callback(currentTask)
	// ---------------------------------------

	// Create SSE client for the task's event stream
	sseURL := fmt.Sprintf("%s/events/%s", strings.TrimSuffix(client.rpcURL, "/rpc"), initialTaskState.ID) // Use ID from response
	sseClient := sse.NewClient(sseURL)

	// Add authentication headers if needed
	if client.Card.Authentication != nil {
		for _, scheme := range client.Card.Authentication.Schemes {
			if scheme == "Bearer" && client.Card.Authentication.Credentials != nil {
				sseClient.Headers["Authorization"] = fmt.Sprintf("Bearer %s", *client.Card.Authentication.Credentials)
			}
		}
	}

	// Channel to signal completion or error
	done := make(chan error, 1)
	// Channel to signal final event received
	final := make(chan struct{})

	// Start SSE subscription in a goroutine
	go func() {
		eventHandler := func(event *sse.Event) {
			log.Debug("Received SSE event", "type", event.Event, "id", event.ID, "dataLen", len(event.Data))

			switch event.Event {
			case "artifact":
				var artifactEvent types.TaskArtifactUpdateEvent
				if err := json.Unmarshal(event.Data, &artifactEvent); err != nil {
					log.Error("Failed to unmarshal artifact event", "error", err)
					return
				}

				// Add the new artifact to our current task state
				currentTask.Artifacts = append(currentTask.Artifacts, artifactEvent.Artifact)
				log.Debug("Added artifact to task state", "artifact", artifactEvent.Artifact, "totalArtifacts", len(currentTask.Artifacts))

				// Create a deep copy before calling the callback
				taskCopy := currentTask
				callback(taskCopy)

			case "task_status":
				var statusEvent types.TaskStatusUpdateEvent
				if err := json.Unmarshal(event.Data, &statusEvent); err != nil {
					log.Error("Failed to unmarshal status event", "error", err)
					return
				}

				// Update the status in our current task state
				currentTask.Status = statusEvent.Status
				log.Debug("Updated task status", "status", statusEvent.Status)

				// Create a deep copy before calling the callback
				taskCopy := currentTask
				callback(taskCopy)

				// If it's a final status, signal that we're done
				if statusEvent.Final {
					close(final)
				}
			default:
				log.Debug("Ignoring unknown event type", "eventType", event.Event)
			}
		}

		log.Debug("Subscribing to SSE events", "url", sseURL)
		if err := sseClient.SubscribeWithContext(ctx, "", eventHandler); err != nil {
			log.Error("SSE subscription error", "error", err)
			done <- err
		}
	}()

	// Wait for completion signal or context cancellation
	select {
	case err := <-done:
		sseClient.Close()
		return err
	case <-final:
		log.Debug("Final event received, closing SSE client")
		sseClient.Close()
		return nil
	case <-ctx.Done():
		log.Debug("Context cancelled, closing SSE client")
		sseClient.Close()
		return ctx.Err()
	}
}
