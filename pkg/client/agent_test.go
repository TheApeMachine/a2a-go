package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/smarty/assertions/should"
	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func TestNewAgentClient(t *testing.T) {
	convey.Convey("Given an AgentCard", t, func() {
		card := types.AgentCard{
			Name:    "Test Agent",
			Version: "1.0.0",
			URL:     "http://test-agent:3210",
		}

		convey.Convey("When creating a new AgentClient", func() {
			client := NewAgentClient(card)

			convey.Convey("Then the client should be properly initialized", func() {
				convey.So(client.Card.Name, should.Equal, "Test Agent")
				convey.So(client.Card.Version, should.Equal, "1.0.0")
				convey.So(client.Card.URL, should.Equal, "http://test-agent:3210")
				convey.So(client.rpcURL, should.Equal, "http://test-agent:3210/rpc")
			})
		})
	})
}

func TestSendTaskRequest(t *testing.T) {
	convey.Convey("Given an AgentClient and a test server", t, func() {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" || r.URL.Path != "/rpc" {
				http.Error(w, "Invalid request", http.StatusBadRequest)
				return
			}

			// Parse request
			var req struct {
				JSONRPC string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  types.Task      `json:"params"`
				ID      json.RawMessage `json:"id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			// Send response
			w.Header().Set("Content-Type", "application/json")
			response := struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Result  types.Task      `json:"result"`
			}{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: types.Task{
					ID: req.Params.ID,
					Status: types.TaskStatus{
						State: types.TaskStateCompleted,
					},
					Artifacts: []types.Artifact{
						{
							Parts: []types.Part{
								{
									Type: types.PartTypeText,
									Text: "Test response",
								},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		// Create client
		client := NewAgentClient(types.AgentCard{
			Name:    "Test Agent",
			Version: "1.0.0",
			URL:     server.URL,
		})

		convey.Convey("When sending a task request", func() {
			response, err := client.SendTaskRequest("test prompt")

			convey.Convey("Then the response should be correct", func() {
				convey.So(err, should.BeNil)
				convey.So(response, should.Equal, "Test response")
			})
		})
	})
}

func TestStreamTask(t *testing.T) {
	convey.Convey("Given an AgentClient and a test server", t, func() {
		var taskID string

		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" && r.URL.Path == "/rpc" {
				// Handle RPC request
				var req struct {
					JSONRPC string          `json:"jsonrpc"`
					Method  string          `json:"method"`
					Params  types.Task      `json:"params"`
					ID      json.RawMessage `json:"id"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, "Invalid JSON", http.StatusBadRequest)
					return
				}

				taskID = req.Params.ID
				t.Logf("Received RPC request for task: %s", taskID)

				// Send response with initial artifact
				w.Header().Set("Content-Type", "application/json")
				response := struct {
					JSONRPC string          `json:"jsonrpc"`
					ID      json.RawMessage `json:"id"`
					Result  types.Task      `json:"result"`
				}{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: types.Task{
						ID: req.Params.ID,
						Status: types.TaskStatus{
							State: types.TaskStateWorking,
						},
						Artifacts: []types.Artifact{
							{
								Parts: []types.Part{
									{
										Type: types.PartTypeText,
										Text: "Initial response",
									},
								},
							},
						},
					},
				}
				json.NewEncoder(w).Encode(response)
				t.Log("Sent initial response")
			} else if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/events/") {
				// Handle SSE request
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
					return
				}
				flusher.Flush() // Send headers
				t.Log("SSE connection established, headers flushed")

				// --- Revert back to sending sequence of events ---
				events := []struct {
					Event string
					Data  interface{}
				}{
					{
						Event: "artifact",
						Data: types.TaskArtifactUpdateEvent{
							ID: taskID,
							Artifact: types.Artifact{
								Parts: []types.Part{
									{
										Type: types.PartTypeText,
										Text: "Test response 1",
									},
								},
							},
						},
					},
					{
						Event: "artifact",
						Data: types.TaskArtifactUpdateEvent{
							ID: taskID,
							Artifact: types.Artifact{
								Parts: []types.Part{
									{
										Type: types.PartTypeText,
										Text: "Test response 2",
									},
								},
							},
						},
					},
					{
						Event: "task_status",
						Data: types.TaskStatusUpdateEvent{
							ID: taskID,
							Status: types.TaskStatus{
								State: types.TaskStateCompleted,
							},
							Final: true,
						},
					},
				}

				for _, event := range events {
					dataBytes, _ := json.Marshal(event.Data)
					// Proper SSE format: event field + data field, each on a separate line
					fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Event, string(dataBytes))
					flusher.Flush()
					t.Logf("Sent SSE event: %s for task %s", event.Event, taskID)
					time.Sleep(10 * time.Millisecond) // Small delay between events
				}
				// --- Keep the connection open for a little while to give the client time to process ---
				time.Sleep(100 * time.Millisecond)
				t.Log("SSE handler: Finished sending events, handler returning.")
				// -------------------------------------------------
			} else {
				http.Error(w, "Not found", http.StatusNotFound)
			}
		}))
		defer server.Close()

		// Create client
		client := NewAgentClient(types.AgentCard{
			Name:    "Test Agent",
			Version: "1.0.0",
			URL:     server.URL,
		})

		convey.Convey("When streaming a task", func() {
			// --- Set log level to Debug for this test ---
			originalLevel := log.GetLevel()
			log.SetLevel(log.DebugLevel)
			defer log.SetLevel(originalLevel)
			// -------------------------------------------\n
			var finalUpdate types.Task
			done := make(chan struct{})

			// Start streaming
			go func() {
				err := client.StreamTask("test prompt", func(task types.Task) {
					// This is the test's callback
					t.Logf("Received task update: status=%v, artifacts=%d, taskID=%s", task.Status.State, len(task.Artifacts), task.ID)
					for i, art := range task.Artifacts {
						t.Logf("  Artifact[%d]: %s", i, art.Parts[0].Text)
					}
					if task.Status.State == types.TaskStateCompleted {
						finalUpdate = task // Capture the state passed in the final status callback
						close(done)
					}
				})
				if err != nil {
					t.Errorf("StreamTask error: %v", err)
				}
			}()

			// Wait for completion or timeout
			select {
			case <-done:
				t.Logf("Final update captured: artifacts=%d", len(finalUpdate.Artifacts))
				for i, art := range finalUpdate.Artifacts {
					t.Logf("  Final Artifact[%d]: %s", i, art.Parts[0].Text)
				}
				// Assertions on the captured final state - do this in the main goroutine
				convey.So(finalUpdate.Status.State, convey.ShouldEqual, types.TaskStateCompleted)
				convey.So(len(finalUpdate.Artifacts), convey.ShouldEqual, 3) // Expect 3 artifacts
				if len(finalUpdate.Artifacts) >= 3 {
					convey.So(finalUpdate.Artifacts[2].Parts[0].Text, convey.ShouldEqual, "Test response 2")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Test timed out")
			}
		})
	})
}
