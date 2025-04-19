// tool-calling-agent demonstrates a complete agent that can use multiple tools
// to fulfill tasks. This example shows how to:
// 1. Set up an A2A server with built-in tools
// 2. Connect an OpenAI agent with access to these tools
// 3. Process tasks that require tool use across browser and Docker
//
// Usage:
//   OPENAI_API_KEY=sk-... go run ./examples/tool-calling-agent

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/stores"
	"github.com/theapemachine/a2a-go/pkg/tools"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func main() {
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Println("OPENAI_API_KEY environment variable is required for this example")
		log.Println("Please set it to a valid OpenAI API key and try again.")
		os.Exit(1)
	}

	// Create a task store
	taskStore := stores.NewInMemoryTaskStore()

	// Set up MCP server with built-in tools
	mcpServer := server.NewMCPServer()
	tools.RegisterBuiltInTools(mcpServer, taskStore)

	// Create a custom agent card with enhanced tool descriptions
	toolAgentCard := types.AgentCard{
		Name:        "Tool-calling Agent",
		URL:         "http://localhost:8090",
		Version:     "1.0.0",
		Description: stringPtr("An A2A agent that can use browser and Docker tools to fulfill tasks"),
		Capabilities: types.AgentCapabilities{
			Streaming:              true,
			PushNotifications:      true,
			StateTransitionHistory: true,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain", "image/png"},
		Skills: []types.AgentSkill{{
			ID:          "task-executor", 
			Name:        "Task Executor",
			Description: stringPtr("Can browse the web, take screenshots, and execute code in Docker containers"),
			Examples:    []string{
				"Visit a website and analyze content",
				"Take a screenshot of a webpage",
				"Run code in a Docker container",
				"Extract information from a webpage and process it with Docker",
			},
		}},
	}
	
	// Create an A2A server with custom settings
	a2aServer := service.NewA2AServer(toolAgentCard, service.NewEchoTaskManager(nil))
	a2aServer.SamplingManager = provider.NewOpenAISamplingManager(mcpServer)

	// Serve our handlers
	handlers := a2aServer.Handlers()
	for path, handler := range handlers {
		http.Handle(path, handler)
	}

	// Serve the Agent Card at the well-known path
	http.HandleFunc("/.well-known/agent.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a2aServer.Card)
	})

	// Start the server in a goroutine
	go func() {
		log.Println("Tool-calling agent server listening on http://localhost:8090")
		if err := http.ListenAndServe(":8090", nil); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Now let's demonstrate a few example tasks that use our tools
	ctx := context.Background()

	// Example 1: Enhanced browser capabilities with screenshots
	fmt.Println("\n=== Example 1: Enhanced browser capabilities with screenshots ===")
	tryTask(ctx, mcpServer, a2aServer, "Visit https://golang.org and take a screenshot of the page. Tell me what version of Go is currently featured on the homepage.")

	// Example 2: Web scraping with waiting for dynamic content
	fmt.Println("\n\n=== Example 2: Web scraping with dynamic content ===")
	tryTask(ctx, mcpServer, a2aServer, "Visit https://en.wikipedia.org/wiki/Go_(programming_language) and wait for the infobox to load. Find the initial release date of Go and take a screenshot of just that section.")
	
	// Example 3: Docker with environment variables and volumes
	fmt.Println("\n\n=== Example 3: Docker with environment variables ===")
	tryTask(ctx, mcpServer, a2aServer, "Create a small Python script that prints environment variables, then run it in a Docker container with Python, setting NAME='A2A Test' as an environment variable.")
	
	// Example 4: Complex workflow combining browser and Docker
	fmt.Println("\n\n=== Example 4: Complex workflow combining tools ===")
	tryTask(ctx, mcpServer, a2aServer, "Visit https://golang.org and find the current Go version. Then use Docker to create a 'version.txt' file containing this information, and finally use Docker to read and display the file's contents.")
}

// tryTask attempts to execute a task using the tool-calling agent
func tryTask(ctx context.Context, mcpServer *server.MCPServer, a2a *service.A2AServer, prompt string) {
	// Create a unique ID for the task
	taskID := uuid.New().String()
	sessionID := uuid.New().String()

	// Show the task we're attempting
	fmt.Printf("Task prompt: %s\n", prompt)

	// Create a message with the prompt
	msg := types.Message{
		Role: "user",
		Parts: []types.Part{{
			Type: types.PartTypeText,
			Text: prompt,
		}},
	}

	// Create tool calling executor function
	executor := func(ctx context.Context, t mcp.Tool, args map[string]any) (string, error) {
		req := mcp.CallToolRequest{
			Tool: t,
			Params: mcp.CallToolParams{
				Arguments: args,
			},
		}

		result, err := mcpServer.HandleCallTool(ctx, req)
		if err != nil {
			return "", err
		}

		return result.Content, nil
	}

	// Create OpenAI client and streaming params
	client := provider.NewChatClient(executor)
	
	// Get the list of available tools from MCP server
	availableTools := mcpServer.ListTools()

	// Start streaming response
	fmt.Println("\nAssistant (streaming):")
	
	final, err := client.Stream(ctx, []types.Message{msg}, availableTools, func(delta string) {
		fmt.Print(delta)
	})

	if err != nil {
		log.Fatalf("Streaming failed: %v", err)
	}

	// Create a task with the results
	task := types.Task{
		ID:        taskID,
		SessionID: sessionID,
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
		},
		Artifacts: []types.Artifact{{
			Parts: []types.Part{{
				Type: types.PartTypeText,
				Text: final,
			}},
		}},
	}

	// Store the task and message history
	entry := a2a.TaskManager.(*service.EchoTaskManager).GetStore().Create(taskID, "Task complete")
	entry.SessionID = sessionID
	entry.State = types.TaskStateCompleted
	
	// Add the message to history
	a2a.TaskManager.(*service.EchoTaskManager).GetStore().AddMessageToHistory(taskID, msg)
	
	// Add a response message to history
	responseMsg := types.Message{
		Role: "agent",
		Parts: []types.Part{{
			Type: types.PartTypeText,
			Text: final,
		}},
	}
	a2a.TaskManager.(*service.EchoTaskManager).GetStore().AddMessageToHistory(taskID, responseMsg)

	fmt.Printf("\n\nTask %s completed\n", taskID)
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}