package ai

import (
	"context"
	"fmt"
	"net"

	// "net/http" // No longer needed for httpmock
	"testing"

	// "github.com/jarcoal/httpmock" // No longer using httpmock
	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/provider" // Keep for mockTaskStore, even if linter is confused

	// Keep for mockTaskStore, even if linter is confused
	"github.com/valyala/fasthttp"
)

// mockTaskStore is a mock implementation of stores.TaskStore
type mockTaskStore struct{}

func (m *mockTaskStore) Get(ctx context.Context, s string, i int) (*a2a.Task, *errors.RpcError) {
	return nil, nil
}
func (m *mockTaskStore) Subscribe(ctx context.Context, s string, tasks chan a2a.Task) *errors.RpcError {
	return nil
}
func (m *mockTaskStore) Create(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil }
func (m *mockTaskStore) Update(ctx context.Context, task *a2a.Task) *errors.RpcError { return nil }
func (m *mockTaskStore) Delete(ctx context.Context, s string) *errors.RpcError       { return nil }
func (m *mockTaskStore) Cancel(ctx context.Context, s string) *errors.RpcError       { return nil }

// setupAgentDependenciesForTest provides common setup for agent tests.
// It returns an AgentCard, a configured TaskManager, a CatalogClient pointing to a test server,
// and a cleanup function to shut down the server.
func setupAgentDependenciesForTest(t *testing.T, agentName string) (*a2a.AgentCard, *TaskManager, *catalog.CatalogClient, func()) {
	card := &a2a.AgentCard{
		Name: agentName,
	}

	// Setup fasthttp test server for CatalogClient
	ln, err := net.Listen("tcp", "localhost:0") // Listen on a random free port
	if err != nil {
		t.Fatalf("setupAgentDependenciesForTest: net.Listen failed: %v", err)
	}

	server := &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			if string(ctx.Path()) == "/agent" && ctx.IsPost() {
				ctx.SetStatusCode(fasthttp.StatusOK)
			} else {
				ctx.SetStatusCode(fasthttp.StatusNotFound)
			}
		},
	}

	go func() {
		if serveErr := server.Serve(ln); serveErr != nil {
			// This error is expected if it's due to server.Shutdown() closing the listener.
			// t.Logf("fasthttp server.Serve returned: %v (expected on shutdown)", serveErr)
		}
	}()

	cleanup := func() {
		// It's good practice to check the error from Shutdown, though for tests it might be ignored.
		if shutdownErr := server.Shutdown(); shutdownErr != nil {
			// t.Logf("server.Shutdown error: %v", shutdownErr)
		}
	}

	catalogClientBaseURL := fmt.Sprintf("http://%s", ln.Addr().String())
	testCatalogClient := catalog.NewCatalogClient(catalogClientBaseURL)

	// Mock TaskManager dependencies
	mockStore := &mockTaskStore{}
	// Use a real provider instance; its internal client will be nil if WithOpenAIClient option is not used.
	// This is fine for NewTaskManager as it only checks for provider != nil.
	actualProvider := provider.NewOpenAIProvider()

	testTaskManager, err := NewTaskManager(
		card,
		WithTaskStore(mockStore),
		WithProvider(actualProvider),
	)
	if err != nil {
		cleanup() // Attempt to cleanup resources if setup fails mid-way
		t.Fatalf("setupAgentDependenciesForTest: NewTaskManager failed: %v", err)
	}

	return card, testTaskManager, testCatalogClient, cleanup
}

func TestNewAgentFromCard(t *testing.T) {
	Convey("Given an agent is configured with necessary dependencies", t, func() {
		agentCard, taskManager, catalogClient, cleanup := setupAgentDependenciesForTest(t, "TestAgentForNew")
		defer cleanup()

		Convey("When NewAgentFromCard is called", func() {
			agent, err := NewAgentFromCard(
				agentCard,
				WithTaskManager(taskManager),
				WithCatalogClient(catalogClient),
			)

			Convey("Then it should succeed and return a valid agent", func() {
				So(err, ShouldBeNil)
				So(agent, ShouldNotBeNil)
			})
		})
	})
}

func TestName(t *testing.T) {
	Convey("Given an agent created successfully", t, func() {
		expectedName := "TestAgentForName"
		agentCard, taskManager, catalogClient, cleanup := setupAgentDependenciesForTest(t, expectedName)
		defer cleanup()

		agent, err := NewAgentFromCard(
			agentCard,
			WithTaskManager(taskManager),
			WithCatalogClient(catalogClient),
		)
		So(err, ShouldBeNil) // Ensure agent creation was successful before testing Name()
		So(agent, ShouldNotBeNil)

		Convey("When Name() is called", func() {
			name := agent.Name()

			Convey("Then it should return the correct agent name", func() {
				So(name, ShouldEqual, expectedName)
			})
		})
	})
}

func TestCard(t *testing.T) {
	Convey("Given an agent created successfully", t, func() {
		agentCardOriginal, taskManager, catalogClient, cleanup := setupAgentDependenciesForTest(t, "TestAgentForCard")
		defer cleanup()

		agent, err := NewAgentFromCard(
			agentCardOriginal,
			WithTaskManager(taskManager),
			WithCatalogClient(catalogClient),
		)
		So(err, ShouldBeNil) // Ensure agent creation was successful
		So(agent, ShouldNotBeNil)

		Convey("When Card() is called", func() {
			retrievedCard := agent.Card()

			Convey("Then it should return the original agent card", func() {
				So(retrievedCard, ShouldEqual, agentCardOriginal)
				// For pointers to structs, ShouldEqual checks for pointer equality.
				// If a deep comparison is needed and they might be different instances with same values:
				// So(retrievedCard, ShouldResemble, agentCardOriginal)
			})
		})
	})
}
