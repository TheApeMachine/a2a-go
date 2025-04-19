// agent-client demonstrates how to use the high‑level a2a.Agent abstraction to
// communicate with a remote agent.  For a self‑contained runnable demo we spin
// up a minimal in‑process JSON‑RPC server that implements the "tasks/send"
// method and then point the Agent at it.
//
//   go run ./examples/agent-client
//
// The output should look similar to:
//   Sent task, received status: completed
//   Artifacts: [hello from agent]

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	a2a "github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func main() {
	// ---------------------------------------------------------------------
	// 1. Start a complete A2AServer with an EchoTaskManager for demonstration.
	// ---------------------------------------------------------------------

	// Create an A2A server that supports all protocol features
	server := service.NewA2AServerWithDefaults("http://localhost:8080")
	
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
		log.Println("A2A server listening on http://localhost:8080")
		_ = http.ListenAndServe(":8080", nil)
	}()

	// Give the server a moment to start (avoids connection refused when run
	// with `go run`).
	time.Sleep(100 * time.Millisecond)

	// ---------------------------------------------------------------------
	// 2. Create an Agent instance pointing at the server and demonstrate features.
	// ---------------------------------------------------------------------

	// Get the agent card from the server
	resp, err := http.Get("http://localhost:8080/.well-known/agent.json")
	if err != nil {
		log.Fatalf("failed to get agent card: %v", err)
	}
	defer resp.Body.Close()
	
	var card types.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		log.Fatalf("failed to decode agent card: %v", err)
	}
	
	log.Printf("Connected to agent: %s with capabilities: %+v", card.Name, card.Capabilities)

	agent := a2a.NewAgentFromCard(card)

	// 1. Basic send task
	msg := types.Message{
		Role: "user",
		Parts: []types.Part{{
			Type: types.PartTypeText,
			Text: "Hello A2A!",
		}},
	}

	params := types.TaskSendParams{
		ID:            "task-1",
		SessionID:     "session-1",
		Message:       msg,
		HistoryLength: 1, // Request the message history
	}

	log.Println("1. Sending a basic task...")
	task, err := agent.Send(context.Background(), params)
	if err != nil {
		log.Fatalf("send failed: %v", err)
	}

	log.Printf("   Sent task, received status: %s", task.Status.State)
	if len(task.Artifacts) > 0 && len(task.Artifacts[0].Parts) > 0 {
		log.Printf("   Artifacts: [%s]", task.Artifacts[0].Parts[0].Text)
	}
	if len(task.History) > 0 {
		log.Printf("   History: [%d messages]", len(task.History))
	}
	
	// 2. Configure push notifications
	log.Println("2. Setting up push notifications...")
	pushConfig := types.TaskPushNotificationConfig{
		ID: "task-1",
		PushNotificationConfig: types.PushNotificationConfig{
			URL: "https://example.com/notify",
			Authentication: &types.AuthenticationInfo{
				Schemes: []string{"Bearer"},
			},
		},
	}
	
	client := service.RPCClient{
		Endpoint: "http://localhost:8080/rpc",
	}
	
	var pushResult types.TaskPushNotificationConfig
	err = client.Call(context.Background(), "tasks/pushNotification/set", pushConfig, &pushResult)
	if err != nil {
		log.Printf("   Push notification setup failed: %v", err)
	} else {
		log.Printf("   Push notification configured for URL: %s", pushResult.PushNotificationConfig.URL)
	}
	
	// 3. Get task details with history
	log.Println("3. Getting task details with history...")
	var getTaskParams struct {
		ID            string `json:"id"`
		HistoryLength int    `json:"historyLength"`
	}
	getTaskParams.ID = "task-1"
	getTaskParams.HistoryLength = 5
	
	var getTaskResult types.Task
	err = client.Call(context.Background(), "tasks/get", getTaskParams, &getTaskResult)
	if err != nil {
		log.Printf("   Get task failed: %v", err)
	} else {
		log.Printf("   Task status: %s", getTaskResult.Status.State)
		log.Printf("   Session ID: %s", getTaskResult.SessionID)
		log.Printf("   History length: %d", len(getTaskResult.History))
	}
	
	// 4. Get push notification config
	log.Println("4. Getting push notification config...")
	var getPushParams struct {
		ID string `json:"id"`
	}
	getPushParams.ID = "task-1"
	
	var getPushResult types.TaskPushNotificationConfig
	err = client.Call(context.Background(), "tasks/pushNotification/get", getPushParams, &getPushResult)
	if err != nil {
		log.Printf("   Get push notification failed: %v", err)
	} else {
		log.Printf("   Push notification URL: %s", getPushResult.PushNotificationConfig.URL)
	}
}
