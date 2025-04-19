package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/tj/assert"
)

func init() {
	// Set default configuration values for testing
	viper.SetDefault("server.defaultRPCPath", "/rpc")
	viper.SetDefault("server.defaultSSEPath", "/events")
}

func TestNewAgentFromCard(t *testing.T) {
	card := types.AgentCard{
		URL: "http://example.com",
		Capabilities: types.AgentCapabilities{
			Streaming: true,
		},
	}

	agent := NewAgentFromCard(card)

	assert.Equal(t, card, agent.Card)
	assert.Equal(t, "http://example.com/rpc", agent.rpcEndpoint)
	assert.Equal(t, "http://example.com/events", agent.sseEndpoint)
}

func TestAgentSend(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/rpc", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Send response in JSON-RPC format
		response := service.RPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`1`),
			Result: types.Task{
				ID: "test-task-id",
				Status: types.TaskStatus{
					State: types.TaskStateWorking,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create agent with test server URL
	agent := NewAgentFromCard(types.AgentCard{
		URL: server.URL,
	})

	// Test Send
	params := types.TaskSendParams{
		ID: "test-task-id",
		Message: types.Message{
			Role: "user",
			Parts: []types.Part{
				{
					Type: types.PartTypeText,
					Text: "test message",
				},
			},
		},
	}
	task, err := agent.Send(context.Background(), params)

	assert.NoError(t, err)
	assert.Equal(t, "test-task-id", task.ID)
	assert.Equal(t, types.TaskStateWorking, task.Status.State)
}

func TestAgentGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rpc", r.URL.Path)

		// Send response in JSON-RPC format
		response := service.RPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`1`),
			Result: types.Task{
				ID: "test-task-id",
				Status: types.TaskStatus{
					State: types.TaskStateCompleted,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	agent := NewAgentFromCard(types.AgentCard{
		URL: server.URL,
	})

	task, err := agent.Get(context.Background(), "test-task-id", 10)

	assert.NoError(t, err)
	assert.Equal(t, "test-task-id", task.ID)
	assert.Equal(t, types.TaskStateCompleted, task.Status.State)
}

func TestAgentCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rpc", r.URL.Path)

		// Send response in JSON-RPC format
		response := service.RPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`1`),
			Result:  nil, // Cancel operation returns null result
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	agent := NewAgentFromCard(types.AgentCard{
		URL: server.URL,
	})

	err := agent.Cancel(context.Background(), "test-task-id")
	assert.NoError(t, err)
}

func TestAgentSendStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rpc", r.URL.Path)

		// Set headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send status update
		statusEvent := types.TaskStatusUpdateEvent{
			ID: "test-task-id",
			Status: types.TaskStatus{
				State: types.TaskStateWorking,
			},
		}
		data, _ := json.Marshal(statusEvent)
		w.Write([]byte("data: " + string(data) + "\n\n"))

		// Send artifact update
		artifactEvent := types.TaskArtifactUpdateEvent{
			ID: "test-task-id",
			Artifact: types.Artifact{
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: "test artifact content",
					},
				},
			},
		}
		data, _ = json.Marshal(artifactEvent)
		w.Write([]byte("data: " + string(data) + "\n\n"))

		// Send final status
		finalEvent := types.TaskStatusUpdateEvent{
			ID: "test-task-id",
			Status: types.TaskStatus{
				State: types.TaskStateCompleted,
			},
			Final: true,
		}
		data, _ = json.Marshal(finalEvent)
		w.Write([]byte("data: " + string(data) + "\n\n"))
	}))
	defer server.Close()

	agent := NewAgentFromCard(types.AgentCard{
		URL: server.URL,
		Capabilities: types.AgentCapabilities{
			Streaming: true,
		},
	})

	var statusUpdates []types.TaskStatusUpdateEvent
	var artifactUpdates []types.TaskArtifactUpdateEvent

	err := agent.SendStream(
		context.Background(),
		types.TaskSendParams{
			ID: "test-task-id",
			Message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: "test message",
					},
				},
			},
		},
		func(evt types.TaskStatusUpdateEvent) {
			statusUpdates = append(statusUpdates, evt)
		},
		func(evt types.TaskArtifactUpdateEvent) {
			artifactUpdates = append(artifactUpdates, evt)
		},
	)

	assert.NoError(t, err)
	assert.Len(t, statusUpdates, 2)
	assert.Len(t, artifactUpdates, 1)
	assert.Equal(t, types.TaskStateWorking, statusUpdates[0].Status.State)
	assert.Equal(t, types.TaskStateCompleted, statusUpdates[1].Status.State)
	assert.True(t, statusUpdates[1].Final)
	assert.Equal(t, "test artifact content", artifactUpdates[0].Artifact.Parts[0].Text)
}

func TestFetchAgentCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/agent.json", r.URL.Path)

		card := types.AgentCard{
			URL: "http://example.com",
			Capabilities: types.AgentCapabilities{
				Streaming: true,
			},
		}
		json.NewEncoder(w).Encode(card)
	}))
	defer server.Close()

	agent, err := FetchAgentCard(context.Background(), server.URL)

	assert.NoError(t, err)
	assert.Equal(t, "http://example.com", agent.Card.URL)
	assert.True(t, agent.Card.Capabilities.Streaming)
}
