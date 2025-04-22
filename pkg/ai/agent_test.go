package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
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

	agent := NewAgentFromCard(&card)

	assert.Equal(t, card, agent.card)
}

func TestAgentSend(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/rpc", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Send response in JSON-RPC format
		response := jsonrpc.RPCResponse{
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
	agent := NewAgentFromCard(&types.AgentCard{
		URL: server.URL,
	})

	// Test Send
	params := types.Task{
		ID: "test-task-id",
		History: []types.Message{
			{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: "test message",
					},
				},
			},
		},
	}
	task, err := agent.SendTask(context.Background(), params)

	assert.NoError(t, errors.New(err.Message))
	assert.Equal(t, "test-task-id", task.ID)
	assert.Equal(t, types.TaskStateWorking, task.Status.State)
}

func TestAgentGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rpc", r.URL.Path)

		// Send response in JSON-RPC format
		response := jsonrpc.RPCResponse{
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

	agent := NewAgentFromCard(&types.AgentCard{
		URL: server.URL,
	})

	task, err := agent.GetTask(context.Background(), "test-task-id", 10)

	assert.NoError(t, errors.New(err.Message))
	assert.Equal(t, "test-task-id", task.ID)
	assert.Equal(t, types.TaskStateCompleted, task.Status.State)
}

func TestAgentCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rpc", r.URL.Path)

		// Send response in JSON-RPC format
		response := jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`1`),
			Result:  nil, // Cancel operation returns null result
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	agent := NewAgentFromCard(&types.AgentCard{
		URL: server.URL,
	})

	task, err := agent.CancelTask(context.Background(), "test-task-id")
	assert.NoError(t, errors.New(err.Message))
	assert.Equal(t, "test-task-id", task.ID)
	assert.Equal(t, types.TaskStateCanceled, task.Status.State)
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

	card := NewAgentFromCard(&types.AgentCard{
		URL: server.URL,
	}).Card()

	assert.Equal(t, "http://example.com", card.URL)
	assert.True(t, card.Capabilities.Streaming)
}
