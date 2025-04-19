// streaming-chat demonstrates token‑level streaming from the OpenAI API using
// the a2a‑go ChatClient and also showcases task streaming and resubscription
// features of the A2A protocol.
//
//	OPENAI_API_KEY=sk‑... go run ./examples/streaming-chat
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
	a2a "github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func main() {
	// Part 1: Basic streaming from OpenAI
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Println("OPENAI_API_KEY not set – OpenAI streaming will be skipped, but A2A streaming demo will continue")
	} else {
		demoOpenAIStreaming()
	}
	
	// Part 2: A2A streaming with task resubscription
	fmt.Println("\n\n=== A2A Protocol Streaming & Resubscription Demo ===")
	demoA2AStreaming()
}

func demoOpenAIStreaming() {
	executor := func(ctx context.Context, t mcp.Tool, args map[string]any) (string, error) {
		return "tool execution placeholder", nil
	}

	client := provider.NewChatClient(executor)

	history := []types.Message{{
		Role: "user",
		Parts: []types.Part{{
			Type: types.PartTypeText,
			Text: "Explain the HTTP 418 status code in one sentence.",
		}},
	}}

	fmt.Println("Assistant (OpenAI streaming):")

	final, err := client.Stream(context.Background(), history, nil, func(delta string) {
		fmt.Print(delta)
	})
	if err != nil {
		log.Fatalf("stream failure: %v", err)
	}

	fmt.Printf("\n\n[final content size=%d bytes]\n", len(final))
}

func demoA2AStreaming() {
	// 1. Start a local A2A server that supports streaming and resubscription
	server := service.NewA2AServerWithDefaults("http://localhost:9090")
	
	// Serve the handlers on appropriate endpoints
	handlers := server.Handlers()
	for path, handler := range handlers {
		http.Handle(path, handler)
	}
	
	// Serve the Agent Card on /.well-known/agent.json
	http.HandleFunc("/.well-known/agent.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(server.Card)
	})
	
	// Start the server
	go func() {
		log.Println("A2A server listening on http://localhost:9090")
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Fatalf("server failed: %v", err)
		}
	}()
	
	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	// 2. Connect to the server and fetch its agent card
	agent, err := a2a.FetchAgentCard(context.Background(), "http://localhost:9090")
	if err != nil {
		log.Fatalf("failed to connect to agent: %v", err)
	}
	
	log.Printf("Connected to agent: %s with capabilities: %+v", agent.Card.Name, agent.Card.Capabilities)
	
	// 3. Create a streaming task with a long simulated response
	taskID := uuid.New().String()
	sessionID := uuid.New().String()
	
	fmt.Println("\nInitiating streaming task. This will simulate a long response...")
	
	artifactText := ""
	artifacts := 0
	statusUpdates := 0
	
	err = agent.SendStream(
		context.Background(),
		types.TaskSendParams{
			ID:        taskID,
			SessionID: sessionID,
			Message: types.Message{
				Role: "user",
				Parts: []types.Part{{
					Type: types.PartTypeText,
					Text: "Stream a long response with multiple chunks",
				}},
			},
		},
		func(status types.TaskStatusUpdateEvent) {
			statusUpdates++
			fmt.Printf("\nStatus update #%d: %s (final=%v)\n", statusUpdates, status.Status.State, status.Final)
		},
		func(artifact types.TaskArtifactUpdateEvent) {
			artifacts++
			if len(artifact.Artifact.Parts) > 0 && artifact.Artifact.Parts[0].Type == types.PartTypeText {
				chunk := artifact.Artifact.Parts[0].Text
				artifactText += chunk
				fmt.Printf("Chunk #%d: %s", artifacts, chunk)
			}
		},
	)
	
	if err != nil {
		log.Fatalf("streaming failed: %v", err)
	}
	
	fmt.Printf("\n\nTask streaming completed. Received %d chunks and %d status updates.\n", artifacts, statusUpdates)
	
	// 4. Demonstrate task resubscription
	fmt.Println("\nNow demonstrating resubscription to the completed task...")
	
	resubArtifacts := 0
	resubStatusUpdates := 0
	
	err = agent.Resubscribe(
		context.Background(),
		taskID,
		1, // request history
		func(status types.TaskStatusUpdateEvent) {
			resubStatusUpdates++
			fmt.Printf("\nResubscribe status update #%d: %s (final=%v)\n", 
				resubStatusUpdates, status.Status.State, status.Final)
		},
		func(artifact types.TaskArtifactUpdateEvent) {
			resubArtifacts++
			if len(artifact.Artifact.Parts) > 0 && artifact.Artifact.Parts[0].Type == types.PartTypeText {
				fmt.Printf("Resubscribe artifact #%d: %s\n", resubArtifacts, artifact.Artifact.Parts[0].Text)
			}
		},
	)
	
	if err != nil {
		log.Fatalf("resubscription failed: %v", err)
	}
	
	fmt.Printf("\nResubscription completed. Received %d artifacts and %d status updates.\n", 
		resubArtifacts, resubStatusUpdates)
		
	// 5. Demonstrate fetching task with history
	fmt.Println("\nFetching task details with history...")
	
	task, err := agent.Get(context.Background(), taskID, 1)
	if err != nil {
		log.Fatalf("get task failed: %v", err)
	}
	
	fmt.Printf("Task ID: %s\n", task.ID)
	fmt.Printf("Session ID: %s\n", task.SessionID)
	fmt.Printf("Status: %s\n", task.Status.State)
	fmt.Printf("History length: %d\n", len(task.History))
	
	if len(task.History) > 0 {
		fmt.Printf("First history message role: %s\n", task.History[0].Role)
		if len(task.History[0].Parts) > 0 {
			fmt.Printf("First history message text: %s\n", task.History[0].Parts[0].Text)
		}
	}
}
