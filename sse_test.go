package a2a

import (
    "bufio"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func TestSSEBrokerBroadcast(t *testing.T) {
    broker := NewSSEBroker()

    // HTTP server exposing /events endpoint.
    ts, errTS := newTestServerSSE(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        broker.Subscribe(w, r)
    }))
    if errTS != nil {
        t.Skip("network disabled; skipping SSE test")
    }
    defer ts.Close()

    // Create client connection.
    client, err := http.Get(ts.URL)
    if err != nil {
        t.Fatalf("client get: %v", err)
    }
    defer client.Body.Close()

    // Wait briefly to ensure subscription established.
    time.Sleep(50 * time.Millisecond)

    // Broadcast an event.
    ev := TaskStatusUpdateEvent{
        ID:    "abc",
        Final: true,
        Status: TaskStatus{
            State: TaskStateCompleted,
        },
    }
    if err := broker.Broadcast(ev); err != nil {
        t.Fatalf("broadcast: %v", err)
    }


    reader := bufio.NewReader(client.Body)
    var line string
    deadline := time.After(5 * time.Second)
L:
    for {
        select {
        case <-deadline:
            t.Fatal("timeout waiting for SSE data line")
        default:
            var err error
            line, err = reader.ReadString('\n')
            if err != nil {
                t.Fatalf("read: %v", err)
            }
            // Skip blank lines and comments
            if strings.TrimSpace(line) == "" || strings.HasPrefix(line, ":") {
                continue
            }
            if strings.HasPrefix(line, "data: ") {
                break L
            }
        }
    }

    payload := strings.TrimPrefix(strings.TrimSpace(line), "data: ")

    var got TaskStatusUpdateEvent
    if err := json.Unmarshal([]byte(payload), &got); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }

    if got.ID != ev.ID || got.Status.State != ev.Status.State || !got.Final {
        t.Fatalf("event mismatch: %+v vs %+v", got, ev)
    }
}

// newTestServer mirrors the helper in jsonrpc_test.go – duplicated to avoid
// import cycles in tests.
func newTestServerSSE(h http.Handler) (*httptest.Server, error) {
    var srv *httptest.Server
    var err error
    func() {
        defer func() {
            if r := recover(); r != nil {
                err = fmt.Errorf("listener not permitted: %v", r)
            }
        }()
        srv = httptest.NewServer(h)
    }()
    return srv, err
}

