package client

import (
	"context"
	"encoding/json"
	"fmt"
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

	log.Debug("Initiating streaming task", "agentName", client.Card.Name, "taskID", task.ID)

	// First, send the task via RPC to get the initial state
	var result types.Task
	if err := client.rpcClient.Call(ctx, "tasks/sendSubscribe", task, &result); err != nil {
		return fmt.Errorf("RPC call failed: %w", err)
	}

	// Create SSE client for the task's event stream
	sseURL := fmt.Sprintf("%s/events/%s", client.rpcURL, task.ID)
	sseClient := sse.NewClient(sseURL)

	// Add authentication headers if needed
	if client.Card.Authentication != nil {
		for _, scheme := range client.Card.Authentication.Schemes {
			if scheme == "Bearer" && client.Card.Authentication.Credentials != nil {
				sseClient.Headers["Authorization"] = fmt.Sprintf("Bearer %s", *client.Card.Authentication.Credentials)
			}
		}
	}

	// Channel for backpressure control
	backpressure := make(chan struct{}, 10) // Allow up to 10 pending updates

	// Channel to signal completion or error
	done := make(chan error, 1)

	// Start SSE subscription in a goroutine
	go func() {
		defer close(done)

		// Retry loop for reconnection
		for retries := 0; retries < 3; retries++ {
			if retries > 0 {
				log.Info("Reconnecting to SSE stream", "taskID", task.ID, "attempt", retries)
				sseClient.Metrics.RecordReconnection()
				time.Sleep(time.Second * time.Duration(retries)) // Exponential backoff
			}

			err := sseClient.SubscribeWithContext(ctx, "", func(msg *sse.Event) {
				// Apply backpressure
				select {
				case backpressure <- struct{}{}:
				default:
					log.Warn("Backpressure limit reached, dropping update", "taskID", task.ID)
					sseClient.Metrics.RecordEvent(true, 0, 0)
					return
				}

				// Process the event
				data := string(msg.Data)
				if data == "" {
					<-backpressure
					return
				}

				// Try parsing as TaskStatusUpdateEvent
				var statusEvent types.TaskStatusUpdateEvent
				if err := json.Unmarshal(msg.Data, &statusEvent); err == nil && statusEvent.ID == task.ID {
					// Update task status
					task.Status = statusEvent.Status
					callback(task)

					if statusEvent.Final {
						done <- nil
						return
					}
					<-backpressure
					return
				}

				// Try parsing as TaskArtifactUpdateEvent
				var artifactEvent types.TaskArtifactUpdateEvent
				if err := json.Unmarshal(msg.Data, &artifactEvent); err == nil && artifactEvent.ID == task.ID {
					// Update task artifacts
					task.Artifacts = append(task.Artifacts, artifactEvent.Artifact)
					callback(task)
					<-backpressure
					return
				}

				log.Warn("Received unknown SSE event structure", "data", data)
				<-backpressure
			})

			if err == nil {
				// Normal completion
				return
			}

			if ctx.Err() != nil {
				// Context cancelled
				done <- ctx.Err()
				return
			}

			log.Error("SSE subscription failed", "error", err, "taskID", task.ID)
		}

		// All retries failed
		done <- fmt.Errorf("failed to establish SSE connection after 3 attempts")
	}()

	// Wait for completion or error
	return <-done
}
