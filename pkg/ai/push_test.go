package ai

// import (
// 	"bytes"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"

// 	. "github.com/smartystreets/goconvey/convey"
// 	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
// 	"github.com/theapemachine/a2a-go/pkg/push"
// 	"github.com/theapemachine/a2a-go/pkg/stores/s3"
// 	"github.com/theapemachine/a2a-go/pkg/types"
// )

// // MockStore is a local type that embeds s3.Conn
// type MockStore struct {
// 	*s3.Conn
// }

// // Get overrides the Get method for tests
// func (m *MockStore) Get(_ context.Context, _, _ string) (io.ReadSeeker, error) {
// 	taskJSON := `{
// 		"id": "test-task-id",
// 		"status": {
// 			"state": "working"
// 		}
// 	}`
// 	return bytes.NewReader([]byte(taskJSON)), nil
// }

// // NewMockStore creates a new mock store
// func NewMockStore() *MockStore {
// 	return &MockStore{Conn: &s3.Conn{}}
// }

// // Verify that GetPushNotification is properly mocked
// func TestResubscribe(t *testing.T) {
// 	Convey("Given an agent with a task", t, func() {
// 		// Setup a mock server to handle RPC calls
// 		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			// Check request body to determine which RPC call this is
// 			var req jsonrpc.RPCRequest
// 			json.NewDecoder(r.Body).Decode(&req)

// 			// Return appropriate mock data based on the method
// 			if req.Method == "get_task" {
// 				// Mock GetTask response
// 				resp := jsonrpc.RPCResponse{
// 					JSONRPC: "2.0",
// 					ID:      req.ID,
// 					Result: types.Task{
// 						ID: "test-task-id",
// 						Status: types.TaskStatus{
// 							State: types.TaskStateWorking,
// 						},
// 					},
// 				}
// 				json.NewEncoder(w).Encode(resp)
// 			} else if req.Method == "get_push_notification" {
// 				// Mock GetPushNotification response
// 				resp := jsonrpc.RPCResponse{
// 					JSONRPC: "2.0",
// 					ID:      req.ID,
// 					Result: types.TaskPushNotificationConfig{
// 						ID: "test-task-id",
// 						PushNotificationConfig: types.PushNotificationConfig{
// 							URL: "http://example.com/events",
// 						},
// 					},
// 				}
// 				json.NewEncoder(w).Encode(resp)
// 			}
// 		}))
// 		defer server.Close()

// 		// Setup SSE server to mock events
// 		sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			w.Header().Set("Content-Type", "text/event-stream")
// 			w.Header().Set("Cache-Control", "no-cache")
// 			w.Header().Set("Connection", "keep-alive")

// 			// Send a sample event
// 			fmt.Fprintf(w, "event: task_status\ndata: {\"state\":\"working\"}\n\n")
// 			w.(http.Flusher).Flush()
// 		}))
// 		defer sseServer.Close()

// 		// Create agent with rpc client pointed to our mock server
// 		agent := &Agent{
// 			card: types.AgentCard{
// 				Name: "test-agent",
// 				URL:  server.URL,
// 			},
// 			rpc:         jsonrpc.NewRPCClient(server.URL),
// 			pushService: push.NewService(),
// 			Logger: func(format string, args ...any) {
// 				// No-op logger for testing
// 			},
// 		}

// 		// Set up push notification configuration
// 		pushConfig := &types.TaskPushNotificationConfig{
// 			ID: "test-task-id",
// 			PushNotificationConfig: types.PushNotificationConfig{
// 				URL: sseServer.URL,
// 			},
// 		}
// 		agent.SetPush(context.Background(), pushConfig)

// 		Convey("When resubscribing to the task", func() {
// 			err := agent.Resubscribe(context.Background(), "test-task-id")

// 			Convey("Then it should succeed", func() {
// 				So(err, ShouldBeNil)
// 			})
// 		})
// 	})
// }

// func TestSetPush(t *testing.T) {
// 	Convey("Given an agent", t, func() {
// 		agent := &Agent{
// 			card: types.AgentCard{
// 				Name: "test-agent",
// 			},
// 			pushService: push.NewService(),
// 		}

// 		Convey("When setting push notification config", func() {
// 			config := &types.TaskPushNotificationConfig{
// 				ID: "test-task-id",
// 				PushNotificationConfig: types.PushNotificationConfig{
// 					URL: "http://example.com/events",
// 				},
// 			}
// 			err := agent.SetPush(context.Background(), config)

// 			Convey("Then it should succeed", func() {
// 				So(err, ShouldBeNil)
// 			})

// 			Convey("And the config should be retrievable", func() {
// 				storedConfig, err := agent.GetPush(context.Background(), "test-task-id")
// 				So(err, ShouldBeNil)
// 				So(storedConfig, ShouldNotBeNil)
// 				So(storedConfig.ID, ShouldEqual, "test-task-id")
// 				So(storedConfig.PushNotificationConfig.URL, ShouldEqual, "http://example.com/events")
// 			})
// 		})
// 	})
// }

// func TestGetPush(t *testing.T) {
// 	Convey("Given an agent", t, func() {
// 		agent := &Agent{
// 			card: types.AgentCard{
// 				Name: "test-agent",
// 			},
// 			pushService: push.NewService(),
// 		}

// 		Convey("When no push config exists", func() {
// 			config, err := agent.GetPush(context.Background(), "non-existent-task")

// 			Convey("Then it should return an error", func() {
// 				So(err, ShouldNotBeNil)
// 				So(config, ShouldBeNil)
// 			})
// 		})

// 		Convey("When push config exists", func() {
// 			// First set a config
// 			testConfig := &types.TaskPushNotificationConfig{
// 				ID: "test-task-id",
// 				PushNotificationConfig: types.PushNotificationConfig{
// 					URL: "http://example.com/events",
// 				},
// 			}
// 			agent.SetPush(context.Background(), testConfig)

// 			// Then retrieve it
// 			config, err := agent.GetPush(context.Background(), "test-task-id")

// 			Convey("Then it should return the config", func() {
// 				So(err, ShouldBeNil)
// 				So(config, ShouldNotBeNil)
// 				So(config.ID, ShouldEqual, "test-task-id")
// 				So(config.PushNotificationConfig.URL, ShouldEqual, "http://example.com/events")
// 			})
// 		})
// 	})
// }

// // TestSendPushNotification tests the SendPushNotification function
// func TestSendPushNotification(t *testing.T) {
// 	// Tests with just empty struct
// 	Convey("Given an agent with no push service", t, func() {
// 		agent := &Agent{
// 			card: types.AgentCard{
// 				Name: "test-agent",
// 			},
// 			pushService: nil,
// 		}

// 		Convey("When sending a push notification", func() {
// 			err := agent.SendPushNotification("test-task-id", "test")

// 			Convey("Then it should return an error", func() {
// 				So(err, ShouldNotBeNil)
// 				So(err.Error(), ShouldContainSubstring, "not initialized")
// 			})
// 		})
// 	})
// }
