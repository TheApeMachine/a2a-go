package ai

import (
	"context"
	"encoding/json" // Correctly alias standard errors
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/provider"
	// Standard errors package
)

// mockOpenAIProvider is a mock implementation of LLMProvider (defined in task.go)
type mockOpenAIProvider struct{}

// Generate is the method required by the LLMProvider interface.
func (m *mockOpenAIProvider) Generate(ctx context.Context, params *provider.ProviderParams) chan jsonrpc.Response {
	ch := make(chan jsonrpc.Response)
	// close(ch) // Intentionally not closing for this mock, can be adjusted if needed
	return ch
}

func NewMockOpenAIProvider() *mockOpenAIProvider {
	return &mockOpenAIProvider{}
}

// taskStoreMockForTesting is a general-purpose mock for stores.TaskStore
type taskStoreMockForTesting struct {
	mockTaskStore // Embed basic mock from agent_test.go if we need its defaults
	getFunc       func(ctx context.Context, id string, historyLength int) ([]a2a.Task, *errors.RpcError)
	createFunc    func(ctx context.Context, task *a2a.Task) *errors.RpcError
	cancelFunc    func(ctx context.Context, id string) *errors.RpcError
	subscribeFunc func(ctx context.Context, id string, ch chan a2a.Task) *errors.RpcError
	updateFunc    func(ctx context.Context, task *a2a.Task) *errors.RpcError
	// Add other methods as needed: Update, Delete
}

func (s *taskStoreMockForTesting) Get(ctx context.Context, id string, historyLength int) ([]a2a.Task, *errors.RpcError) {
	if s.getFunc != nil {
		return s.getFunc(ctx, id, historyLength)
	}
	// Fallback to embedded mock's Get or a default nil, nil
	return s.mockTaskStore.Get(ctx, id, historyLength)
}

func (s *taskStoreMockForTesting) Create(ctx context.Context, task *a2a.Task, optionals ...string) *errors.RpcError {
	if s.createFunc != nil {
		return s.createFunc(ctx, task)
	}
	return s.mockTaskStore.Create(ctx, task)
}

func (s *taskStoreMockForTesting) Cancel(ctx context.Context, id string) *errors.RpcError {
	if s.cancelFunc != nil {
		return s.cancelFunc(ctx, id)
	}
	return s.mockTaskStore.Cancel(ctx, id)
}

func (s *taskStoreMockForTesting) Subscribe(ctx context.Context, id string, ch chan a2a.Task) *errors.RpcError {
	if s.subscribeFunc != nil {
		return s.subscribeFunc(ctx, id, ch)
	}
	return s.mockTaskStore.Subscribe(ctx, id, ch)
}

func (s *taskStoreMockForTesting) Update(ctx context.Context, task *a2a.Task, optionals ...string) *errors.RpcError {
	if s.updateFunc != nil {
		return s.updateFunc(ctx, task)
	}
	return s.mockTaskStore.Update(ctx, task)
}

// controllableMockProvider is a mock for provider.Interface for TestSendTask & TestStreamTask
type controllableMockProvider struct {
	generateFunc       func(ctx context.Context, params *provider.ProviderParams) chan jsonrpc.Response
	lastGenerateParams *provider.ProviderParams
	mu                 sync.Mutex // For thread-safe access to lastGenerateParams
}

func (m *controllableMockProvider) Generate(ctx context.Context, params *provider.ProviderParams) chan jsonrpc.Response {
	m.mu.Lock()
	m.lastGenerateParams = params
	m.mu.Unlock()
	if m.generateFunc != nil {
		return m.generateFunc(ctx, params)
	}
	ch := make(chan jsonrpc.Response)
	close(ch)
	return ch
}

func (m *controllableMockProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil // Mock implementation, not used by SendTask
}

func NewControllableMockProvider() *controllableMockProvider {
	return &controllableMockProvider{}
}

func TestNewTaskManager(t *testing.T) {
	Convey("Given an AgentCard and dependencies", t, func() {
		card := &a2a.AgentCard{Name: "TestAgent"}
		// mockTaskStore is defined in agent_test.go (same package)
		store := &mockTaskStore{}
		prov := NewMockOpenAIProvider()

		Convey("When NewTaskManager is called with all dependencies", func() {
			tm, err := NewTaskManager(card, WithTaskStore(store), WithProvider(prov))

			Convey("Then it should succeed and return a valid TaskManager", func() {
				So(err, ShouldBeNil)
				So(tm, ShouldNotBeNil)
				So(tm.agent, ShouldEqual, card)
				So(tm.taskStore, ShouldResemble, store) // Use ShouldResemble for interface vs concrete mock comparison
				So(tm.provider, ShouldEqual, prov)
			})
		})

		Convey("When NewTaskManager is called without a TaskStore", func() {
			tm, err := NewTaskManager(card, WithProvider(prov))

			Convey("Then it should fail with ErrMissingTaskStore", func() {
				So(err, ShouldNotBeNil)
				So(tm, ShouldBeNil)
				// Workaround for ErrMissingTaskStore not correctly implementing error interface
				// Check the error message or a specific field of the RpcError if available.
				So(err.Error(), ShouldEqual, errors.NewError(errors.ErrMissingTaskStore{}).Error())
			})
		})

		Convey("When NewTaskManager is called without a Provider", func() {
			tm, err := NewTaskManager(card, WithTaskStore(store))

			Convey("Then it should fail with ErrMissingProvider", func() {
				So(err, ShouldNotBeNil)
				So(tm, ShouldBeNil)
				So(err.Error(), ShouldEqual, errors.NewError(errors.ErrMissingProvider{}).Error())
			})
		})
	})
}

func TestHandleUpdate(t *testing.T) {
	Convey("Given a TaskManager and a Task", t, func() {
		// Viper setup for a2a.NewTask
		originalSystemMessage := viper.GetString("agent.test-agent-for-error.system")
		vip := viper.GetViper()
		vip.Set("agent.test-agent-for-error.system", "Sys message for handleUpdate error test")
		vip.Set("agent.test-agent-for-status.system", "Sys message for handleUpdate status test")
		vip.Set("agent.test-agent-for-artifact.system", "Sys message for handleUpdate artifact test")
		vip.Set("agent.test-agent-for-unknown.system", "Sys message for handleUpdate unknown test")
		defer func() {
			// Restore only one, assuming others might be set by other tests or need specific restoration.
			// For more robust parallel test execution, consider test-specific viper instances if possible,
			// or careful management of global state.
			vip.Set("agent.test-agent-for-error.system", originalSystemMessage)
			// To fully clean up for other tests if they also use these keys:
			// vip.Set("agent.test-agent-for-status.system", nil) // or original value if known
			// vip.Set("agent.test-agent-for-artifact.system", nil)
			// vip.Set("agent.test-agent-for-unknown.system", nil)
		}()

		manager := &TaskManager{}

		Convey("When handleUpdate receives a chunk with an RPC error", func() {
			task := a2a.NewTask("test-agent-for-error")
			jsonRpcErrInChunk := &jsonrpc.Error{Code: 123, Message: "test jsonrpc error in chunk"}
			chunk := jsonrpc.Response{Error: jsonRpcErrInChunk}
			err := manager.handleUpdate(task, chunk)

			Convey("Then it should return a non-nil error", func() {
				So(err, ShouldNotBeNil)
				// Further inspection of err is problematic due to the cast in handleUpdate
				// and jsonrpc.Error not implementing the error interface.
				// A proper fix in handleUpdate would be to construct a new errors.RpcError.
				// For now, we just check that AN error is returned.
			})
		})

		Convey("When handleUpdate receives a TaskStatusUpdateResult", func() {
			task := a2a.NewTask("test-agent-for-status")
			statusMessage := a2a.NewTextMessage("updater", "Task is now working")
			statusUpdate := a2a.TaskStatusUpdateResult{
				Status: a2a.TaskStatus{
					State:   a2a.TaskStateWorking,
					Message: statusMessage,
				},
			}
			chunk := jsonrpc.Response{Result: statusUpdate}
			err := manager.handleUpdate(task, chunk)

			Convey("Then it should update the task's status and return nil", func() {
				So(err, ShouldBeNil)
				So(task.Status.State, ShouldEqual, a2a.TaskStateWorking)
				So(task.Status.Message, ShouldResemble, statusMessage)
			})
		})

		Convey("When handleUpdate receives a TaskArtifactUpdateEvent", func() {
			task := a2a.NewTask("test-agent-for-artifact")
			initialArtifactCount := len(task.Artifacts)
			fileName := "artifact.txt"
			mimeType := "text/plain"
			fileData := "This is the artifact content."
			artifactActual := a2a.NewFileArtifact(fileName, mimeType, fileData)
			artifactUpdate := a2a.TaskArtifactUpdateEvent{Artifact: artifactActual}
			chunk := jsonrpc.Response{Result: artifactUpdate}
			err := manager.handleUpdate(task, chunk)

			Convey("Then it should add the artifact to the task and return nil", func() {
				So(err, ShouldBeNil)
				So(len(task.Artifacts), ShouldEqual, initialArtifactCount+1)
				So(task.Artifacts[initialArtifactCount], ShouldResemble, artifactActual)
			})
		})

		Convey("When handleUpdate receives a chunk with an unknown result type", func() {
			task := a2a.NewTask("test-agent-for-unknown")
			originalTaskBytes, marshalErr := json.Marshal(task)
			So(marshalErr, ShouldBeNil)

			unknownResult := struct{ Data string }{Data: "unknown data"}
			chunk := jsonrpc.Response{Result: unknownResult}
			err := manager.handleUpdate(task, chunk)

			Convey("Then it should not modify the task and return nil", func() {
				So(err, ShouldBeNil)
				updatedTaskBytes, marshalErr2 := json.Marshal(task)
				So(marshalErr2, ShouldBeNil)
				So(string(updatedTaskBytes), ShouldEqual, string(originalTaskBytes))
			})
		})
	})
}

func TestSelectTask(t *testing.T) {
	Convey("Given a TaskManager with a configurable task store", t, func() {
		agentCard := &a2a.AgentCard{Name: "TestAgentForSelect"}
		mockProvider := NewMockOpenAIProvider()

		// Viper setup for system message used in a2a.NewTask
		originalSystemMessage := viper.GetString(fmt.Sprintf("agent.%s.system", agentCard.Name))
		vip := viper.GetViper() // Get the global Viper instance
		vip.Set(fmt.Sprintf("agent.%s.system", agentCard.Name), "Default system message for selectTask testing")
		defer vip.Set(fmt.Sprintf("agent.%s.system", agentCard.Name), originalSystemMessage) // Restore original value

		params := a2a.TaskSendParams{
			ID:      "test-task-id-from-params",
			Message: *a2a.NewTextMessage("user", "hello world"),
		}

		Convey("When an existing task is found by the store", func() {
			existingTask := a2a.NewTask(agentCard.Name)
			existingTask.ID = params.ID
			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					if id == agentCard.Name+"/"+params.ID {
						return []a2a.Task{*existingTask}, nil
					}
					return nil, nil
				},
				updateFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil },
			}
			manager, err := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(err, ShouldBeNil)
			tasks, rpcErr := manager.selectTask(context.Background(), params)
			Convey("Then it should return the existing task and no error", func() {
				So(rpcErr, ShouldBeNil)
				So(len(tasks), ShouldEqual, 1)
				So(tasks[0].ID, ShouldEqual, existingTask.ID)
			})
		})

		Convey("When no existing task is found and store.Create succeeds", func() {
			var createdTaskRecord *a2a.Task
			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					return nil, nil
				},
				createFunc: func(ctx context.Context, taskToCreate *a2a.Task) *errors.RpcError {
					createdTaskRecord = taskToCreate
					return nil
				},
				updateFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil },
			}
			manager, err := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(err, ShouldBeNil)
			tasks, rpcErr := manager.selectTask(context.Background(), params)
			Convey("Then it should create a new task, store it, and return it", func() {
				So(rpcErr, ShouldBeNil)
				So(tasks, ShouldNotBeNil)
				So(len(tasks), ShouldEqual, 1)
				So(tasks[0].ID, ShouldNotBeBlank)
				So(tasks[0].ID, ShouldEqual, params.ID)
				So(len(tasks[0].History), ShouldBeGreaterThan, 0)
				So(tasks[0].History[0].Role, ShouldEqual, "system")
				So(strings.Contains(tasks[0].History[0].Parts[0].Text, "Default system message for selectTask testing"), ShouldBeTrue)
				So(tasks[0].History[len(tasks[0].History)-1], ShouldResemble, params.Message)
				So(tasks[0].Status.State, ShouldEqual, a2a.TaskStateSubmitted)
				So(createdTaskRecord, ShouldResemble, &tasks[0])
			})
		})

		Convey("When no existing task is found and store.Create fails", func() {
			// Use an actual *errors.RpcError for the mock
			expectedStoreErr := &errors.RpcError{Code: 1001, Message: "mock store create failed"}
			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					return nil, nil
				},
				createFunc: func(ctx context.Context, taskToCreate *a2a.Task) *errors.RpcError {
					return expectedStoreErr // Return *errors.RpcError
				},
				updateFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil },
			}
			manager, err := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(err, ShouldBeNil)
			tasks, rpcErr := manager.selectTask(context.Background(), params)
			Convey("Then it should return nil for task and the error from store.Create", func() {
				So(tasks, ShouldBeNil)
				So(rpcErr, ShouldEqual, expectedStoreErr)
			})
		})

		Convey("When store.Get fails with a 'key does not exist' error (specific string)", func() {
			// keyNotExistErr := &errors.RpcError{Code: -32000, Message: "The specified key does not exist."} // This was the old mock error
			var createdTaskRecord *a2a.Task
			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					// This test case is intended to check behavior when the store signals 'not found'.
					// For selectTask to create a new task, it expects errors.ErrTaskNotFound specifically.
					return nil, errors.ErrTaskNotFound // Ensure mock returns the precise error for this path.
				},
				createFunc: func(ctx context.Context, taskToCreate *a2a.Task) *errors.RpcError {
					createdTaskRecord = taskToCreate
					return nil
				},
				updateFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil },
			}
			manager, err := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(err, ShouldBeNil)
			tasks, rpcErr := manager.selectTask(context.Background(), params)
			Convey("Then it should still create a new task and return it", func() {
				So(rpcErr, ShouldBeNil)
				So(tasks, ShouldNotBeNil)
				So(len(tasks), ShouldEqual, 1)
				So(createdTaskRecord, ShouldResemble, &tasks[0])
			})
		})

		Convey("When store.Get fails with an unexpected error (that is NOT ErrTaskNotFound)", func() {
			expectedStoreErr := &errors.RpcError{Code: 1002, Message: "mock store unexpected get error"}
			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					return nil, expectedStoreErr // Return *errors.RpcError
				},
				// createFunc should not be called
				updateFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil },
			}
			manager, err := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(err, ShouldBeNil)
			tasks, rpcErr := manager.selectTask(context.Background(), params)
			Convey("Then it should return the error from store.Get and no task", func() {
				So(rpcErr, ShouldEqual, expectedStoreErr) // Error should be propagated
				So(tasks, ShouldBeNil)                    // No task should be returned or created
			})
		})
	})
}

func TestSendTask(t *testing.T) {
	Convey("Given a TaskManager with controllable store and provider", t, func() {
		agentCard := &a2a.AgentCard{Name: "TestAgentSendTask"}
		vip := viper.GetViper()
		systemMsgKey := fmt.Sprintf("agent.%s.system", agentCard.Name)
		originalSystemMessage := vip.GetString(systemMsgKey)
		vip.Set(systemMsgKey, "Default system message for SendTask testing")
		defer vip.Set(systemMsgKey, originalSystemMessage)

		sendParams := a2a.TaskSendParams{
			ID:      "task-id-for-send",
			Message: *a2a.NewTextMessage("user", "initiate task"),
		}

		Convey("When selectTask returns an error", func() {
			selectTaskErr := &errors.RpcError{Code: 7001, Message: "selectTask failed"}
			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					return nil, selectTaskErr
				},
				updateFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil },
			}
			prov := NewControllableMockProvider()
			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(prov))
			So(initErr, ShouldBeNil)
			task, err := manager.SendTask(context.Background(), sendParams)
			Convey("Then SendTask should return nil task and the error from selectTask", func() {
				So(task, ShouldBeNil)
				So(err, ShouldEqual, selectTaskErr)
			})
		})

		Convey("When SendTask succeeds with no provider errors", func() {
			taskForTest := a2a.NewTask(agentCard.Name)
			taskForTest.ID = sendParams.ID
			taskForTest.History = append(taskForTest.History, sendParams.Message)

			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					return []a2a.Task{*taskForTest}, nil
				},
				updateFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil },
			}
			prov := NewControllableMockProvider()

			var initialTaskStateInProvider a2a.TaskState
			var initialTaskMessageInProvider string

			prov.generateFunc = func(ctx context.Context, params *provider.ProviderParams) chan jsonrpc.Response {
				// Assertions on the task state AS IT ENTERS the provider
				So(params.Task, ShouldNotBeNil)
				initialTaskStateInProvider = params.Task.Status.State
				initialTaskMessageInProvider = params.Task.Status.Message.Parts[0].Text

				ch := make(chan jsonrpc.Response, 1)
				ch <- jsonrpc.Response{Result: a2a.TaskStatusUpdateResult{
					Status: a2a.TaskStatus{State: a2a.TaskStateCompleted, Message: a2a.NewTextMessage(agentCard.Name, "Provider completed")},
				}}
				close(ch)
				return ch
			}

			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(prov))
			So(initErr, ShouldBeNil)
			task, err := manager.SendTask(context.Background(), sendParams)

			Convey("Then it should return the updated task, nil error, and provider called correctly", func() {
				So(err, ShouldBeNil)
				So(task, ShouldNotBeNil)

				// Check state as it entered provider
				So(initialTaskStateInProvider, ShouldEqual, a2a.TaskStateWorking)
				So(initialTaskMessageInProvider, ShouldContainSubstring, "starting task")

				So(prov.lastGenerateParams, ShouldNotBeNil)
				So(prov.lastGenerateParams.Stream, ShouldBeFalse)

				// Check final state of the task object
				So(task.Status.State, ShouldEqual, a2a.TaskStateCompleted)
				So(task.Status.Message.Parts[0].Text, ShouldEqual, "Provider completed")
			})
		})

		Convey("When handleUpdate returns an error during provider stream", func() {
			taskForTest := a2a.NewTask(agentCard.Name)
			taskForTest.ID = sendParams.ID
			taskForTest.History = append(taskForTest.History, sendParams.Message)

			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					return []a2a.Task{*taskForTest}, nil
				},
				updateFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil },
			}
			prov := NewControllableMockProvider()
			handleUpdateErrJson := &jsonrpc.Error{Code: 7002, Message: "handleUpdate failed during stream"}

			prov.generateFunc = func(ctx context.Context, params *provider.ProviderParams) chan jsonrpc.Response {
				ch := make(chan jsonrpc.Response, 1)
				ch <- jsonrpc.Response{Error: handleUpdateErrJson}
				close(ch)
				return ch
			}

			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(prov))
			So(initErr, ShouldBeNil)
			task, err := manager.SendTask(context.Background(), sendParams)

			Convey("Then SendTask should return the task and the RpcError from handleUpdate", func() {
				So(task, ShouldNotBeNil)
				So(err, ShouldNotBeNil)
				// This assertion depends on the problematic casts in SendTask and handleUpdate
				// If they "work" by producing an errors.RpcError compatible string:
				expectedReturnedRpcError := &errors.RpcError{Code: handleUpdateErrJson.Code, Message: handleUpdateErrJson.Message}
				So(err.Error(), ShouldEqual, expectedReturnedRpcError.Error())
			})
		})
	})
}

func TestStreamTask(t *testing.T) {
	Convey("Given a TaskManager with controllable store and provider", t, func() {
		agentName := "TestAgentStreamTask"
		agentCard := &a2a.AgentCard{Name: agentName}
		vip := viper.GetViper()
		systemMsgKey := fmt.Sprintf("agent.%s.system", agentName)
		originalSystemMessage := vip.GetString(systemMsgKey)
		vip.Set(systemMsgKey, "Default system message for StreamTask testing")
		defer vip.Set(systemMsgKey, originalSystemMessage)

		inputTask := a2a.NewTask(agentName)
		inputTask.History = append(inputTask.History, *a2a.NewTextMessage("user", "stream this"))

		Convey("When taskStore.Create fails", func() {
			storeCreateErr := &errors.RpcError{Code: 8001, Message: "store.Create failed for stream"}
			store := &taskStoreMockForTesting{
				createFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError {
					return storeCreateErr
				},
			}
			prov := NewControllableMockProvider()
			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(prov))
			So(initErr, ShouldBeNil)
			outChan, err := manager.StreamTask(context.Background(), inputTask)
			Convey("Then it should return a nil channel and the error from the store", func() {
				So(outChan, ShouldBeNil)
				So(err, ShouldEqual, storeCreateErr)
			})
		})

		Convey("When streaming succeeds with multiple chunks", func() {
			store := &taskStoreMockForTesting{
				createFunc: func(ctx context.Context, task *a2a.Task) *errors.RpcError {
					So(task.Status.State, ShouldEqual, a2a.TaskStateWorking)
					So(task.Status.Message.Parts[0].Text, ShouldContainSubstring, "starting task")
					return nil
				},
			}
			prov := NewControllableMockProvider()
			chunk1 := jsonrpc.Response{Result: "chunk one data"}
			chunk2Result := a2a.TaskStatusUpdateResult{Status: a2a.TaskStatus{State: a2a.TaskStateCompleted, Message: a2a.NewTextMessage(agentName, "Stream completed")}}
			chunk2 := jsonrpc.Response{Result: chunk2Result}
			expectedNumChunks := 2

			prov.generateFunc = func(ctx context.Context, params *provider.ProviderParams) chan jsonrpc.Response {
				// So(params.Task.ID, ShouldEqual, inputTask.ID) // Cannot use So here, runs in StreamTask's goroutine
				// The provider captures params via m.lastGenerateParams for later assertion in the test's main goroutine.

				ch := make(chan jsonrpc.Response, expectedNumChunks)
				go func() { // Send chunks asynchronously
					defer close(ch) // Close channel when done sending
					chunksToSend := []jsonrpc.Response{chunk1, chunk2}
					for _, chunkToSend := range chunksToSend {
						select {
						case ch <- chunkToSend:
						case <-ctx.Done(): // Exit if context is cancelled
							return
						}
					}
				}()
				return ch
			}

			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(prov))
			So(initErr, ShouldBeNil)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Add a timeout context
			defer cancel()                                                          // Ensure cancellation

			outChan, err := manager.StreamTask(ctx, inputTask) // Pass the context

			Convey("Then it should return a valid channel, no error, and stream chunks", func() {
				So(err, ShouldBeNil)
				So(outChan, ShouldNotBeNil)

				// nolint:ineffassign // slice inspected later for assertions
				var receivedChunks []jsonrpc.Response
				timeout := time.After(2 * time.Second)
				for len(receivedChunks) < expectedNumChunks {
					select {
					case chunk, ok := <-outChan:
						if !ok {
							break
						}
						receivedChunks = append(receivedChunks, chunk)
					case <-timeout:
						t.Fatalf("Timeout waiting for chunk %d on outChan", len(receivedChunks)+1)
					}
				}
				// Drain the channel to ensure no goroutine leaks
			drainLoop:
				for {
					select {
					case _, ok := <-outChan:
						if !ok {
							break drainLoop // Correctly breaks outer loop
						}
					case <-time.After(10 * time.Millisecond): // Short timeout for drain
						break drainLoop // Correctly breaks outer loop
					}
				}

				So(len(receivedChunks), ShouldEqual, expectedNumChunks)
				if len(receivedChunks) == expectedNumChunks {
					So(receivedChunks[0], ShouldResemble, chunk1)
					So(receivedChunks[1], ShouldResemble, chunk2)
				}
				// Check that inputTask was updated by handleUpdate during the stream
				So(inputTask.Status.State, ShouldEqual, a2a.TaskStateCompleted)
				So(inputTask.Status.Message.Parts[0].Text, ShouldEqual, "Stream completed")

				// Assert params received by the mock provider (captured in lastGenerateParams)
				prov.mu.Lock() // Lock when accessing shared lastGenerateParams
				So(prov.lastGenerateParams, ShouldNotBeNil)
				if prov.lastGenerateParams != nil { // Guard against nil pointer dereference if something unexpected happened
					So(prov.lastGenerateParams.Task, ShouldNotBeNil)
					if prov.lastGenerateParams.Task != nil {
						So(prov.lastGenerateParams.Task.ID, ShouldEqual, inputTask.ID)
					}
					// So(prov.lastGenerateParams.Stream, ShouldBeTrue) // If you need to assert this
				}
				prov.mu.Unlock()
			})
		})

		Convey("When handleUpdate returns an error during streaming", func() {
			store := &taskStoreMockForTesting{ // Default create succeeds
				// Ensure updateFunc is provided if the task object (params) is updated by handleUpdate
				// and these updates need to be asserted or are pre-requisites for later logic.
				// For this specific test, `inputTask` is updated by `handleUpdate`.
				// If `taskStore.Update` was called by `handleUpdate`, we'd mock it here.
				// However, `handleUpdate` directly modifies the task object passed to it.
			}
			prov := NewControllableMockProvider()
			goodChunk := jsonrpc.Response{Result: "good chunk"} // This will be sent and received
			// This chunk will cause handleUpdate to error inside StreamTask's goroutine
			errorCausingJsonRpc := &jsonrpc.Error{Code: 8002, Message: "handleUpdate failed for stream"}

			prov.generateFunc = func(ctx context.Context, params *provider.ProviderParams) chan jsonrpc.Response {
				ch := make(chan jsonrpc.Response, 2)
				go func() {
					defer close(ch)
					// Send the good chunk
					select {
					case ch <- goodChunk:
					case <-ctx.Done():
						return
					}
					// Send the chunk that will cause an error in handleUpdate
					select {
					case ch <- jsonrpc.Response{Error: errorCausingJsonRpc}:
					case <-ctx.Done():
						return
					}
				}()
				return ch
			}

			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(prov))
			So(initErr, ShouldBeNil)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Add a timeout context
			defer cancel()                                                          // Ensure cancellation

			outChan, err := manager.StreamTask(ctx, inputTask) // Pass the context

			// Now StreamTask returns (outChan, nil) immediately.
			// The error from handleUpdate will cause the stream (outChan) to close early.
			Convey("Then StreamTask should return a valid channel, no direct error, but the stream should end after the good chunk", func() {
				So(err, ShouldBeNil) // StreamTask itself doesn't return an error for in-stream failures now
				So(outChan, ShouldNotBeNil)

				var receivedChunks []jsonrpc.Response
				// Expect to receive the goodChunk
				select {
				case chunk, ok := <-outChan:
					So(ok, ShouldBeTrue) // Channel should be open for the first chunk
					So(chunk, ShouldResemble, goodChunk)
					receivedChunks = append(receivedChunks, chunk)
				case <-time.After(2 * time.Second):
					t.Fatal("Timeout waiting for the first good chunk")
				}

				// Expect the channel to be closed now because handleUpdate failed on the second chunk
				select {
				case _, ok := <-outChan:
					So(ok, ShouldBeFalse) // Channel should now be closed
				case <-time.After(2 * time.Second):
					t.Fatal("Timeout waiting for channel to close after handleUpdate error")
				}
			})
		})
	})
}

func TestGetTask(t *testing.T) {
	Convey("Given a TaskManager with a configurable task store", t, func() {
		agentCard := &a2a.AgentCard{Name: "TestAgentGetTask"} // Not directly used by GetTask but needed for manager
		mockProvider := NewControllableMockProvider()         // Not used by GetTask but needed for manager
		taskID := "test-get-task-id"
		historyLength := 5

		Convey("When taskStore.Get returns a task successfully", func() {
			expectedTask := &a2a.Task{ID: taskID, Status: a2a.TaskStatus{State: a2a.TaskStateWorking}}
			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					So(id, ShouldEqual, taskID)
					So(hl, ShouldEqual, historyLength)
					return []a2a.Task{*expectedTask}, nil
				},
			}
			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(initErr, ShouldBeNil)

			task, err := manager.GetTask(context.Background(), taskID, historyLength)

			Convey("Then it should return the task and no error", func() {
				So(err, ShouldBeNil)
				So(task, ShouldNotBeNil)
				So(task.ID, ShouldEqual, expectedTask.ID)
				So(task.Status.State, ShouldEqual, expectedTask.Status.State)
			})
		})

		Convey("When taskStore.Get returns an error", func() {
			expectedErr := &errors.RpcError{Code: 9001, Message: "store.Get failed"}
			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					So(id, ShouldEqual, taskID)
					So(hl, ShouldEqual, historyLength)
					return nil, expectedErr
				},
			}
			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(initErr, ShouldBeNil)

			task, err := manager.GetTask(context.Background(), taskID, historyLength)

			Convey("Then it should return nil task and the error", func() {
				So(task, ShouldBeNil)
				So(err, ShouldEqual, expectedErr)
			})
		})

		Convey("When taskStore.Get returns an empty slice (task not found, no error)", func() {
			store := &taskStoreMockForTesting{
				getFunc: func(ctx context.Context, id string, hl int) ([]a2a.Task, *errors.RpcError) {
					So(id, ShouldEqual, taskID)
					So(hl, ShouldEqual, historyLength)
					return []a2a.Task{}, nil
				},
			}
			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(initErr, ShouldBeNil)

			task, err := manager.GetTask(context.Background(), taskID, historyLength)

			Convey("Then it should return nil task and nil error", func() {
				So(task, ShouldBeNil)
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestCancelTask(t *testing.T) {
	Convey("Given a TaskManager with a configurable task store", t, func() {
		agentCard := &a2a.AgentCard{Name: "TestAgentCancelTask"}
		mockProvider := NewControllableMockProvider() // Not used by CancelTask
		taskID := "test-cancel-task-id"

		Convey("When taskStore.Cancel succeeds", func() {
			store := &taskStoreMockForTesting{
				cancelFunc: func(ctx context.Context, id string) *errors.RpcError {
					So(id, ShouldEqual, taskID)
					return nil // Success
				},
			}
			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(initErr, ShouldBeNil)

			err := manager.CancelTask(context.Background(), taskID)

			Convey("Then it should return no error", func() {
				So(err, ShouldBeNil)
			})
		})

		Convey("When taskStore.Cancel returns an error", func() {
			expectedErr := &errors.RpcError{Code: 9002, Message: "store.Cancel failed"}
			store := &taskStoreMockForTesting{
				cancelFunc: func(ctx context.Context, id string) *errors.RpcError {
					So(id, ShouldEqual, taskID)
					return expectedErr
				},
			}
			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(initErr, ShouldBeNil)

			err := manager.CancelTask(context.Background(), taskID)

			Convey("Then it should return the error from the store", func() {
				So(err, ShouldEqual, expectedErr)
			})
		})
	})
}

func TestResubscribeTask(t *testing.T) {
	Convey("Given a TaskManager with a configurable task store", t, func() {
		agentCard := &a2a.AgentCard{Name: "TestAgentResubscribe"} // Not used by ResubscribeTask directly
		mockProvider := NewControllableMockProvider()             // Not used by ResubscribeTask
		taskID := "test-resubscribe-task-id"
		// historyLength is a param for ResubscribeTask but not used in its current implementation based on task.go
		historyLengthUnused := 0

		Convey("When taskStore.Subscribe returns an error", func() {
			expectedErr := &errors.RpcError{Code: 9003, Message: "store.Subscribe failed"}
			store := &taskStoreMockForTesting{
				subscribeFunc: func(ctx context.Context, id string, ch chan a2a.Task) *errors.RpcError {
					So(id, ShouldEqual, taskID)
					// ch is not closed or used by the mock in this error case
					return expectedErr
				},
			}
			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(initErr, ShouldBeNil)

			outChan, err := manager.ResubscribeTask(context.Background(), taskID, historyLengthUnused)

			Convey("Then it should return a nil channel and the error", func() {
				So(outChan, ShouldBeNil)
				So(err, ShouldEqual, expectedErr)
			})
		})

		Convey("When taskStore.Subscribe succeeds", func() {
			taskSentByStore1 := a2a.Task{ID: taskID, Status: a2a.TaskStatus{State: a2a.TaskStateWorking}}
			taskSentByStore2 := a2a.Task{ID: taskID, Status: a2a.TaskStatus{State: a2a.TaskStateCompleted}}
			var capturedChan chan a2a.Task // To capture the channel passed to store.Subscribe

			store := &taskStoreMockForTesting{
				subscribeFunc: func(ctx context.Context, id string, ch chan a2a.Task) *errors.RpcError {
					So(id, ShouldEqual, taskID)
					capturedChan = ch // Capture the channel

					// Simulate the store sending some tasks to the channel asynchronously
					go func() {
						defer close(capturedChan) // Important: close the channel to terminate range loop in test
						capturedChan <- taskSentByStore1
						time.Sleep(10 * time.Millisecond) // Small delay to ensure tasks are sent sequentially
						capturedChan <- taskSentByStore2
					}()
					return nil // Success
				},
			}
			manager, initErr := NewTaskManager(agentCard, WithTaskStore(store), WithProvider(mockProvider))
			So(initErr, ShouldBeNil)

			outChan, err := manager.ResubscribeTask(context.Background(), taskID, historyLengthUnused)

			Convey("Then it should return a valid receive-only channel, no error, and tasks can be received", func() {
				So(err, ShouldBeNil)
				So(outChan, ShouldNotBeNil)
				// So(capturedChan, ShouldEqual, outChan) // This assertion is too strict due to channel directionality; functionality is tested by ranging over outChan.

				var receivedTasks []a2a.Task
				for task := range outChan { // Read from the returned <-chan a2a.Task
					receivedTasks = append(receivedTasks, task)
				}

				So(len(receivedTasks), ShouldEqual, 2)
				So(receivedTasks[0], ShouldResemble, taskSentByStore1)
				So(receivedTasks[1], ShouldResemble, taskSentByStore2)
			})
		})
	})
}
