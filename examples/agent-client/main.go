// agent-client demonstrates a simple agent using the A2A protocol
// with in-memory tools registered.
//
//   go run ./examples/agent-client

package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/tools"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func main() {
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create the MCP server with memory tools
	mcpServer := server.NewMCPServer("memory-agent", "1.0.0")
	tools.RegisterBuiltInTools(mcpServer, nil)
	
	// Memory system setup is handled internally by tools.RegisterBuiltInTools

	// Create a custom agent card
	memoryAgentCard := types.AgentCard{
		Name:        "Memory Agent",
		URL:         "http://localhost:8080",
		Version:     "1.0.0",
		Description: stringPtr("An A2A agent that can store and retrieve information using the memory system"),
		Capabilities: types.AgentCapabilities{
			Streaming:              true,
			PushNotifications:      true,
			StateTransitionHistory: true,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
	}

	// Create an A2A server
	taskManager := service.NewEchoTaskManager(nil)
	a2aServer := service.NewA2AServer(memoryAgentCard, taskManager)

	// Set up HTTP server
	handlers := a2aServer.Handlers()
	for path, handler := range handlers {
		http.Handle(path, handler)
	}

	// Start the server in the background
	go func() {
		fmt.Println("A2A server listening on http://localhost:8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	fmt.Println("=== Memory-Enabled Agent Demo ===")
	fmt.Println("This agent can store and retrieve information using the memory system.")
	fmt.Println("Try telling it some information and then ask it to recall it later.")
	fmt.Println()
	fmt.Println("Example prompts:")
	fmt.Println("1. 'My favorite color is blue'")
	fmt.Println("2. 'I enjoy hiking in the mountains'")
	fmt.Println("3. 'What do you remember about me?'")
	fmt.Println("4. 'What outdoor activities do I like?'")
	fmt.Println()
	fmt.Println("Press Ctrl+C to exit or type 'exit' to quit")
	fmt.Println()

	// Create a conversation loop
	for {
		// Get user input
		fmt.Print("User: ")
		reader := bufio.NewReader(os.Stdin)
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)

		if userInput == "" {
			continue
		}

		if userInput == "exit" || userInput == "quit" {
			break
		}

		// Send task to the A2A server
		params := types.TaskSendParams{
			ID: fmt.Sprintf("task-%d", time.Now().UnixNano()),
			Message: types.Message{
				Role: "user",
				Parts: []types.Part{
					{
						Type: "text",
						Text: userInput,
					},
				},
			},
		}

		task, err := taskManager.SendTask(ctx, params)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Display agent response
		fmt.Print("Agent: ")
		if len(task.Artifacts) > 0 && len(task.Artifacts[0].Parts) > 0 {
			fmt.Println(task.Artifacts[0].Parts[0].Text)
		} else {
			fmt.Println("No response from agent.")
		}
		fmt.Println()
	}
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}