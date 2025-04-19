package a2a

// A minimal Server‑Sent Events (SSE) helper to stream TaskStatusUpdateEvent and
// TaskArtifactUpdateEvent objects to connected HTTP clients.  This is **not** a
// general‑purpose pub/sub implementation – just enough for the reference
// server anticipated by the A2A spec.

import (
    "encoding/json"
    "net/http"
    "sync"
    "time"
)

// SSEBroker maintains a list of subscribers and broadcasts JSON‑encoded events
// to them.  Each event is sent as a single‑line SSE message of the form:
//   data: {json}\n\n
type SSEBroker struct {
    mu       sync.RWMutex
    clients  map[chan []byte]struct{}
    closed   bool
}

func NewSSEBroker() *SSEBroker {
    return &SSEBroker{
        clients: make(map[chan []byte]struct{}),
    }
}

// Subscribe upgrades the HTTP connection to an SSE stream and blocks until the
// client disconnects.  Use from an HTTP handler:
//   broker.Subscribe(w, r)
func (b *SSEBroker) Subscribe(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming unsupported", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    ch := make(chan []byte, 8)
    b.mu.Lock()
    if b.closed {
        b.mu.Unlock()
        http.Error(w, "broker closed", http.StatusGone)
        return
    }
    b.clients[ch] = struct{}{}
    b.mu.Unlock()

    // heartbeat ticker to keep connection alive in the presence of proxies.
    ticker := time.NewTicker(25 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-r.Context().Done():
            b.remove(ch)
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

// Broadcast marshals v to JSON and sends it to all connected clients.
func (b *SSEBroker) Broadcast(v interface{}) error {
    msg, err := json.Marshal(v)
    if err != nil {
        return err
    }

    b.mu.RLock()
    defer b.mu.RUnlock()
    if b.closed {
        return nil
    }
    for ch := range b.clients {
        select {
        case ch <- msg:
        default:
            // slow client – drop message to avoid blocking.
        }
    }
    return nil
}

// Close disconnects all clients and prevents further subscriptions.
func (b *SSEBroker) Close() {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.closed {
        return
    }
    b.closed = true
    for ch := range b.clients {
        close(ch)
        delete(b.clients, ch)
    }
}

func (b *SSEBroker) remove(ch chan []byte) {
    b.mu.Lock()
    delete(b.clients, ch)
    close(ch)
    b.mu.Unlock()
}
