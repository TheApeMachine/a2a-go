package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/tj/assert"
)

const (
	eventsPath  = "/events"
	testURL     = "http://test.local"
	contentType = "Content-Type"
	jsonContent = "application/json"
)

func TestNewA2AServerWithDefaults(t *testing.T) {
	server := NewA2AServerWithDefaults(testURL)

	// Verify server configuration
	assert.NotNil(t, server)
	assert.Equal(t, testURL, server.Card.URL)
	assert.True(t, server.Card.Capabilities.Streaming)
	assert.True(t, server.Card.Capabilities.PushNotifications)
	assert.True(t, server.Card.Capabilities.StateTransitionHistory)
	assert.Contains(t, server.Card.DefaultInputModes, "text/plain")
	assert.Contains(t, server.Card.DefaultOutputModes, "text/plain")
	assert.NotEmpty(t, server.Card.Skills)

	// Verify handlers are registered
	handlers := server.Handlers()
	assert.NotNil(t, handlers["/rpc"])
	assert.NotNil(t, handlers[eventsPath])
}

func TestA2AServerRPCEndpoint(t *testing.T) {
	server := NewA2AServerWithDefaults(testURL)
	rpcHandler := server.Handlers()["/rpc"]

	tests := []struct {
		name       string
		method     string
		params     interface{}
		wantStatus int
		wantError  bool
	}{
		{
			name:   "tasks/send",
			method: "tasks/send",
			params: types.TaskSendParams{
				ID: "test-task",
				Message: types.Message{
					Role: "user",
					Parts: []types.Part{
						{Type: types.PartTypeText, Text: "test message"},
					},
				},
			},
			wantStatus: http.StatusOK,
			wantError:  false,
		},
		{
			name:       "invalid method",
			method:     "invalid/method",
			params:     struct{}{},
			wantStatus: http.StatusOK, // JSON-RPC always returns 200 OK
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create JSON-RPC request
			reqBody := RPCRequest{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`1`),
				Method:  tt.method,
			}
			if tt.params != nil {
				params, _ := json.Marshal(tt.params)
				reqBody.Params = params
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(string(body)))
			req.Header.Set(contentType, jsonContent)
			rec := httptest.NewRecorder()

			rpcHandler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)

			var response RPCResponse
			err := json.NewDecoder(rec.Body).Decode(&response)
			assert.NoError(t, err)

			if tt.wantError {
				assert.NotNil(t, response.Error)
			} else {
				assert.Nil(t, response.Error)
				assert.NotNil(t, response.Result)
			}
		})
	}
}

func TestA2AServerSSEEndpoint(t *testing.T) {
	server := NewA2AServerWithDefaults(testURL)
	sseHandler := server.Handlers()[eventsPath]

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, eventsPath, nil)
	rec := httptest.NewRecorder()

	go func() {
		sseHandler.ServeHTTP(rec, req)
	}()

	time.Sleep(100 * time.Millisecond)

	// Verify SSE response headers
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))
}

func TestA2AServerTaskStreaming(t *testing.T) {
	server := NewA2AServerWithDefaults(testURL)
	rpcHandler := server.Handlers()["/rpc"]

	// Create streaming task request
	reqBody := RPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tasks/sendSubscribe",
		Params: json.RawMessage(`{
			"id": "test-stream",
			"message": {
				"role": "user",
				"parts": [{"type": "text", "text": "test message"}]
			}
		}`),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(string(body)))
	req.Header.Set(contentType, jsonContent)
	rec := httptest.NewRecorder()

	rpcHandler.ServeHTTP(rec, req)

	// Verify initial response
	assert.Equal(t, http.StatusOK, rec.Code)

	var response RPCResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Nil(t, response.Error)

	// Verify response contains initial status
	var event types.TaskStatusUpdateEvent
	resultBytes, _ := json.Marshal(response.Result)
	err = json.Unmarshal(resultBytes, &event)
	assert.NoError(t, err)
	assert.Equal(t, "test-stream", event.ID)
	assert.Equal(t, types.TaskStateWorking, event.Status.State)
	assert.False(t, event.Final)
}

func TestA2AServerPrompts(t *testing.T) {
	server := NewA2AServerWithDefaults(testURL)
	rpcHandler := server.Handlers()["/rpc"]

	// Test prompts/list
	listReq := RPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "prompts/list",
	}
	body, _ := json.Marshal(listReq)

	req := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(string(body)))
	req.Header.Set(contentType, jsonContent)
	rec := httptest.NewRecorder()

	rpcHandler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response RPCResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Nil(t, response.Error)
	assert.NotNil(t, response.Result)
}
