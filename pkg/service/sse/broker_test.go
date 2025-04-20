package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/theapemachine/a2a-go/pkg/types"
)

func TestSSEBrokerBroadcast(t *testing.T) {
	broker := NewTestSSEBroker()

	// HTTP server exposing /events endpoint.
	ts, errTS := newTestServerSSE(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		broker.Subscribe(w, r)
	}))
	if errTS != nil {
		t.Skip("network disabled; skipping SSE test")
	}
	defer ts.Close()

	// Create client connection.
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("client get: %v", err)
	}
	defer resp.Body.Close()

	// Wait briefly to ensure subscription established.
	time.Sleep(10 * time.Millisecond)

	// Broadcast an event.
	ev := types.TaskStatusUpdateEvent{
		ID:    "abc",
		Final: true,
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
		},
	}
	if err := broker.Broadcast(ev); err != nil {
		t.Fatalf("broadcast: %v", err)
	}

	reader := bufio.NewReader(resp.Body)
	var line string
	deadline := time.After(1 * time.Second)
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

	var got types.TaskStatusUpdateEvent
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != ev.ID || got.Status.State != ev.Status.State || !got.Final {
		t.Fatalf("event mismatch: %+v vs %+v", got, ev)
	}

	// Close the response body first to trigger the context cancellation
	resp.Body.Close()
	// Then close the broker
	broker.Close()
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
