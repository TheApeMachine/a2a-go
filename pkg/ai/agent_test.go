package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/registry"
	"github.com/theapemachine/a2a-go/pkg/stores/s3"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func init() {
	// Set default configuration values for testing
	viper.SetDefault("server.defaultRPCPath", "/rpc")
	viper.SetDefault("server.defaultSSEPath", "/events")
	viper.SetDefault("provider.openai.model", "gpt-3.5-turbo")
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

// mockOpenAIProvider is a mock implementation of the OpenAI provider for testing
type mockOpenAIProvider struct {
	Execute provider.ToolExecutor
	Model   string
}

func (m *mockOpenAIProvider) Complete(ctx context.Context, task *types.Task, tools *map[string]*registry.ToolDescriptor) error {
	return nil
}

func (m *mockOpenAIProvider) Stream(ctx context.Context, task *types.Task, tools *map[string]*registry.ToolDescriptor, onDelta func(*types.Task)) error {
	return nil
}

func (m *mockOpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

func (m *mockOpenAIProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}

func TestAgentSend(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rpc", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Parse the request body to verify the method
		var req jsonrpc.RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "SendTask", req.Method)

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

	// Create agent with test server URL
	agent := NewAgentFromCard(&types.AgentCard{
		URL: server.URL,
	})

	// Initialize task store
	agent.taskStore = s3.NewConn()

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

	if err != nil {
		t.Fatalf("SendTask failed: %v", err)
	}
	assert.Equal(t, "test-task-id", task.ID)
	assert.Equal(t, types.TaskStateCompleted, task.Status.State)
}

func TestAgentGet(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rpc", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req jsonrpc.RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "GetTask", req.Method)

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

	// Create agent with test server URL
	agent := NewAgentFromCard(&types.AgentCard{
		URL: server.URL,
	})

	// Initialize task store
	agent.taskStore = s3.NewConn()

	// Create and store a task first
	task := types.Task{
		ID: "test-task-id",
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
		},
	}
	_, storeErr := agent.storeTask(context.Background(), task)
	if storeErr != nil {
		t.Fatalf("Failed to store task: %v", storeErr)
	}

	// Test Get
	result, rpcErr := agent.GetTask(context.Background(), "test-task-id", 10)
	if rpcErr != nil {
		t.Fatalf("GetTask failed: %v", rpcErr)
	}
	assert.Equal(t, "test-task-id", result.ID)
	assert.Equal(t, types.TaskStateCompleted, result.Status.State)
}

func TestAgentCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rpc", r.URL.Path)

		// Send response in JSON-RPC format
		response := jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`1`),
			Result: types.Task{
				ID: "test-task-id",
				Status: types.TaskStatus{
					State: types.TaskStateCanceled,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	agent := NewAgentFromCard(&types.AgentCard{
		URL: server.URL,
	})

	// Initialize task store
	agent.taskStore = s3.NewConn()

	// Create and store a task first
	task := types.Task{
		ID: "test-task-id",
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
	}
	_, storeErr := agent.storeTask(context.Background(), task)
	if storeErr != nil {
		t.Fatalf("Failed to store task: %v", storeErr)
	}

	// Test Cancel
	result, rpcErr := agent.CancelTask(context.Background(), "test-task-id")
	if rpcErr != nil {
		t.Fatalf("CancelTask failed: %v", rpcErr)
	}
	assert.Equal(t, "test-task-id", result.ID)
	assert.Equal(t, types.TaskStateCanceled, result.Status.State)
}

func TestFetchAgentCard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/agent.json", r.URL.Path)

		card := types.AgentCard{
			URL: r.Host,
			Capabilities: types.AgentCapabilities{
				Streaming: true,
			},
		}
		json.NewEncoder(w).Encode(card)
	}))
	defer server.Close()

	agent := NewAgentFromCard(&types.AgentCard{
		URL: server.URL,
		Capabilities: types.AgentCapabilities{
			Streaming: true,
		},
	})

	card := agent.Card()

	assert.Equal(t, server.URL, card.URL)
	assert.True(t, card.Capabilities.Streaming)
}
