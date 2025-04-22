package sse

import (
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
	mu       sync.RWMutex
	clients  map[chan []byte]struct{}
	closed   bool
	testMode bool
}

/*
NewSSEBroker creates a new SSEBroker.
*/
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan []byte]struct{}),
	}
}

/*
NewTestSSEBroker creates a broker with a shorter ticker interval for testing
*/
func NewTestSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients:  make(map[chan []byte]struct{}),
		testMode: true,
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
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(msg)
			_, _ = w.Write([]byte("\n\n"))
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
		case ch <- msg:
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
