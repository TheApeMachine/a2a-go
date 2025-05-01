package client

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
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

	// Use the TaskSendParams to send the request according to A2A protocol
	sessionID := task.SessionID
	params := types.TaskSendParams{
		ID:        task.ID,
		SessionID: &sessionID,
		Message:   task.History[0],
	}

	log.Debug("Sending task to agent", "agentName", client.Card.Name, "taskID", task.ID)

	// Execute the RPC call
	var result types.Task
	if err := client.rpcClient.Call(ctx, "tasks/send", params, &result); err != nil {
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
	// Implementation for streaming would go here
	// This would use the tasks/sendSubscribe endpoint and SSE
	return fmt.Errorf("streaming not implemented yet")
}
