package sse

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

/*
SSEBroker maintains a list of subscribers and broadcasts JSON‑encoded events
to them.  Each event is sent as a single‑line SSE message of the form:

data: {json}\n\n
*/
type SSEBroker struct {
	mu          sync.RWMutex
	clients     map[chan []byte]struct{}
	taskBrokers map[string]*SSEBroker // Map of task-specific brokers
	closed      bool
	testMode    bool
}

/*
NewSSEBroker creates a new SSEBroker.
*/
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients:     make(map[chan []byte]struct{}),
		taskBrokers: make(map[string]*SSEBroker),
	}
}

/*
NewTestSSEBroker creates a broker with a shorter ticker interval for testing
*/
func NewTestSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients:     make(map[chan []byte]struct{}),
		taskBrokers: make(map[string]*SSEBroker),
		testMode:    true,
	}
}

/*
GetOrCreateTaskBroker returns a task-specific broker, creating one if it doesn't exist.
This allows for targeted event delivery to clients interested in specific tasks.
*/
func (broker *SSEBroker) GetOrCreateTaskBroker(taskID string) interface{} {
	broker.mu.Lock()
	defer broker.mu.Unlock()

	if broker.closed {
		return nil
	}

	if taskBroker, exists := broker.taskBrokers[taskID]; exists {
		return taskBroker
	}

	// Create a new broker for this task
	taskBroker := &SSEBroker{
		clients:  make(map[chan []byte]struct{}),
		testMode: broker.testMode,
	}
	broker.taskBrokers[taskID] = taskBroker
	return taskBroker
}

/*
BroadcastToTask sends a message to all clients subscribed to a specific task.
*/
func (broker *SSEBroker) BroadcastToTask(taskID string, v any) error {
	broker.mu.RLock()
	taskBroker, exists := broker.taskBrokers[taskID]
	broker.mu.RUnlock()

	if !exists || broker.closed {
		return nil // Silently ignore if task broker doesn't exist or is closed
	}

	return taskBroker.Broadcast(v)
}

/*
CloseTaskBroker closes a specific task broker and removes it from the registry.
*/
func (broker *SSEBroker) CloseTaskBroker(taskID string) {
	broker.mu.Lock()
	defer broker.mu.Unlock()

	if taskBroker, exists := broker.taskBrokers[taskID]; exists {
		taskBroker.Close()
		delete(broker.taskBrokers, taskID)
	}
}

/*
Subscribe upgrades the HTTP connection to an SSE stream and blocks until the
client disconnects.  Use from an HTTP handler:

broker.Subscribe(w, r)
*/
func (broker *SSEBroker) Subscribe(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)

	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []byte, 8)
	broker.mu.Lock()

	if broker.closed {
		broker.mu.Unlock()
		http.Error(w, "broker closed", http.StatusGone)
		return
	}

	broker.clients[ch] = struct{}{}
	broker.mu.Unlock()

	// heartbeat ticker to keep connection alive in the presence of proxies.
	tickerInterval := 25 * time.Second

	if broker.testMode {
		tickerInterval = 100 * time.Millisecond
	}

	ticker := time.NewTicker(tickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			broker.remove(ch)
			return
		case msg := <-ch:
			// Handle messages that already have event prefixes
			if bytes.HasPrefix(msg, []byte("event:")) {
				// This is a message with event type already set
				parts := bytes.SplitN(msg, []byte("\n"), 2)
				if len(parts) == 2 {
					// Write the event line
					_, _ = w.Write(parts[0])
					_, _ = w.Write([]byte("\n"))
					// Write the data line with "data: " prefix
					_, _ = w.Write([]byte("data: "))
					_, _ = w.Write(parts[1])
					_, _ = w.Write([]byte("\n\n"))
				} else {
					// Malformed message, just write it with data: prefix
					_, _ = w.Write([]byte("data: "))
					_, _ = w.Write(msg)
					_, _ = w.Write([]byte("\n\n"))
				}
			} else {
				// Standard message with just data
				_, _ = w.Write([]byte("data: "))
				_, _ = w.Write(msg)
				_, _ = w.Write([]byte("\n\n"))
			}
			flusher.Flush()
		case <-ticker.C:
			// comment heartbeat
			_, _ = w.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		}
	}
}

/*
Broadcast marshals v to JSON and sends it to all connected clients.
*/
func (broker *SSEBroker) Broadcast(v any) error {
	// Determine event type based on the value type
	eventType := "message"
	switch data := v.(type) {
	case struct{ Event string }:
		eventType = data.Event
	case map[string]interface{}:
		if evt, ok := data["event"].(string); ok {
			eventType = evt
		}
	}

	// If this is a specific event type, format properly
	if typeMap, ok := v.(map[string]interface{}); ok && typeMap["type"] != nil {
		if eventType == "message" && typeMap["type"] != nil {
			eventType = typeMap["type"].(string)
		}
	}

	msg, err := json.Marshal(v)
	if err != nil {
		return err
	}

	broker.mu.RLock()
	defer broker.mu.RUnlock()

	if broker.closed {
		return nil
	}

	for ch := range broker.clients {
		select {
		case ch <- append([]byte("event: "+eventType+"\n"), msg...):
		default:
			// slow client – drop message to avoid blocking.
		}
	}

	return nil
}

// BroadcastWithEventType marshals v to JSON and sends it to all connected clients with the specified event type.
func (broker *SSEBroker) BroadcastWithEventType(eventType string, v any) error {
	msg, err := json.Marshal(v)
	if err != nil {
		return err
	}

	broker.mu.RLock()
	defer broker.mu.RUnlock()

	if broker.closed {
		return nil
	}

	for ch := range broker.clients {
		select {
		case ch <- append([]byte("event: "+eventType+"\n"), msg...):
		default:
			// slow client – drop message to avoid blocking.
		}
	}

	return nil
}

/*
Close disconnects all clients and prevents further subscriptions.
*/
func (broker *SSEBroker) Close() {
	broker.mu.Lock()
	defer broker.mu.Unlock()

	if broker.closed {
		return
	}

	broker.closed = true

	for ch := range broker.clients {
		close(ch)
	}

	broker.clients = map[chan []byte]struct{}{}
}

/*
remove removes a client from the broker.
*/
func (broker *SSEBroker) remove(ch chan []byte) {
	broker.mu.Lock()

	if _, ok := broker.clients[ch]; ok {
		delete(broker.clients, ch)
		close(ch)
	}

	broker.mu.Unlock()
}
